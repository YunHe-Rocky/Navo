package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/domain/capture"
	"navo/internal/network"
	"navo/internal/network/tun"
	"navo/internal/supervisor"
)

const (
	captureTransitionTimeout = 45 * time.Second
	captureRollbackTimeout   = 20 * time.Second
	tunAdapterTimeout        = 15 * time.Second
	tunAdapterCleanupTimeout = 8 * time.Second
)

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

	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	payload, err := s.prepareCaptureLocked(ctx, mode)
	if err != nil {
		rollbackCtx, rollbackCancel := context.WithTimeout(
			context.Background(), captureRollbackTimeout,
		)
		defer rollbackCancel()
		rollbackErr := s.rollbackCaptureLocked(rollbackCtx)
		return errorResponse(requestID, "CAPTURE_TRANSITION_FAILED", errors.Join(err, rollbackErr))
	}
	s.clearTUNFault()
	payload["mode"] = mode.String()
	return response(requestID, payload)
}

func (s *Service) prepareCaptureLocked(
	ctx context.Context,
	mode capture.Mode,
) (map[string]interface{}, error) {
	if !s.coreSupportsCapture(s.host.ID(), mode) {
		return nil, fmt.Errorf("core %s does not support %s capture mode", s.host.ID(), mode)
	}
	if s.networkManager == nil {
		recoveryManager, err := s.newTUNNetworkManager(ctx, s.runtimeTUNName())
		if err != nil {
			return nil, err
		}
		s.networkManager = recoveryManager
	}
	if err := s.stopCoreForCapture(ctx); err != nil {
		return nil, fmt.Errorf("stop old mode core: %w", err)
	}
	if s.networkManager != nil {
		if err := s.networkManager.Deactivate(ctx); err != nil {
			return nil, fmt.Errorf("restore old TUN routes and DNS: %w", err)
		}
		s.networkManager = nil
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
	if err := s.compileCaptureConfig(ctx, false); err != nil {
		return nil, err
	}
	if err := s.startCoreForCapture(ctx); err != nil {
		return nil, fmt.Errorf("start system-proxy core: %w", err)
	}
	return map[string]interface{}{
		"status":  "running",
		"pid":     s.sup.Status().PID,
		"adapter": capture.AdapterStatus{Name: s.runtimeTUNName(), State: capture.AdapterMissing},
	}, nil
}

func (s *Service) prepareTUNLocked(ctx context.Context) (map[string]interface{}, error) {
	name := s.runtimeTUNName()
	manager, err := s.newTUNNetworkManager(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("prepare TUN network transaction: %w", err)
	}
	if err := manager.Preflight(ctx); err != nil {
		return nil, fmt.Errorf("TUN preflight: %w", err)
	}
	// sing-box/sing-tun exclusively owns creation and configuration of the
	// Wintun adapter. Pre-creating the same named adapter here leaves an open
	// handle and makes sing-tun fail with ERROR_ALREADY_EXISTS.
	if err := s.compileCaptureConfig(ctx, true); err != nil {
		return nil, err
	}
	if err := s.startCoreForCapture(ctx); err != nil {
		return nil, fmt.Errorf("start TUN core: %w", err)
	}
	adapterWaitCtx, adapterWaitCancel := context.WithTimeout(ctx, tunAdapterTimeout)
	defer adapterWaitCancel()
	adapter, err := tun.WaitForAdapterState(
		adapterWaitCtx, name,
		capture.AdapterEnabled, 200*time.Millisecond,
	)
	if err != nil {
		return nil, err
	}
	if err := manager.Activate(ctx); err != nil {
		return nil, fmt.Errorf("configure owned TUN routes and DNS: %w", err)
	}
	s.networkManager = manager
	return map[string]interface{}{
		"status": "running", "pid": s.sup.Status().PID, "adapter": adapter,
	}, nil
}

func (s *Service) rollbackCaptureLocked(ctx context.Context) error {
	var result error
	if s.networkManager != nil {
		result = errors.Join(result, s.networkManager.Deactivate(ctx))
		s.networkManager = nil
	}
	result = errors.Join(result, s.stopCoreForCapture(ctx))
	result = errors.Join(result, s.destroyTUNAdapter(ctx, s.runtimeTUNName()))
	result = errors.Join(result, s.compileCaptureConfig(ctx, false))
	waitCtx, cancel := context.WithTimeout(ctx, tunAdapterTimeout)
	defer cancel()
	_, waitErr := tun.WaitForAdapterState(
		waitCtx, s.runtimeTUNName(), capture.AdapterMissing, 200*time.Millisecond,
	)
	return errors.Join(result, waitErr)
}

func (s *Service) destroyTUNAdapter(ctx context.Context, name string) error {
	existing := tun.InspectAdapter(ctx, name)
	if existing.State == capture.AdapterMissing {
		return s.tunManager.Destroy(ctx)
	}
	// Core-created Wintun adapters can remain disabled after process exit.
	// Open the existing handle only after the core has stopped, then destroy it.
	if err := s.tunManager.Create(ctx, name); err != nil {
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
	if err := s.sup.Start(ctx, s.cfg.ConfigPath); err != nil {
		return err
	}
	return s.commitHealthyRuntime(ctx)
}

func (s *Service) compileCaptureConfig(ctx context.Context, enabled bool) error {
	name, mtu := s.runtimeTUNConfig()
	if err := s.setTUNRuntime(ctx, enabled, name, mtu); err != nil {
		return fmt.Errorf("compile capture config: %w", err)
	}
	return nil
}

func (s *Service) runtimeTUNConfig() (string, int) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	name, mtu := strings.TrimSpace(s.runtime.TUNName), s.runtime.TUNMTU
	if name == "" {
		name = "Navo"
	}
	if mtu <= 0 {
		mtu = 1500
	}
	return name, mtu
}

func (s *Service) runtimeTUNName() string {
	name, _ := s.runtimeTUNConfig()
	return name
}

func (s *Service) newTUNNetworkManager(
	ctx context.Context,
	name string,
) (*network.Manager, error) {
	configDir := s.cfg.ConfigDir
	if configDir == "" {
		configDir = filepath.Join(os.TempDir(), "navo", "service")
	}
	manager, err := network.NewManager(network.Config{
		Enabled: true, AdapterName: name,
		WintunDLLPath:    filepath.Join(filepath.Dir(s.cfg.SingBoxPath), "wintun.dll"),
		JournalPath:      filepath.Join(configDir, "tun_network_journal.json"),
		TUNIPv4Gateway:   "172.19.0.2",
		DNSServers:       []string{"172.19.0.2"},
		ProxyEndpointIPs: s.selectedEndpointIPs(ctx),
		IPv6Mode:         network.IPv6Block,
		AdapterTimeout:   tunAdapterTimeout,
	}, network.NewSystemExecutor(), network.NewPlatform())
	if err != nil {
		return nil, err
	}
	if err := manager.Recover(ctx); err != nil {
		return nil, fmt.Errorf("recover incomplete network transaction: %w", err)
	}
	return manager, nil
}

func (s *Service) selectedEndpointIPs(ctx context.Context) []string {
	s.runtimeMu.Lock()
	selectedID := s.runtime.SelectedOutbound
	s.runtimeMu.Unlock()
	for _, outbound := range s.currentOutbounds(ctx) {
		if outbound.ID != selectedID {
			continue
		}
		if ip := net.ParseIP(outbound.Server); ip != nil {
			return []string{ip.String()}
		}
		resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		addresses, err := net.DefaultResolver.LookupIPAddr(resolveCtx, outbound.Server)
		cancel()
		if err != nil {
			log.Printf("[service] resolve selected endpoint %q: %v", outbound.Server, err)
			return nil
		}
		result := make([]string, 0, len(addresses))
		for _, address := range addresses {
			if address.IP.To4() != nil {
				result = append(result, address.IP.String())
			}
		}
		return result
	}
	return nil
}

func (s *Service) monitorTUNAdapter(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
		s.runtimeMu.Lock()
		enabled, name := s.runtime.TUNEnabled, s.runtime.TUNName
		s.runtimeMu.Unlock()
		if !enabled || s.sup.State() != supervisor.StateRunning {
			continue
		}
		status := tun.InspectAdapter(ctx, name)
		if status.State == capture.AdapterEnabled {
			continue
		}
		if !s.captureMu.TryLock() {
			continue
		}
		s.sup.SetRestartSuppressed(true)
		s.runtimeMu.Lock()
		stillEnabled := s.runtime.TUNEnabled
		s.runtimeMu.Unlock()
		if stillEnabled {
			message := fmt.Sprintf(
				"TUN adapter %q became %s: %s",
				name, status.State, status.Error,
			)
			s.setTUNFault(message)
			rollbackCtx, rollbackCancel := context.WithTimeout(
				context.Background(), captureRollbackTimeout,
			)
			if err := s.rollbackCaptureLocked(rollbackCtx); err != nil {
				log.Printf("[service] TUN adapter failure rollback: %v", err)
			}
			rollbackCancel()
		}
		s.sup.SetRestartSuppressed(false)
		s.captureMu.Unlock()
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
