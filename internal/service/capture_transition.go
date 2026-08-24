package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/network"
	"navo/internal/network/tun"
	"navo/internal/supervisor"
)

const (
	captureTransitionTimeout  = 90 * time.Second
	captureRollbackTimeout    = 20 * time.Second
	tunAdapterTimeout         = 15 * time.Second
	tunAdapterCleanupTimeout  = 8 * time.Second
	tunHealthFailureThreshold = 3
)

type tunNetworkManager interface {
	Preflight(context.Context) error
	Activate(context.Context) (network.AdapterSnapshot, error)
	Rebind(context.Context) (network.AdapterSnapshot, error)
	Deactivate(context.Context) error
	Recover(context.Context) error
}

func (s *Service) handleCapturePrepare(
	parent context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	rawMode, _ := msg["mode"].(string)
	mode := capture.Mode(rawMode)
	if !mode.Valid() {
		return errorResponse(requestID, "INVALID", fmt.Errorf("unsupported capture mode %q", rawMode))
	}
	ctx, cancel := context.WithTimeout(parent, captureTransitionTimeout)
	defer cancel()

	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "CAPTURE_BUSY", err)
	}
	defer s.captureMu.Unlock()
	if mode == capture.ModeTUN {
		if requestedName, ok := msg["tun_name"].(string); ok && requestedName != "" {
			name, err := normalizeOwnedTUNName(requestedName)
			if err != nil {
				return errorResponse(requestID, "NET_ADAPTER_OWNERSHIP", err)
			}
			mtu := 0
			switch value := msg["tun_mtu"].(type) {
			case int:
				mtu = value
			case float64:
				mtu = int(value)
			}
			if mtu < 1280 || mtu > 9000 {
				return errorResponse(requestID, "INVALID", fmt.Errorf("TUN MTU must be between 1280 and 9000"))
			}
			s.runtimeMu.Lock()
			s.runtime.TUNName, s.runtime.TUNMTU = name, mtu
			s.runtimeMu.Unlock()
		}
	}
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	payload, err := s.prepareCaptureLocked(ctx, mode)
	if err != nil {
		rollbackCtx, rollbackCancel := context.WithTimeout(
			context.Background(), captureRollbackTimeout,
		)
		defer rollbackCancel()
		rollbackErr := s.rollbackCaptureLocked(rollbackCtx)
		combined := errors.Join(err, rollbackErr)
		code := "CAPTURE_TRANSITION_FAILED"
		var tunErr *network.TUNError
		if errors.As(combined, &tunErr) && tunErr.Code != "" {
			code = tunErr.Code
		}
		var captureErr *captureTransitionError
		if errors.As(combined, &captureErr) && captureErr.code != "" {
			code = captureErr.code
		}
		if mode == capture.ModeTUN {
			s.setTUNFault(combined.Error())
		} else {
			// System Proxy failures are not TUN adapter failures. Publishing them
			// through tun.status makes the Agent rewrite the requested mode and the
			// desktop offer an invalid "retry TUN" action.
			s.clearTUNFault()
		}
		return errorResponse(requestID, code, combined)
	}
	s.clearTUNFault()
	s.runtimeMu.Lock()
	runtimeMode := s.runtime.Mode
	s.runtimeMu.Unlock()
	payload["runtime_mode"] = runtimeMode
	payload["mode"] = mode.String()
	return response(requestID, payload)
}

func (s *Service) lockCapture(ctx context.Context) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.captureMu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for active connection transaction: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *Service) prepareCaptureLocked(
	ctx context.Context,
	mode capture.Mode,
) (map[string]interface{}, error) {
	if !s.coreSupportsCapture(s.host.ID(), mode) {
		return nil, fmt.Errorf("core %s does not support %s capture mode", s.host.ID(), mode)
	}
	if s.networkManager == nil {
		recoveryManager, err := s.newTUNRecoveryManager(s.runtimeTUNName())
		if err != nil {
			return nil, err
		}
		s.networkManager = recoveryManager
	}
	if err := s.stopCoreForCapture(ctx); err != nil {
		return nil, fmt.Errorf("stop old mode core: %w", err)
	}
	if s.networkManager != nil {
		if err := s.deactivateTUNNetwork(ctx); err != nil {
			return nil, fmt.Errorf("restore old TUN routes and DNS: %w", err)
		}
	}
	tunName := s.runtimeTUNName()
	if err := s.destroyTUNAdapter(ctx, tunName); err != nil {
		return nil, fmt.Errorf("release old TUN adapter: %w", err)
	}
	adapterWaitCtx, adapterWaitCancel := context.WithTimeout(ctx, tunAdapterCleanupTimeout)
	defer adapterWaitCancel()
	if _, err := tun.WaitForAdapterState(
		adapterWaitCtx, tunName,
		capture.AdapterMissing, 200*time.Millisecond,
	); err != nil {
		remaining := tun.InspectAdapter(ctx, tunName)
		return nil, fmt.Errorf(
			"stale TUN adapter %q remains %s after cleanup: %w",
			tunName, remaining.State, err,
		)
	}
	s.clearTUNRuntimeResult()

	if mode == capture.ModeOff {
		if err := s.compileCaptureConfig(ctx, false); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status": "stopped",
			"adapter": capture.AdapterStatus{
				Name: s.runtimeTUNName(), State: capture.AdapterMissing,
			},
		}, nil
	}

	if mode == capture.ModeTUN {
		return s.prepareTUNLocked(ctx)
	}
	if err := s.verifySelectedOutboundReachable(ctx); err != nil {
		return nil, err
	}
	name, mtu := s.runtimeTUNConfig()
	if err := s.setSystemProxyRuntime(ctx, name, mtu); err != nil {
		return nil, fmt.Errorf("compile system-proxy config after endpoint readiness: %w", err)
	}
	if err := s.startCoreForCapture(ctx); err != nil {
		return nil, fmt.Errorf("start system-proxy core: %w", err)
	}
	verification, err := s.verifyActiveRuntimeRouting(ctx)
	if err != nil {
		return nil, fmt.Errorf("verify ChatGPT application routing: %w", err)
	}
	if err := s.commitHealthyRuntime(ctx); err != nil {
		return nil, fmt.Errorf("commit system-proxy runtime: %w", err)
	}
	return map[string]interface{}{
		"status":       "running",
		"pid":          s.sup.Status().PID,
		"adapter":      capture.AdapterStatus{Name: s.runtimeTUNName(), State: capture.AdapterMissing},
		"verification": verification,
	}, nil
}

type captureTransitionError struct {
	code string
	err  error
}

func (e *captureTransitionError) Error() string { return e.err.Error() }
func (e *captureTransitionError) Unwrap() error { return e.err }

func (s *Service) verifySelectedOutboundReachable(ctx context.Context) error {
	s.runtimeMu.Lock()
	selectedID := strings.TrimSpace(s.runtime.SelectedOutbound)
	runtimeMode := s.runtime.Mode
	s.runtimeMu.Unlock()
	if selectedID == "" {
		if runtimeMode == runtimeModeDirect {
			return nil
		}
		return &captureTransitionError{
			code: "OUTBOUND_REQUIRED",
			err:  fmt.Errorf("select an available proxy route before enabling capture in %s mode", runtimeMode),
		}
	}
	return verifyOutboundReachability(ctx, selectedID, s.currentOutbounds(ctx), s.prober)
}

func verifyOutboundReachability(
	ctx context.Context,
	selectedID string,
	outbounds []compiler.Outbound,
	prober outboundProber,
) error {
	for _, outbound := range outbounds {
		if outbound.ID != selectedID {
			continue
		}
		if prober == nil {
			return &captureTransitionError{
				code: "OUTBOUND_UNREACHABLE",
				err:  fmt.Errorf("selected outbound %q cannot be checked: TCP prober is unavailable", selectedID),
			}
		}
		probe := prober.ProbeTCP(ctx, outbound.ID, outbound.Server, outbound.Port)
		if probe != nil && probe.Healthy {
			return nil
		}
		reason := "TCP probe failed"
		if probe != nil && strings.TrimSpace(probe.Error) != "" {
			reason = probe.Error
		}
		return &captureTransitionError{
			code: "OUTBOUND_UNREACHABLE",
			err: fmt.Errorf(
				"selected outbound %q (%s:%d) is unreachable: %s",
				selectedID, outbound.Server, outbound.Port, reason,
			),
		}
	}
	return &captureTransitionError{
		code: "OUTBOUND_UNAVAILABLE",
		err:  fmt.Errorf("selected outbound %q is not available in the current source set", selectedID),
	}
}

func (s *Service) prepareTUNLocked(ctx context.Context) (map[string]interface{}, error) {
	name := s.runtimeTUNName()
	preflightManager, err := s.newTUNRecoveryManager(name)
	if err != nil {
		return nil, fmt.Errorf("prepare TUN network transaction: %w", err)
	}
	s.networkManager = preflightManager
	s.setTUNStage(network.TUNStagePreflight)
	if err := preflightManager.Preflight(ctx); err != nil {
		return nil, fmt.Errorf("TUN preflight: %w", err)
	}
	directIP, err := s.tunVerifier.CaptureDirectIP(ctx)
	if err != nil {
		return nil, &network.TUNError{Code: network.ErrTUNPreflightFailed, Stage: network.TUNStageBaselineCaptured, Resource: "direct_ip", Expected: "valid public IP before TUN", Actual: "unavailable", Cause: err}
	}
	s.setTUNStage(network.TUNStageBaselineCaptured)
	plan, selected, directMode, err := s.buildTUNActivationPlan(ctx, name)
	if err != nil {
		return nil, err
	}
	manager, err := s.newTUNNetworkManager(plan)
	if err != nil {
		return nil, fmt.Errorf("prepare planned TUN network transaction: %w", err)
	}
	// Save rollback ownership before the core can create the adapter and before
	// any route, NRPT, or firewall mutation.
	s.networkManager = manager
	// sing-box/sing-tun exclusively owns creation and configuration of the
	// Wintun adapter. Pre-creating the same named adapter here leaves an open
	// handle and makes sing-tun fail with ERROR_ALREADY_EXISTS.
	if err := s.compileCaptureConfigWithPlan(ctx, plan); err != nil {
		return nil, err
	}
	s.setTUNStage(network.TUNStageConfigCompiled)
	if err := s.startCoreForCapture(ctx); err != nil {
		return nil, &network.TUNError{Code: network.ErrTUNCoreStartFailed, Stage: network.TUNStageCoreStarted, Resource: plan.CoreID, Cause: err}
	}
	s.setTUNStage(network.TUNStageCoreStarted)
	adapter, err := manager.Activate(ctx)
	if err != nil {
		return nil, fmt.Errorf("configure owned TUN routes and DNS: %w", err)
	}
	s.setTUNStage(network.TUNStageControlPlaneVerified)
	if tunFailurePoint() == "during-dataplane" {
		return nil, &network.TUNError{Code: network.ErrTUNHTTPSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: "failure_injection", Expected: "data-plane verification", Actual: "injected failure"}
	}
	verification, err := s.tunVerifier.Verify(ctx, VerifyRequest{
		SessionID: plan.SessionID, DirectIP: directIP, DirectMode: directMode,
		ProxyPort: s.cfg.ProxyPort, TUNDNSIPv4: plan.TUNDNSIPv4,
		UDPRequired: directMode || outboundRequiresUDP(selected),
	})
	if err != nil {
		return nil, err
	}
	s.setTUNStage(network.TUNStageDataPlaneVerified)
	if err := s.commitHealthyRuntime(ctx); err != nil {
		return nil, fmt.Errorf("commit verified TUN runtime: %w", err)
	}
	s.setTUNRuntimeResult(network.TUNStageHealthCommitted, plan.SessionID, adapter, verification)
	return map[string]interface{}{
		"status": "running", "pid": s.sup.Status().PID,
		"stage": network.TUNStageHealthCommitted, "adapter": adapter,
		"verification": verification,
	}, nil
}

func (s *Service) rollbackCaptureLocked(ctx context.Context) error {
	var result error
	if s.networkManager != nil {
		if err := s.deactivateTUNNetwork(ctx); err != nil {
			result = errors.Join(result, err)
		}
	}
	result = errors.Join(result, s.stopCoreForCapture(ctx))
	result = errors.Join(result, s.destroyTUNAdapter(ctx, s.runtimeTUNName()))
	waitCtx, cancel := context.WithTimeout(ctx, tunAdapterTimeout)
	defer cancel()
	_, waitErr := tun.WaitForAdapterState(
		waitCtx, s.runtimeTUNName(), capture.AdapterMissing, 200*time.Millisecond,
	)
	result = errors.Join(result, waitErr)
	if result == nil {
		// Emergency rollback must not resolve credentials or compile a config.
		// The core is stopped, and the next requested mode compiles before start.
		s.resetTUNRuntimeAfterRollback()
		s.clearTUNRuntimeResult()
	}
	return result
}

func (s *Service) deactivateTUNNetwork(ctx context.Context) error {
	if s.networkManager == nil {
		return nil
	}
	if err := s.networkManager.Deactivate(ctx); err != nil {
		return err
	}
	s.networkManager = nil
	return nil
}

func (s *Service) destroyTUNAdapter(ctx context.Context, name string) error {
	ownedName, err := normalizeOwnedTUNName(name)
	if err != nil {
		return err
	}
	name = ownedName
	existing := tun.InspectAdapter(ctx, name)
	if existing.State == capture.AdapterMissing {
		return s.tunManager.Destroy(ctx)
	}
	// Core-created Wintun adapters can remain disabled after process exit.
	// Open the existing handle only after the core has stopped, then destroy it.
	if err := s.tunManager.Create(ctx, name); err != nil {
		// The core can release its final Wintun handle between inspection and
		// open. Treat the resulting disappearance as successful cleanup.
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if _, waitErr := tun.WaitForAdapterState(
			waitCtx, name, capture.AdapterMissing, 100*time.Millisecond,
		); waitErr == nil {
			return nil
		}
		return fmt.Errorf("open existing TUN adapter %q for cleanup: %w", name, err)
	}
	return s.tunManager.Destroy(ctx)
}

func (s *Service) stopCoreForCapture(ctx context.Context) error {
	switch s.sup.State() {
	case supervisor.StateStopped:
		return nil
	case supervisor.StateRunning, supervisor.StateDegraded, supervisor.StateFailed, supervisor.StateStarting:
		return s.sup.Stop(ctx)
	default:
		return fmt.Errorf("core lifecycle is busy (state=%s)", s.sup.State())
	}
}

func (s *Service) startCoreForCapture(ctx context.Context) error {
	if s.sup.State() == supervisor.StateRunning {
		return nil
	}
	if s.cfg.ConfigPath == "" {
		return fmt.Errorf("runtime config path is empty")
	}
	return s.sup.Start(ctx, s.cfg.ConfigPath)
}

func (s *Service) compileCaptureConfig(ctx context.Context, enabled bool) error {
	name, mtu := s.runtimeTUNConfig()
	if err := s.setTUNRuntime(ctx, enabled, name, mtu); err != nil {
		return fmt.Errorf("compile capture config: %w", err)
	}
	return nil
}

func (s *Service) compileCaptureConfigWithPlan(ctx context.Context, plan network.TUNActivationPlan) error {
	name, mtu := s.runtimeTUNConfig()
	if err := s.setTUNRuntimeWithPlan(ctx, true, name, mtu, plan); err != nil {
		return fmt.Errorf("compile pinned TUN config: %w", err)
	}
	return nil
}

func (s *Service) runtimeTUNConfig() (string, int) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	_, mtu := strings.TrimSpace(s.runtime.TUNName), s.runtime.TUNMTU
	name := network.OwnedTUNAdapterName
	if mtu <= 0 {
		mtu = 1500
	}
	return name, mtu
}

func (s *Service) runtimeTUNName() string {
	name, _ := s.runtimeTUNConfig()
	return name
}

func (s *Service) newTUNRecoveryManager(name string) (*network.Manager, error) {
	return s.newTUNNetworkManager(network.TUNActivationPlan{AdapterName: name})
}

func (s *Service) newTUNNetworkManager(plan network.TUNActivationPlan) (*network.Manager, error) {
	configDir := s.cfg.ConfigDir
	if configDir == "" {
		configDir = filepath.Join(os.TempDir(), "navo", "service")
	}
	return network.NewManager(network.Config{
		Enabled: true, AdapterName: plan.AdapterName,
		WintunDLLPath:  filepath.Join(filepath.Dir(s.cfg.SingBoxPath), "wintun.dll"),
		JournalPath:    filepath.Join(configDir, "tun_network_journal.json"),
		TUNIPv4Gateway: "172.19.0.2", TUNIPv4Address: "172.19.0.1/30",
		TUNIPv4Peer: "172.19.0.2", TUNDNSIPv4: "172.19.0.2", MTU: plan.MTU,
		DNSServers:     []string{"172.19.0.2"},
		IPv6Mode:       network.IPv6Block,
		AdapterTimeout: tunAdapterTimeout,
		ActivationPlan: plan,
		StageFn:        s.setTUNStage,
		FailurePoint:   tunManagerFailurePoint(),
		CrashPoint:     tunCrashPoint(),
		CrashFn:        func() { os.Exit(91) },
	}, network.NewSystemExecutor(), network.NewPlatform())
}

func tunFailurePoint() string {
	if os.Getenv("NAVO_TUN_TEST_MODE") != "1" {
		return ""
	}
	return strings.TrimSpace(os.Getenv("NAVO_TUN_FAILURE_POINT"))
}

func tunManagerFailurePoint() string {
	point := tunFailurePoint()
	if point == "during-dataplane" || point == "none" {
		return ""
	}
	return point
}

func tunCrashPoint() string {
	if os.Getenv("NAVO_TUN_TEST_MODE") != "1" {
		return ""
	}
	return strings.TrimSpace(os.Getenv("NAVO_TUN_CRASH_POINT"))
}

func (s *Service) buildTUNActivationPlan(ctx context.Context, name string) (network.TUNActivationPlan, *compiler.Outbound, bool, error) {
	s.runtimeMu.Lock()
	selectedID, coreID, mtu, runtimeMode := s.runtime.SelectedOutbound, s.runtime.CoreID, s.runtime.TUNMTU, s.runtime.Mode
	s.runtimeMu.Unlock()
	directMode := false
	var selected *compiler.Outbound
	for _, outbound := range s.currentOutbounds(ctx) {
		if outbound.ID == selectedID {
			copy := outbound
			selected = &copy
			break
		}
	}
	// A fresh profile has no persisted selection and the generated runtime is
	// direct-only. Treat that state as explicit direct mode for TUN validation.
	directMode = isDirectRuntime(runtimeMode, selectedID, selected)
	if !directMode && selected == nil {
		return network.TUNActivationPlan{}, nil, false, &network.TUNError{Code: network.ErrTUNEndpointResolveFailed, Stage: network.TUNStagePreflight, Resource: selectedID, Cause: fmt.Errorf("selected outbound was not found")}
	}
	host := ""
	if selected != nil && !directMode {
		host = selected.Server
	}
	plan, err := network.BuildTUNActivationPlan(ctx, network.ActivationPlanRequest{
		SessionID: network.NewTUNSessionID(), CoreID: coreID, AdapterName: name,
		TUNIPv4Address: "172.19.0.1/30", TUNIPv4Peer: "172.19.0.2",
		TUNDNSIPv4: "172.19.0.2", MTU: mtu, SelectedOutboundID: selectedID,
		OriginalServerHost: host, IPv6Mode: network.IPv6Block,
	})
	return plan, selected, directMode, err
}

func isUnselectedDirectRuntime(selectedID string, selected *compiler.Outbound) bool {
	return selected == nil && selectedID == ""
}

func isDirectRuntime(mode, selectedID string, selected *compiler.Outbound) bool {
	return mode == runtimeModeDirect || isUnselectedDirectRuntime(selectedID, selected)
}

func outboundRequiresUDP(outbound *compiler.Outbound) bool {
	if outbound == nil {
		return false
	}
	switch outbound.Type {
	case compiler.OutboundShadowsocks, compiler.OutboundVMess, compiler.OutboundVLESS,
		compiler.OutboundTrojan, compiler.OutboundHysteria2, compiler.OutboundTUIC:
		return true
	default:
		return false
	}
}

func (s *Service) setTUNStage(stage network.TUNStage) {
	s.tunRuntimeMu.Lock()
	s.tunStage = stage
	s.tunRuntimeMu.Unlock()
	log.Printf("[service] TUN transition stage=%s", stage)
}

func (s *Service) setTUNRuntimeResult(stage network.TUNStage, sessionID string, adapter network.AdapterSnapshot, verification VerifyResult) {
	s.tunRuntimeMu.Lock()
	s.tunStage, s.tunSessionID, s.tunAdapter, s.tunVerification = stage, sessionID, adapter, verification
	s.tunRuntimeMu.Unlock()
}

func (s *Service) clearTUNRuntimeResult() {
	s.tunRuntimeMu.Lock()
	s.tunStage, s.tunSessionID, s.tunAdapter, s.tunVerification = "", "", network.AdapterSnapshot{}, VerifyResult{}
	s.tunRuntimeMu.Unlock()
}

type tunHealthExpectation struct {
	sessionID string
	name      string
	guid      string
	index     int
}

type tunHealthTracker struct {
	sessionID   string
	consecutive int
}

func (t *tunHealthTracker) observe(sessionID string, actionable bool) bool {
	if !actionable || sessionID == "" {
		t.sessionID, t.consecutive = "", 0
		return false
	}
	if t.sessionID != sessionID {
		t.sessionID, t.consecutive = sessionID, 1
	} else {
		t.consecutive++
	}
	return t.consecutive >= tunHealthFailureThreshold
}

func normalizeAdapterGUID(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), "{}")
}

func tunObservationHealthy(expected tunHealthExpectation, observed capture.AdapterStatus) bool {
	if observed.State != capture.AdapterEnabled || observed.InterfaceIndex <= 0 || observed.InterfaceGUID == "" {
		return false
	}
	return tunObservationMatchesIdentity(expected, observed)
}

func tunObservationMatchesIdentity(expected tunHealthExpectation, observed capture.AdapterStatus) bool {
	if expected.index > 0 && observed.InterfaceIndex != expected.index {
		return false
	}
	return expected.guid == "" || normalizeAdapterGUID(observed.InterfaceGUID) == normalizeAdapterGUID(expected.guid)
}

func actionableTUNObservation(expected tunHealthExpectation, observed capture.AdapterStatus) bool {
	if observed.State == capture.AdapterEnabled {
		return !tunObservationHealthy(expected, observed)
	}
	switch observed.State {
	case capture.AdapterMissing, capture.AdapterDisabled, capture.AdapterDriverError:
		return true
	default:
		// Starting/stopping and inspection failures are observations, not proof
		// that the committed adapter disappeared. Never roll back on ambiguity.
		return false
	}
}

func (s *Service) currentTUNHealthExpectation() (tunHealthExpectation, bool) {
	s.runtimeMu.Lock()
	enabled, name := s.runtime.TUNEnabled, s.runtime.TUNName
	s.runtimeMu.Unlock()
	s.tunRuntimeMu.RLock()
	stage, sessionID, adapter := s.tunStage, s.tunSessionID, s.tunAdapter
	s.tunRuntimeMu.RUnlock()
	if !enabled || stage != network.TUNStageHealthCommitted || sessionID == "" || s.sup.State() != supervisor.StateRunning {
		return tunHealthExpectation{}, false
	}
	if strings.TrimSpace(name) == "" {
		name = "Navo"
	}
	return tunHealthExpectation{
		sessionID: sessionID,
		name:      name,
		guid:      adapter.InterfaceGUID,
		index:     int(adapter.InterfaceIndex),
	}, true
}

func (s *Service) monitorTUNAdapter(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	tracker := tunHealthTracker{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
		expected, committed := s.currentTUNHealthExpectation()
		if !committed {
			tracker.observe("", false)
			continue
		}
		status := tun.InspectAdapter(ctx, expected.name)
		if tunObservationHealthy(expected, status) {
			tracker.observe(expected.sessionID, false)
			continue
		}
		if !tracker.observe(expected.sessionID, actionableTUNObservation(expected, status)) {
			continue
		}
		current, stillCommitted := s.currentTUNHealthExpectation()
		if stillCommitted && current == expected {
			// Scheduler publishes evidence only. Agent owns the cross-domain
			// recovery transaction and will re-check this fault before mutation.
			confirmed := tun.InspectAdapter(ctx, current.name)
			if actionableTUNObservation(current, confirmed) {
				message := fmt.Sprintf(
					"TUN adapter %q failed %d consecutive health checks (state=%s guid=%s index=%d): %s",
					current.name, tunHealthFailureThreshold, confirmed.State,
					confirmed.InterfaceGUID, confirmed.InterfaceIndex, confirmed.Error,
				)
				s.setTUNFault(message)
				log.Printf("[service] TUN health event: %s", message)
			}
		}
		tracker.observe("", false)
	}
}

func (s *Service) setTUNFault(message string) {
	s.tunFaultMu.Lock()
	defer s.tunFaultMu.Unlock()
	if s.tunFaultID == "" {
		s.tunFaultID = fmt.Sprintf("tun-fault-%d", time.Now().UnixNano())
	}
	s.tunFault = message
}

func (s *Service) clearTUNFault() {
	s.tunFaultMu.Lock()
	s.tunFaultID, s.tunFault = "", ""
	s.tunFaultMu.Unlock()
}

func (s *Service) currentTUNFault() (string, string) {
	s.tunFaultMu.RLock()
	defer s.tunFaultMu.RUnlock()
	return s.tunFaultID, s.tunFault
}
