// Package service provides the Windows Service wrapper for the Navo proxy core.
// It integrates with the Windows Service Control Manager (SCM) and wraps
// the Supervisor to manage sing-box lifecycle.
//
// Architecture: Agent Coordinator → Service domain gate → Supervisor → selected CoreHost.
// Service executes privileged domain actions; it does not choose cross-domain order.
package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"navo/internal/compiler"
	"navo/internal/coreadapter"
	"navo/internal/credential"
	"navo/internal/domain/capture"
	"navo/internal/domain/revision"
	"navo/internal/domain/selection"
	"navo/internal/host"
	"navo/internal/ipdetect"
	"navo/internal/logstore"
	"navo/internal/monitor"
	"navo/internal/network"
	"navo/internal/network/tun"
	"navo/internal/pipe"
	"navo/internal/selfheal"
	"navo/internal/subscription"
	"navo/internal/supervisor"
	"navo/internal/upstreamproxy"
)

const (
	defaultServiceIPCRequestTimeout = 30 * time.Second
	captureServiceIPCRequestTimeout = 2 * time.Minute
	coreServiceIPCRequestTimeout    = 4 * time.Minute
)

// Config holds the service configuration.
type Config struct {
	SingBoxPath          string               // path to sing-box.exe
	MihomoPath           string               // optional path to mihomo.exe
	XrayPath             string               // optional path to xray.exe
	ConfigPath           string               // path to sing-box config JSON
	ConfigDir            string               // directory for config storage
	PipeName             string               // named pipe name for IPC, e.g. "Navo.Agent.Service.v1"
	ProxyPort            int                  // local mixed inbound port
	SelectionRepository  selection.Repository // local active-selection store
	RevisionRepository   revision.Repository  // local revision-history store
	CredentialStore      credential.Store     // optional override for tests
	DeferCoreStart       bool                 // combined desktop starts disconnected
	EnableExternalPipe   bool                 // standalone Service IPC only; disabled in desktop process
	TUNDataPlaneVerifier TUNDataPlaneVerifier // optional deterministic verifier for tests
}

// Service is the Windows Service that manages the proxy core and TUN infrastructure.
type Service struct {
	cfg            Config
	host           host.CoreHost
	sup            *supervisor.Supervisor
	selfHeal       *selfheal.Engine
	selfHealEvents <-chan supervisor.StateEvent

	reconciler  *network.Reconciler
	tunManager  tun.Manager
	tunVerifier TUNDataPlaneVerifier

	captureMu       sync.Mutex
	networkManager  tunNetworkManager
	tunRuntimeMu    sync.RWMutex
	tunStage        network.TUNStage
	tunSessionID    string
	tunAdapter      network.AdapterSnapshot
	tunVerification VerifyResult
	tunPlanMu       sync.RWMutex
	tunPlan         network.TUNActivationPlan
	tunFaultMu      sync.RWMutex
	tunFaultID      string
	tunFault        string
	coreDetectOnce  sync.Once
	coreDetections  []coreDetection

	subMgr             *subscription.Manager
	upstreamMgr        *upstreamproxy.Manager
	credentialStore    credential.Store
	collector          *monitor.Collector
	trafficSampler     monitor.TrafficSampler
	metricsMu          sync.Mutex
	metricsReader      coreadapter.MetricsReader
	metricsInitialized bool
	metricsConfig      string
	metricsCoreID      string
	metricsReason      string
	prober             outboundProber
	ipDetector         *ipdetect.Detector
	directIPDetector   *ipdetect.Detector
	endpointStatusMu   sync.RWMutex
	endpointStatuses   map[string]EndpointStatus
	diagnosticMu       sync.RWMutex
	exitIP             string
	exitCountry        string

	runtimeMu sync.Mutex
	runtime   runtimeState
	ipcReplay ipcReplayCache

	pipeListener  pipe.Listener
	storePath     string
	selectionRepo selection.Repository
	revisionRepo  revision.Repository
	coreAdapters  *coreadapter.Registry

	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	stopOnce  sync.Once
	readyCh   chan struct{}
	readyOnce sync.Once
}

type outboundProber interface {
	ProbeTCP(context.Context, string, string, int) *monitor.ProbeResult
}

// New creates a new Service with TUN infrastructure wired in.
func New(cfg Config) (*Service, error) {
	if cfg.SingBoxPath == "" {
		return nil, fmt.Errorf("SingBoxPath is required")
	}
	if cfg.PipeName == "" {
		cfg.PipeName = "Navo.Agent.Service.v1"
	}
	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 12080
	}

	// The launcher validates and pins this exact binary. Never replace it with
	// an environment/PATH discovery result after the trust decision.
	binaryPath := cfg.SingBoxPath
	var err error

	tunMgr := tun.NewManagerWithDLL(
		filepath.Join(filepath.Dir(cfg.SingBoxPath), "wintun.dll"),
	)
	routeMgr := tun.NewRouteManager()
	dnsMgr := tun.NewDNSManager()
	reconciler := network.NewReconciler(tunMgr, routeMgr, dnsMgr)
	if cfg.ConfigDir != "" {
		reconciler.SetStateFilePath(filepath.Join(cfg.ConfigDir, "recovery_state.json"))
	}

	credentialStore := cfg.CredentialStore
	if credentialStore == nil {
		if cfg.ConfigDir == "" {
			credentialStore = credential.NewMemoryStore()
		} else {
			credentialStore, err = credential.NewFileStore(
				filepath.Join(cfg.ConfigDir, "credentials.dpapi"),
			)
			if err != nil {
				return nil, fmt.Errorf("initialize credential store: %w", err)
			}
		}
	}
	subStorePath := ""
	if cfg.ConfigDir != "" {
		subStorePath = filepath.Join(cfg.ConfigDir, "subscriptions.json")
	}
	subMgr, err := subscription.NewManagerWithCredentialStore(subStorePath, credentialStore)
	if err != nil {
		return nil, fmt.Errorf("initialize subscription repository: %w", err)
	}
	upstreamPath := ""
	if cfg.ConfigDir != "" {
		upstreamPath = filepath.Join(cfg.ConfigDir, "upstream_proxies.json")
	}
	upstreamMgr, err := upstreamproxy.NewManager(upstreamPath)
	if err != nil {
		return nil, fmt.Errorf("initialize upstream proxy repository: %w", err)
	}
	collector := monitor.NewCollector()
	prober := monitor.NewProber()
	ipDet := ipdetect.NewDetectorWithProxy(
		fmt.Sprintf("http://127.0.0.1:%d", cfg.ProxyPort),
	)
	directIPDet := ipdetect.NewDetector()

	runtime := loadRuntimeState(cfg.ConfigDir)
	coreHost, err := newCoreHost(cfg, runtime.CoreID, binaryPath)
	if err != nil {
		log.Printf("[service] persisted core %q unavailable, falling back to sing-box: %v", runtime.CoreID, err)
		runtime.CoreID = "sing-box"
		coreHost = host.NewSingBoxHost(binaryPath, "")
	}
	sup := supervisor.NewSupervisor(coreHost, reconciler)
	selfHealState := ""
	if cfg.ConfigDir != "" {
		selfHealState = filepath.Join(cfg.ConfigDir, "state", "selfheal-state.json")
	}
	selfHealRegistry, err := selfheal.NewRegistry(selfheal.DefaultObserverPolicies()...)
	if err != nil {
		return nil, fmt.Errorf("initialize self-heal policy registry: %w", err)
	}
	selfHealEngine, err := selfheal.New(selfheal.DefaultConfig(selfHealState), selfHealRegistry)
	if err != nil {
		return nil, fmt.Errorf("initialize self-heal engine: %w", err)
	}

	service := &Service{
		cfg:              cfg,
		host:             coreHost,
		sup:              sup,
		selfHeal:         selfHealEngine,
		reconciler:       reconciler,
		tunManager:       tunMgr,
		subMgr:           subMgr,
		upstreamMgr:      upstreamMgr,
		credentialStore:  credentialStore,
		collector:        collector,
		prober:           prober,
		ipDetector:       ipDet,
		directIPDetector: directIPDet,
		endpointStatuses: make(map[string]EndpointStatus),
		selectionRepo:    cfg.SelectionRepository,
		revisionRepo:     cfg.RevisionRepository,
		coreAdapters:     coreadapter.NewDefaultRegistry(),
		tunVerifier:      cfg.TUNDataPlaneVerifier,
		stopCh:           make(chan struct{}),
		readyCh:          make(chan struct{}),
	}
	if service.tunVerifier == nil {
		service.tunVerifier = newTUNDataPlaneVerifier()
	}
	service.runtime = runtime
	return service, nil
}

func newCoreHost(cfg Config, coreID, singBoxPath string) (host.CoreHost, error) {
	switch coreID {
	case "", "sing-box":
		return host.NewSingBoxHost(singBoxPath, ""), nil
	case "mihomo":
		if cfg.MihomoPath == "" {
			return nil, fmt.Errorf("mihomo binary is not installed")
		}
		return host.NewMihomoHost(cfg.MihomoPath, ""), nil
	case "xray":
		if cfg.XrayPath == "" {
			return nil, fmt.Errorf("xray binary is not installed")
		}
		return host.NewXrayHost(cfg.XrayPath, ""), nil
	default:
		return nil, fmt.Errorf("unsupported core %q", coreID)
	}
}

// Run starts the service. On Windows, this is called from the SCM handler.
// On non-Windows or standalone mode, it runs until interrupted.
func (s *Service) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("service already running")
	}
	s.running = true
	s.mu.Unlock()

	log.Printf("[service] starting Navo Service")
	log.Printf("[service] sing-box: %s", s.cfg.SingBoxPath)
	stopSupervisor := func() error {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), captureRollbackTimeout)
		defer stopCancel()
		return s.sup.Stop(stopCtx)
	}
	// Recover the exact V2 network journal before compiling or starting a core.
	recoveryManager, err := s.newTUNRecoveryManager(s.runtimeTUNName())
	if err != nil {
		return fmt.Errorf("initialize TUN recovery: %w", err)
	}
	s.networkManager = recoveryManager
	if err := recoveryManager.Recover(ctx); err != nil {
		return fmt.Errorf("recover TUN network transaction: %w", err)
	}
	s.networkManager = nil

	// Always compile the initial configuration for the persisted core. The
	// launcher bootstrap file is sing-box JSON and must never be passed to a
	// restored Mihomo or Xray host.
	if err := s.prepareRuntimeConfig(ctx); err != nil {
		return fmt.Errorf("restore persisted runtime: %w", err)
	}

	// Start the supervisor. Use the config path that applyRuntimeConfig may have
	// updated (with persisted outbounds), falling back to the initial config.
	configPath := s.cfg.ConfigPath
	if configPath == "" {
		configPath = "configs/test_direct.json"
	}

	if s.cfg.DeferCoreStart {
		log.Printf("[service] core start deferred until capture mode activation")
	} else {
		if err := s.sup.Start(ctx, configPath); err != nil {
			return fmt.Errorf("failed to start supervisor: %w", err)
		}
		if err := s.commitHealthyRuntime(ctx); err != nil {
			_ = stopSupervisor()
			return fmt.Errorf("commit healthy runtime: %w", err)
		}
		log.Printf("[service] supervisor running (PID: %d)", s.sup.Status().PID)
	}
	if err := s.selfHeal.Start(ctx); err != nil {
		if s.sup.State() != supervisor.StateStopped {
			_ = stopSupervisor()
		}
		return fmt.Errorf("start self-heal engine: %w", err)
	}
	s.selfHealEvents = s.sup.Events()
	go s.watchSupervisorEvents(ctx, s.selfHealEvents)
	go s.monitorTUNAdapter(ctx)

	if s.cfg.EnableExternalPipe {
		listener, err := pipe.NewListener(s.cfg.PipeName)
		if err != nil {
			s.selfHeal.Stop()
			if s.sup.State() != supervisor.StateStopped {
				_ = stopSupervisor()
			}
			return fmt.Errorf("start service pipe: %w", err)
		}
		s.pipeListener = listener
		log.Printf("[service] pipe listening on %s", listener.Addr())
		go s.acceptConnections(ctx)
	}
	s.readyOnce.Do(func() { close(s.readyCh) })

	// Wait for stop signal
	select {
	case <-ctx.Done():
		log.Printf("[service] context cancelled")
	case <-s.stopCh:
		log.Printf("[service] stop requested")
	}

	return s.shutdown()
}

// Ready is closed only after runtime restoration and optional IPC startup
// complete successfully.
func (s *Service) Ready() <-chan struct{} { return s.readyCh }

func (s *Service) prepareRuntimeConfig(ctx context.Context) error {
	return s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", "")
}

// Stop signals the service to stop gracefully.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.stopOnce.Do(func() {
			close(s.stopCh)
		})
	}
}

// Status returns current service status.
func (s *Service) Status() supervisor.SupervisorStatus {
	return s.sup.Status()
}

// shutdown performs graceful shutdown.
func (s *Service) shutdown() error {
	log.Printf("[service] shutting down...")
	var shutdownErr error
	if s.selfHealEvents != nil {
		s.sup.Unsubscribe(s.selfHealEvents)
		s.selfHealEvents = nil
	}
	if s.selfHeal != nil {
		s.selfHeal.Stop()
	}

	// Stop accepting new pipe connections
	if s.pipeListener != nil {
		s.pipeListener.Close()
	}

	// Wait for an in-flight selection/core switch before stopping the active host.
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(), captureRollbackTimeout,
	)
	defer shutdownCancel()
	if s.networkManager != nil {
		if err := s.deactivateTUNNetwork(shutdownCtx); err != nil {
			log.Printf("[service] restore TUN network state during shutdown: %v", err)
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	// Stop supervisor (which stops the currently selected core).
	if s.sup.State() != supervisor.StateStopped {
		if err := s.sup.Stop(shutdownCtx); err != nil {
			log.Printf("[service] supervisor stop error: %v", err)
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if err := s.tunManager.Destroy(shutdownCtx); err != nil {
		log.Printf("[service] release TUN adapter during shutdown: %v", err)
		shutdownErr = errors.Join(shutdownErr, err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	log.Printf("[service] shutdown complete")
	return shutdownErr
}

func (s *Service) watchSupervisorEvents(ctx context.Context, events <-chan supervisor.StateEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Event != supervisor.EventCrash {
				continue
			}
			s.runtimeMu.Lock()
			coreID := s.runtime.CoreID
			s.runtimeMu.Unlock()
			s.selfHeal.Submit(selfheal.ErrorEvent{
				Code: selfheal.CodeCoreCrashed, OccurredAt: event.Timestamp,
				SourceService: "Supervisor", ResourceID: "active-core", CoreID: coreID,
			})
		}
	}
}

// acceptConnections handles incoming pipe connections.
func (s *Service) acceptConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		ch, err := s.pipeListener.Accept()
		if err != nil {
			// Listener closed
			return
		}

		go s.handleConnection(ctx, ch)
	}
}

// handleConnection processes IPC messages from a single pipe connection.
func (s *Service) handleConnection(ctx context.Context, ch *pipe.Channel) {
	defer ch.Close()

	log.Printf("[service] new connection accepted")

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		// Set a read deadline so we don't block forever
		if err := ch.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			log.Printf("[service] set read deadline: %v", err)
			return
		}

		var msg map[string]interface{}
		if err := ch.Receive(&msg); err != nil {
			if !isExpectedPipeDisconnect(err) {
				log.Printf("[service] receive error: %v", err)
			}
			return
		}

		log.Printf("[service] received: method=%v", msg["method"])

		// Dispatch based on method
		requestCtx, cancel := context.WithTimeout(ctx, serviceIPCRequestTimeout(msg))
		response := s.dispatch(requestCtx, msg)
		cancel()
		if response != nil {
			if err := ch.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Printf("[service] set write deadline: %v", err)
				return
			}
			if err := ch.Send(response); err != nil {
				if !isExpectedPipeDisconnect(err) {
					log.Printf("[service] send error: %v", err)
				}
				return
			}
		}
	}
}

func serviceIPCRequestTimeout(msg map[string]interface{}) time.Duration {
	method, _ := msg["method"].(string)
	switch method {
	case "core.select", "core.update.stop", "core.update.start":
		return coreServiceIPCRequestTimeout
	case "capture.prepare", "network.recover", "outbound.select", "outbound.create", "outbound.delete",
		"subscription.add", "subscription.remove", "subscription.refresh",
		"runtime.verify",
		"runtime.mode.set", "runtime.rules.set", "runtime.list_mode.set":
		return captureServiceIPCRequestTimeout
	default:
		return defaultServiceIPCRequestTimeout
	}
}

func isExpectedPipeDisconnect(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") ||
		strings.Contains(message, "pipe has been ended") ||
		strings.Contains(message, "pipe is being closed")
}

// Dispatch routes an IPC message to the appropriate handler.
// Exported so the combined launcher can call it directly without a Named Pipe relay.
func (s *Service) Dispatch(ctx context.Context, msg map[string]interface{}) map[string]interface{} {
	return s.dispatch(ctx, msg)
}

func (s *Service) dispatch(ctx context.Context, msg map[string]interface{}) map[string]interface{} {
	requestID, _ := msg["request_id"].(string)
	if requestID == "" {
		return s.dispatchUncached(ctx, msg)
	}
	fingerprint, err := fingerprintIPCRequest(msg)
	if err != nil {
		return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("fingerprint IPC request: %w", err))
	}
	return s.ipcReplay.execute(ctx, requestID, fingerprint, func() map[string]interface{} {
		return s.dispatchUncached(ctx, msg)
	})
}

func (s *Service) dispatchUncached(ctx context.Context, msg map[string]interface{}) map[string]interface{} {
	method, _ := msg["method"].(string)
	requestID, _ := msg["request_id"].(string)
	_ = logstore.Emit(logstore.LevelDebug, logServiceForMethod(method), "IPC", "request received", map[string]any{
		"method": method, "request_id": requestID,
	})
	if _, supplied := msg["config_path"]; supplied {
		return errorResponse(requestID, "INVALID_ARGUMENT", fmt.Errorf("config_path is not accepted over IPC"))
	}

	switch method {
	case "core.status":
		return s.handleCoreStatus(requestID)
	case "core.list":
		return s.handleCoreList(requestID)
	case "core.select":
		return s.handleCoreSelect(ctx, requestID, msg)
	case "core.update.stop":
		return s.handleCoreUpdateStop(ctx, requestID, msg)
	case "core.update.start":
		return s.handleCoreUpdateStart(ctx, requestID, msg)
	case "tun.enable":
		return s.handleTUNEnable(ctx, requestID, msg)
	case "tun.disable":
		return s.handleTUNDisable(ctx, requestID)
	case "tun.status":
		return s.handleTUNStatus(ctx, requestID)
	case "tun.config":
		return s.handleTUNConfig(ctx, requestID, msg)
	case "capture.prepare":
		return s.handleCapturePrepare(ctx, requestID, msg)
	case "subscription.add":
		return s.handleSubAdd(ctx, requestID, msg)
	case "subscription.update":
		return s.handleSubUpdate(requestID, msg)
	case "subscription.remove":
		return s.handleSubRemove(ctx, requestID, msg)
	case "subscription.list":
		return s.handleSubList(requestID)
	case "subscription.refresh":
		return s.handleSubRefresh(ctx, requestID, msg)
	case "outbound.list":
		return s.handleOutboundList(requestID)
	case "outbound.select":
		return s.handleOutboundSelect(ctx, requestID, msg)
	case "outbound.create":
		return s.handleOutboundCreate(ctx, requestID, msg)
	case "outbound.delete":
		return s.handleOutboundDelete(ctx, requestID, msg)
	case "outbound.update":
		return s.handleOutboundUpdate(requestID, msg)
	case "outbound.test":
		return s.handleOutboundTest(requestID, msg)
	case "outbound.testAll":
		return s.handleOutboundTestAll(requestID)
	case "outbound.failover_candidates":
		return s.handleOutboundFailoverCandidates(ctx, requestID, msg)
	case "runtime.status":
		return s.handleRuntimeStatus(requestID)
	case "runtime.verify":
		return s.handleRuntimeVerify(ctx, requestID)
	case "runtime.mode.set":
		return s.handleRuntimeModeSet(ctx, requestID, msg)
	case "runtime.rules.set":
		return s.handleRuntimeRulesSet(ctx, requestID, msg)
	case "runtime.list_mode.set":
		return s.handleRuntimeListModeSet(ctx, requestID, msg)
	case "metrics.current":
		return s.handleMetricsCurrent(requestID)
	case "ip.check":
		return s.handleIPCheck(requestID)
	case "network.recover":
		return s.handleNetworkRecover(ctx, requestID)
	case "network.observe":
		return s.handleNetworkObserve(ctx, requestID)
	case "diagnostics.export":
		return s.handleDiagnosticsExport(ctx, requestID)
	case "core.log.tail":
		return s.handleCoreLogTail(requestID)
	case "log.tail":
		return s.handleLogTail(requestID)
	case "logs.query":
		return s.handleLogsQuery(requestID, msg)
	case "logs.services":
		return response(requestID, map[string]interface{}{"services": logstore.Default().Services()})
	case "logs.categories":
		return response(requestID, map[string]interface{}{"categories": logstore.Categories()})
	case "logs.levels":
		return response(requestID, map[string]interface{}{"levels": []string{"DEBUG", "INFO", "WARN", "ERROR"}})
	case "logs.clear.persisted":
		if err := logstore.Default().Clear(); err != nil {
			return errorResponse(requestID, "LOG_CLEAR_FAILED", err)
		}
		return response(requestID, map[string]interface{}{"cleared": true})
	default:
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "ERROR",
			"payload": map[string]interface{}{
				"code":    "METHOD_NOT_FOUND",
				"message": fmt.Sprintf("unknown method: %s", method),
			},
		}
	}
}

func (s *Service) handleCoreStatus(requestID string) map[string]interface{} {
	status := s.sup.Status()
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"core_id":        s.host.ID(),
			"state":          string(status.State),
			"pid":            status.PID,
			"uptime_seconds": int64(status.Uptime.Seconds()),
			"config_hash":    status.ConfigHash,
			"restart_count":  status.RestartCount,
			"last_error":     status.LastError,
		},
	}
}

func (s *Service) handleCoreList(requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	selectedID := s.runtime.SelectedOutbound
	s.runtimeMu.Unlock()
	var selected *compiler.Outbound
	for _, outbound := range s.currentOutbounds(context.Background()) {
		if outbound.ID == selectedID {
			copy := outbound
			selected = &copy
			break
		}
	}
	detections := s.detectCores()
	cores := make([]map[string]interface{}, 0, len(detections))
	for _, detection := range detections {
		item := map[string]interface{}{
			"id":                     detection.ID,
			"name":                   detection.Name,
			"version":                detection.Version,
			"installed":              detection.Installed,
			"active":                 detection.ID == s.host.ID(),
			"recommended":            detection.ID == "mihomo",
			"capture_modes":          detection.CaptureModes,
			"system_proxy_supported": detection.SystemProxy,
			"tun_supported":          detection.TUN,
			"controller_supported":   detection.Controller,
			"metrics_supported":      detection.Metrics,
			"detection_error":        detection.DetectionError,
		}
		switch {
		case !detection.Installed:
			item["color"] = "red"
			item["reason"] = "core binary is missing"
		case selected != nil && !compiler.Compatible(detection.ID, *selected):
			item["color"] = "yellow"
			item["reason"] = fmt.Sprintf(
				"current protocol %s is incompatible",
				selected.Type,
			)
		default:
			item["color"] = "green"
			item["reason"] = ""
		}
		cores = append(cores, item)
	}
	return response(requestID, map[string]interface{}{"active": s.host.ID(), "cores": cores})
}

func (s *Service) handleCoreSelect(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	coreID, _ := msg["core_id"].(string)
	if coreID == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("core_id is required"))
	}
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "CORE_SWITCH_BUSY", err)
	}
	defer s.captureMu.Unlock()
	if coreID == s.host.ID() {
		return response(requestID, map[string]interface{}{"active": coreID})
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.runtime.TUNEnabled && !s.coreSupportsCapture(coreID, capture.ModeTUN) {
		return errorResponse(
			requestID,
			"CORE_INCOMPATIBLE",
			fmt.Errorf("core %s does not support TUN; disable TUN before switching", coreID),
		)
	}
	nextHost, err := newCoreHost(s.cfg, coreID, s.cfg.SingBoxPath)
	if err != nil {
		return errorResponse(requestID, "CORE_NOT_INSTALLED", err)
	}
	wasRunning := s.sup.State() == supervisor.StateRunning
	previousHost, previousSupervisor := s.host, s.sup
	previousConfigPath := s.cfg.ConfigPath
	previousRuntime := s.runtime
	nextSupervisor := supervisor.NewSupervisor(nextHost, s.reconciler)
	restorePrevious := func(cause error) error {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rollbackCancel()
		var rollbackErr error
		if s.host == nextHost && nextSupervisor.State() != supervisor.StateStopped {
			if err := nextSupervisor.Stop(rollbackCtx); err != nil {
				forceErr := nextHost.Stop(rollbackCtx, true, 5*time.Second)
				rollbackErr = errors.Join(err, forceErr)
			}
		}
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("stop failed replacement core: %w", rollbackErr))
		}
		s.runtime = previousRuntime
		s.host, s.sup = previousHost, previousSupervisor
		s.cfg.ConfigPath = previousConfigPath
		if err := s.saveRuntimeStateLocked(s.cfg.ConfigDir); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
		if wasRunning && previousSupervisor.State() != supervisor.StateRunning {
			if err := previousSupervisor.Start(rollbackCtx, previousConfigPath); err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restart previous core: %w", err))
			}
		}
		return errors.Join(cause, rollbackErr)
	}
	if wasRunning {
		if err := previousSupervisor.Stop(ctx); err != nil {
			return errorResponse(requestID, "CORE_SWITCH_FAILED", err)
		}
	}
	s.runtime.CoreID = coreID
	s.host, s.sup = nextHost, nextSupervisor
	if err := s.applyRuntimeConfigLocked(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		return errorResponse(requestID, "CORE_SWITCH_FAILED", restorePrevious(err))
	}
	if wasRunning {
		if err := nextSupervisor.Start(ctx, s.cfg.ConfigPath); err != nil {
			return errorResponse(requestID, "CORE_SWITCH_FAILED", restorePrevious(err))
		}
		if err := s.commitHealthyRuntimeLocked(ctx); err != nil {
			return errorResponse(requestID, "CORE_SWITCH_COMMIT_FAILED", restorePrevious(err))
		}
	}
	return response(requestID, map[string]interface{}{"active": coreID})
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *Service) handleCoreUpdateStop(ctx context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	coreID, _ := msg["core_id"].(string)
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "CORE_UPDATE_BUSY", err)
	}
	defer s.captureMu.Unlock()
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if coreID == "" || coreID != s.host.ID() {
		return errorResponse(requestID, "CORE_UPDATE_TARGET_MISMATCH", errors.New("only the active core can be stopped for update"))
	}
	if s.sup.State() == supervisor.StateStopped {
		return response(requestID, map[string]interface{}{"stopped": true})
	}
	s.sup.SetRestartSuppressed(true)
	if err := s.sup.Stop(ctx); err != nil {
		s.sup.SetRestartSuppressed(false)
		return errorResponse(requestID, "CORE_UPDATE_STOP_FAILED", err)
	}
	return response(requestID, map[string]interface{}{"stopped": true})
}

func (s *Service) handleCoreUpdateStart(ctx context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	coreID, _ := msg["core_id"].(string)
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "CORE_UPDATE_BUSY", err)
	}
	defer s.captureMu.Unlock()
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if coreID == "" || coreID != s.host.ID() {
		return errorResponse(requestID, "CORE_UPDATE_TARGET_MISMATCH", errors.New("only the active core can be restarted after update"))
	}
	s.sup.SetRestartSuppressed(false)
	if s.sup.State() == supervisor.StateRunning {
		return response(requestID, map[string]interface{}{"running": true})
	}
	if err := s.sup.Start(ctx, s.cfg.ConfigPath); err != nil {
		return errorResponse(requestID, "CORE_UPDATE_START_FAILED", err)
	}
	return response(requestID, map[string]interface{}{"running": true})
}

func (s *Service) handleCoreRestart(requestID string) map[string]interface{} {
	err := s.sup.Restart(context.Background(), s.cfg.ConfigPath)
	if err != nil {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "ERROR",
			"payload": map[string]interface{}{
				"code":    "CORE_005",
				"message": err.Error(),
			},
		}
	}

	newStatus := s.sup.Status()
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"pid":    newStatus.PID,
			"status": "running",
		},
	}
}

func (s *Service) handleTUNEnable(ctx context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	name, nameErr := normalizeOwnedTUNName(name)
	if nameErr != nil {
		return errorResponse(requestID, "NET_ADAPTER_OWNERSHIP", nameErr)
	}
	mtu, _ := msg["mtu"].(float64)
	if mtu == 0 {
		mtu = 1500
	}
	if mtu < 1280 || mtu > 9000 {
		return errorResponse(
			requestID,
			"INVALID",
			fmt.Errorf("TUN MTU must be between 1280 and 9000"),
		)
	}
	if !s.wintunAvailable() {
		return errorResponse(
			requestID,
			"NET_001",
			fmt.Errorf("wintun.dll was not found beside sing-box.exe"),
		)
	}
	return s.handleCapturePrepare(
		ctx, requestID,
		map[string]interface{}{
			"mode": capture.ModeTUN.String(), "tun_name": name, "tun_mtu": int(mtu),
		},
	)
}

func (s *Service) handleTUNDisable(ctx context.Context, requestID string) map[string]interface{} {
	return s.handleCapturePrepare(
		ctx, requestID,
		map[string]interface{}{"mode": capture.ModeSystemProxy.String()},
	)
}

func (s *Service) handleTUNStatus(ctx context.Context, requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	enabled := s.runtime.TUNEnabled
	name := s.runtime.TUNName
	mtu := s.runtime.TUNMTU
	s.runtimeMu.Unlock()
	if strings.TrimSpace(name) == "" {
		name = "Navo"
	}
	// One native observation is authoritative for state and identity. The
	// committed snapshot contributes addresses/MTU only when that identity still
	// matches, avoiding mixed-instant status assembled from two system queries.
	adapterStatus := tun.InspectAdapter(ctx, name)
	s.tunRuntimeMu.RLock()
	stage, sessionID, adapter, verification := s.tunStage, s.tunSessionID, s.tunAdapter, s.tunVerification
	s.tunRuntimeMu.RUnlock()
	expected := tunHealthExpectation{
		sessionID: sessionID, name: name,
		guid: adapter.InterfaceGUID, index: int(adapter.InterfaceIndex),
	}
	identityMatches := adapterStatus.InterfaceIndex > 0 && adapterStatus.InterfaceGUID != "" &&
		tunObservationMatchesIdentity(expected, adapterStatus)
	addresses := []string(nil)
	observedMTU := mtu
	if identityMatches {
		addresses = adapter.IPv4Addresses
		observedMTU = firstPositive(adapter.MTU, mtu)
	}
	faultID, fault := s.currentTUNFault()
	healthy := enabled && sessionID != "" && stage == network.TUNStageHealthCommitted &&
		s.sup.State() == supervisor.StateRunning && tunObservationHealthy(expected, adapterStatus)
	routeCount := 0
	if healthy {
		routeCount = 2
	}
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"installed":         s.wintunAvailable(),
			"created":           adapterStatus.State != capture.AdapterMissing,
			"enabled":           healthy,
			"name":              adapterStatus.Name,
			"state":             adapterStatus.State,
			"identifier":        adapterStatus.InterfaceGUID,
			"interface_index":   adapterStatus.InterfaceIndex,
			"addresses":         addresses,
			"mtu":               observedMTU,
			"route_count":       routeCount,
			"stage":             stage,
			"verification":      verification,
			"fault_id":          faultID,
			"last_error":        fault,
			"observation_error": adapterStatus.Error,
		},
	}
}

func firstPositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (s *Service) handleTUNConfig(ctx context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "TUN_CONFIG_BUSY", err)
	}
	defer s.captureMu.Unlock()
	s.runtimeMu.Lock()
	currentName := s.runtime.TUNName
	currentMTU := s.runtime.TUNMTU
	configuredName := currentName
	configuredMTU := currentMTU
	enabled := s.runtime.TUNEnabled
	s.runtimeMu.Unlock()
	if name == "" {
		name = currentName
	}
	ownedName, nameErr := normalizeOwnedTUNName(name)
	if nameErr != nil {
		return errorResponse(requestID, "NET_ADAPTER_OWNERSHIP", nameErr)
	}
	name = ownedName
	if mtu, ok := msg["mtu"].(float64); ok {
		currentMTU = int(mtu)
	}
	if currentMTU < 1280 || currentMTU > 9000 {
		return errorResponse(
			requestID,
			"INVALID",
			fmt.Errorf("TUN MTU must be between 1280 and 9000"),
		)
	}
	if enabled && (name != configuredName || currentMTU != configuredMTU) {
		return errorResponse(
			requestID,
			"TUN_RESTART_REQUIRED",
			fmt.Errorf("disable TUN before changing its adapter configuration"),
		)
	}
	if enabled {
		return response(requestID, map[string]interface{}{"status": "unchanged"})
	}
	s.runtimeMu.Lock()
	previous := s.runtime
	s.runtime.TUNName = name
	s.runtime.TUNMTU = currentMTU
	configDir := s.cfg.ConfigDir
	if configDir == "" {
		configDir = filepath.Dir(s.cfg.ConfigPath)
	}
	err := s.saveRuntimeStateLocked(configDir)
	if err != nil {
		s.runtime = previous
	}
	s.runtimeMu.Unlock()
	if err != nil {
		return errorResponse(requestID, "NET_007", err)
	}
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload":    map[string]interface{}{"status": "ok"},
	}
}

// ── Subscription handlers ──

func (s *Service) handleSubAdd(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	url, _ := msg["url"].(string)
	skipTLSVerify, _ := msg["skip_tls_verify"].(bool)
	if name == "" || url == "" {
		return map[string]interface{}{
			"request_id": requestID, "type": "ERROR",
			"payload": map[string]interface{}{"code": "INVALID", "message": "name and url required"},
		}
	}
	ctx, cancel := context.WithTimeout(parent, captureServiceIPCRequestTimeout)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "SUB_BUSY", err)
	}
	defer s.captureMu.Unlock()
	sub, err := s.subMgr.AddWithOptions(name, url, skipTLSVerify)
	if err != nil {
		return errorResponse(requestID, "SUB_001", err)
	}
	if boolField(msg, "wait") {
		outbounds, refreshErr := s.subMgr.Refresh(ctx)
		if refreshErr != nil {
			return errorResponse(requestID, "SUB_002", refreshErr)
		}
		if len(outbounds) > 0 {
			if applyErr := s.applyRuntimeConfig(
				ctx,
				s.currentOutbounds(ctx),
				"",
				"",
			); applyErr != nil {
				return errorResponse(requestID, "SUB_003", applyErr)
			}
		}
		return response(requestID, map[string]interface{}{
			"id": sub.ID, "status": "added", "node_count": len(outbounds),
		})
	}
	// Persist-only is intentionally side-effect free. A refresh that may replace
	// the active core configuration must remain inside an Agent-owned connection
	// transaction and therefore has to be requested with wait=true.
	return response(requestID, map[string]interface{}{
		"id": sub.ID, "status": "added_pending_refresh",
	})
}

func (s *Service) handleSubUpdate(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	skipTLSVerify, ok := msg["skip_tls_verify"].(bool)
	if strings.TrimSpace(id) == "" || !ok {
		return errorResponse(
			requestID,
			"INVALID",
			fmt.Errorf("id and skip_tls_verify are required"),
		)
	}
	sub, err := s.subMgr.UpdateTLSCompatibility(id, skipTLSVerify)
	if err != nil {
		return errorResponse(requestID, "SUB_007", err)
	}
	return response(requestID, map[string]interface{}{
		"id": sub.ID, "skip_tls_verify": sub.SkipTLSVerify,
	})
}

func (s *Service) handleSubRemove(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	ctx, cancel := context.WithTimeout(parent, captureServiceIPCRequestTimeout)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "SUB_BUSY", err)
	}
	defer s.captureMu.Unlock()
	removal, ok, err := s.subMgr.Detach(id)
	if err != nil {
		return errorResponse(requestID, "SUB_006", err)
	}
	if !ok {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("subscription not found"))
	}
	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		return errorResponse(requestID, "SUB_008", errors.Join(err, s.subMgr.RestoreRemoval(removal)))
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()
	if err := s.subMgr.FinalizeRemoval(cleanupCtx, removal); err != nil {
		log.Printf("[service] removed subscription %q but retained its unreachable credential for later cleanup: %v", id, err)
		return response(requestID, map[string]interface{}{
			"status": "removed", "cleanup_warning": err.Error(),
		})
	}
	return response(requestID, map[string]interface{}{"status": "removed"})
}

func (s *Service) handleSubList(requestID string) map[string]interface{} {
	subs := s.subMgr.List()
	list := make([]interface{}, len(subs))
	for i, sub := range subs {
		list[i] = map[string]interface{}{
			"id": sub.ID, "name": sub.Name, "configured": sub.URLCredentialRef != "", "enabled": sub.Enabled,
			"skip_tls_verify": sub.SkipTLSVerify,
			"node_count":      sub.NodeCount, "last_error": sub.LastError,
			"created_at": sub.CreatedAt, "updated_at": sub.UpdatedAt,
		}
	}
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{"subscriptions": list},
	}
}

func (s *Service) handleSubRefresh(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	if !boolField(msg, "wait") {
		return errorResponse(
			requestID,
			"SUB_WAIT_REQUIRED",
			fmt.Errorf("subscription refresh must wait for the Agent connection transaction"),
		)
	}
	ctx, cancel := context.WithTimeout(ctx, captureServiceIPCRequestTimeout)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "SUB_BUSY", err)
	}
	defer s.captureMu.Unlock()
	outbounds, err := s.subMgr.Refresh(ctx)
	if err != nil {
		return errorResponse(requestID, "SUB_004", err)
	}
	if len(outbounds) > 0 {
		if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
			return errorResponse(requestID, "SUB_005", err)
		}
	}
	return response(requestID, map[string]interface{}{
		"status": "updated", "node_count": len(outbounds),
	})
}

func (s *Service) handleOutboundList(requestID string) map[string]interface{} {
	outbounds := s.currentOutbounds(context.Background())
	s.runtimeMu.Lock()
	selected := s.runtime.SelectedOutbound
	active := activeOutboundID(s.runtime)
	candidate := candidateOutboundID(s.runtime)
	mode := s.runtime.Mode
	s.runtimeMu.Unlock()
	list := make([]map[string]interface{}, 0, len(outbounds))
	for _, outbound := range outbounds {
		sourceType := "airport_subscription"
		if outbound.ProviderID == "upstream_proxy" {
			sourceType = "upstream_proxy"
		}
		status := s.endpointStatus(outbound)
		item := map[string]interface{}{
			"id": outbound.ID, "name": outbound.Name, "type": outbound.Type,
			"server": outbound.Server, "port": outbound.Port,
			"enabled": outbound.Enabled, "provider_id": outbound.ProviderID,
			"country": outbound.Country, "active": outbound.ID == active,
			"candidate": outbound.ID == candidate, "selected": outbound.ID == selected,
			"source_type": sourceType,
			"available":   status.Available, "color": status.Color,
			"reason": status.Reason, "checked_at": status.CheckedAt,
			"latency_ms": status.LatencyMS,
		}
		list = append(list, item)
	}
	return response(requestID, map[string]interface{}{
		"outbounds":    list,
		"selected_id":  selected,
		"active_id":    active,
		"candidate_id": candidate,
		"mode":         mode,
	})
}

func (s *Service) handleMetricsCurrent(requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	activeID := s.runtime.SelectedOutbound
	mode := s.runtime.Mode
	configPath := s.cfg.ConfigPath
	coreID := s.runtime.CoreID
	s.runtimeMu.Unlock()
	stats := s.collector.Stats()
	totalUp, totalDown := int64(0), int64(0)
	connections := 0
	for _, st := range stats {
		totalUp += st.Upload
		totalDown += st.Download
		connections += st.Connections
	}
	available := false
	reason := "current core does not expose metrics"
	if s.sup.State() == supervisor.StateRunning {
		ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
		coreMetrics, metricsErr := s.readCoreMetrics(ctx, coreID, configPath)
		cancel()
		if metricsErr == nil {
			totalUp = int64(coreMetrics.UploadBytes)
			totalDown = int64(coreMetrics.DownloadBytes)
			connections = coreMetrics.Connections
			available = true
			reason = ""
		} else {
			reason = metricsErr.Error()
		}
	}
	localUp, localDown, localErr := monitor.ReadSystemBytes()
	proxyUp, proxyDown := uint64(0), uint64(0)
	if available {
		proxyUp, proxyDown = uint64(max(totalUp, 0)), uint64(max(totalDown, 0))
	}
	traffic := s.trafficSampler.Sample(
		time.Now(), localUp, localDown, proxyUp, proxyDown,
		localErr == nil, available,
	)
	localReason := ""
	if localErr != nil {
		localReason = localErr.Error()
	}
	payload := map[string]interface{}{
		"mode":                     mode,
		"active_outbound":          activeID,
		"reachable":                s.sup.State() == supervisor.StateRunning,
		"available":                available,
		"unavailable_reason":       reason,
		"core_name":                coreID,
		"latency_ms":               nil,
		"upload_bytes":             totalUp,
		"download_bytes":           totalDown,
		"connections":              connections,
		"local_available":          localErr == nil,
		"local_unavailable_reason": localReason,
		"local_upload_bps":         traffic.LocalUploadBPS,
		"local_download_bps":       traffic.LocalDownloadBPS,
		"proxy_upload_bps":         traffic.ProxyUploadBPS,
		"proxy_download_bps":       traffic.ProxyDownloadBPS,
		"local_upload_total":       traffic.LocalUploadTotal,
		"local_download_total":     traffic.LocalDownloadTotal,
		"proxy_upload_total":       traffic.ProxyUploadTotal,
		"proxy_download_total":     traffic.ProxyDownloadTotal,
		"traffic_source_state":     traffic.SourceState,
		"traffic_sampled_at":       traffic.Timestamp,
	}
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": payload,
	}
}

func (s *Service) handleIPCheck(requestID string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// A user-triggered dual-link check must not reuse results from a previous
	// capture mode. Direct remains independently useful while capture is off.
	var sourceResult, proxyResult *ipdetect.IPResult
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		res, err := s.directIPDetector.CheckFresh(ctx, "source")
		if err != nil {
			sourceResult = &ipdetect.IPResult{
				OutboundID: "source",
				Error:      err.Error(),
				CheckedAt:  time.Now(),
			}
		} else {
			sourceResult = res
		}
	}()

	proxyState := "inactive"
	if s.sup.State() == supervisor.StateRunning {
		proxyState = "unavailable"
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.ipDetector.CheckFresh(ctx, "current")
			if err != nil {
				proxyResult = &ipdetect.IPResult{
					OutboundID: "current",
					Error:      err.Error(),
					CheckedAt:  time.Now(),
				}
				return
			}
			proxyResult = res
			proxyState = "available"
		}()
	} else {
		proxyResult = &ipdetect.IPResult{
			OutboundID: "current",
			Error:      "代理或 TUN 未启用",
			CheckedAt:  time.Now(),
		}
	}

	wg.Wait()
	if net.ParseIP(strings.TrimSpace(proxyResult.IP)) != nil {
		s.diagnosticMu.Lock()
		s.exitIP = proxyResult.IP
		s.exitCountry = proxyResult.Country
		s.diagnosticMu.Unlock()
	}
	sourceState := "unavailable"
	if net.ParseIP(strings.TrimSpace(sourceResult.IP)) != nil && sourceResult.Error == "" {
		sourceState = "available"
	}
	connectionKind := "direct"
	if s.sup.State() == supervisor.StateRunning {
		connectionKind = "navo"
	}

	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{
			"connection_kind": connectionKind,
			"source": map[string]interface{}{
				"available":   sourceState == "available",
				"state":       sourceState,
				"outbound_id": sourceResult.OutboundID,
				"ip":          sourceResult.IP,
				"country":     sourceResult.Country,
				"city":        sourceResult.City,
				"asn":         sourceResult.ASN,
				"isp":         sourceResult.ISP,
				"network":     sourceResult.Network,
				"provider":    sourceResult.Provider,
				"mobile":      sourceResult.Mobile,
				"proxy":       sourceResult.Proxy,
				"hosting":     sourceResult.Hosting,
				"checked_at":  sourceResult.CheckedAt,
				"error":       sourceResult.Error,
			},
			"proxy": map[string]interface{}{
				"available":   proxyState == "available",
				"state":       proxyState,
				"outbound_id": proxyResult.OutboundID,
				"ip":          proxyResult.IP,
				"country":     proxyResult.Country,
				"city":        proxyResult.City,
				"asn":         proxyResult.ASN,
				"isp":         proxyResult.ISP,
				"network":     proxyResult.Network,
				"provider":    proxyResult.Provider,
				"mobile":      proxyResult.Mobile,
				"proxy":       proxyResult.Proxy,
				"hosting":     proxyResult.Hosting,
				"checked_at":  proxyResult.CheckedAt,
				"error":       proxyResult.Error,
			},
		},
	}
}

// ── Standalone runner (for development/testing) ──

// handleLogTail returns the last N lines of the navo log file.
func (s *Service) handleLogTail(requestID string) map[string]interface{} {
	const tailLines = 200
	// Derive log dir from SingBoxPath: <exe>/third_party/sing-box/sing-box.exe -> <exe>/log/navo.log
	exeDir := filepath.Dir(filepath.Dir(filepath.Dir(s.cfg.SingBoxPath)))
	logPath := filepath.Join(exeDir, "log", "navo.log")
	if _, err := os.Stat(logPath); err != nil {
		// Use sing-box log as fallback (also contains useful info).
		logPath = filepath.Join(s.cfg.ConfigDir, "sing-box.log")
	}
	f, err := os.Open(logPath)
	if err != nil {
		return errorResponse(requestID, "LOG_READ_ERROR", fmt.Errorf("open log file: %w", err))
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if start := len(lines) - tailLines; start > 0 {
		lines = lines[start:]
	}
	return response(requestID, map[string]interface{}{
		"path":  logPath,
		"lines": lines,
	})
}

func (s *Service) handleCoreLogTail(requestID string) map[string]interface{} {
	const tailLines = 200
	s.runtimeMu.Lock()
	lines, err := s.host.GetLogs(tailLines)
	s.runtimeMu.Unlock()
	if err != nil {
		return errorResponse(
			requestID,
			"CORE_LOG_READ_ERROR",
			fmt.Errorf("read core log: %w", err),
		)
	}
	return response(requestID, map[string]interface{}{
		"lines": lines,
	})
}

// RunStandalone runs the service in standalone mode until Ctrl+C.
func RunStandalone(svc *Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		log.Println("[service] received interrupt, stopping...")
		cancel()
	}()

	return svc.Run(ctx)
}
