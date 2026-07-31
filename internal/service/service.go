// Package service provides the Windows Service wrapper for the Navo proxy core.
// It integrates with the Windows Service Control Manager (SCM) and wraps
// the Supervisor to manage sing-box lifecycle.
//
// Architecture: Service (Session 0) → Supervisor → CoreHost → sing-box
package service

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"navo/internal/ai"
	"navo/internal/compiler"
	"navo/internal/coreadapter"
	"navo/internal/credential"
	"navo/internal/domain/capture"
	"navo/internal/domain/revision"
	"navo/internal/domain/selection"
	"navo/internal/host"
	"navo/internal/ipdetect"
	"navo/internal/monitor"
	"navo/internal/network"
	"navo/internal/network/tun"
	"navo/internal/pipe"
	"navo/internal/subscription"
	"navo/internal/supervisor"
	"navo/internal/upstreamproxy"
)

// Config holds the service configuration.
type Config struct {
	SingBoxPath         string               // path to sing-box.exe
	MihomoPath          string               // optional path to mihomo.exe
	XrayPath            string               // optional path to xray.exe
	ConfigPath          string               // path to sing-box config JSON
	ConfigDir           string               // directory for config storage
	PipeName            string               // named pipe name for IPC, e.g. "Navo.Agent.Service.v1"
	ProxyPort           int                  // local mixed inbound port
	SelectionRepository selection.Repository // optional cloud-backed selection store
	RevisionRepository  revision.Repository  // optional cloud-backed revision store
	CredentialStore     credential.Store     // optional override for tests
	DeferCoreStart      bool                 // combined desktop starts disconnected
}

// Service is the Windows Service that manages the proxy core and TUN infrastructure.
type Service struct {
	cfg  Config
	host host.CoreHost
	sup  *supervisor.Supervisor

	reconciler *network.Reconciler
	tunManager tun.Manager

	captureMu      sync.Mutex
	networkManager *network.Manager
	tunFaultMu     sync.RWMutex
	tunFaultID     string
	tunFault       string
	coreDetectOnce sync.Once
	coreDetections []coreDetection

	subMgr             *subscription.Manager
	upstreamMgr        *upstreamproxy.Manager
	credentialStore    credential.Store
	collector          *monitor.Collector
	metricsMu          sync.Mutex
	metricsReader      coreadapter.MetricsReader
	metricsInitialized bool
	metricsConfig      string
	metricsCoreID      string
	metricsReason      string
	prober             *monitor.Prober
	ipDetector         *ipdetect.Detector
	directIPDetector   *ipdetect.Detector
	aiAssistant        *ai.Assistant
	aiConfig           ai.Config
	aiMu               sync.RWMutex
	endpointStatusMu   sync.RWMutex
	endpointStatuses   map[string]EndpointStatus
	diagnosticMu       sync.RWMutex
	exitIP             string
	exitCountry        string

	runtimeMu sync.Mutex
	runtime   runtimeState

	pipeListener  pipe.Listener
	storePath     string
	selectionRepo selection.Repository
	revisionRepo  revision.Repository
	coreAdapters  *coreadapter.Registry

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	stopOnce sync.Once
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

	binaryPath, err := host.FindBinary()
	if err != nil {
		binaryPath = cfg.SingBoxPath
	}

	tunMgr := tun.NewManagerWithDLL(
		filepath.Join(filepath.Dir(cfg.SingBoxPath), "wintun.dll"),
	)
	routeMgr := tun.NewRouteManager()
	dnsMgr := tun.NewDNSManager()
	reconciler := network.NewReconciler(tunMgr, routeMgr, dnsMgr)

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

	aiCfg := ai.Config{
		BaseURL:        os.Getenv("NAVO_AI_BASE_URL"),
		APIKey:         os.Getenv("NAVO_AI_API_KEY"),
		Model:          os.Getenv("NAVO_AI_MODEL"),
		TimeoutSeconds: 60,
	}
	if stored, err := loadAIConfig(cfg.ConfigDir); err == nil {
		aiCfg = stored
	} else if !os.IsNotExist(err) {
		log.Printf("[service] AI settings load failed: %v", err)
	}
	if aiCfg.Model == "" {
		aiCfg.Model = "deepseek-v4-pro"
	}
	aiBackend := ai.NewHTTPBackend(aiCfg)
	aiAssistant := ai.NewAssistant(aiBackend)

	runtime := loadRuntimeState(cfg.ConfigDir)
	coreHost, err := newCoreHost(cfg, runtime.CoreID, binaryPath)
	if err != nil {
		log.Printf("[service] persisted core %q unavailable, falling back to sing-box: %v", runtime.CoreID, err)
		runtime.CoreID = "sing-box"
		coreHost = host.NewSingBoxHost(binaryPath, "")
	}
	sup := supervisor.NewSupervisor(coreHost, reconciler)

	service := &Service{
		cfg:              cfg,
		host:             coreHost,
		sup:              sup,
		reconciler:       reconciler,
		tunManager:       tunMgr,
		subMgr:           subMgr,
		upstreamMgr:      upstreamMgr,
		credentialStore:  credentialStore,
		collector:        collector,
		prober:           prober,
		ipDetector:       ipDet,
		directIPDetector: directIPDet,
		aiAssistant:      aiAssistant,
		aiConfig:         aiCfg,
		endpointStatuses: make(map[string]EndpointStatus),
		selectionRepo:    cfg.SelectionRepository,
		revisionRepo:     cfg.RevisionRepository,
		coreAdapters:     coreadapter.NewDefaultRegistry(),
		stopCh:           make(chan struct{}),
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
			_ = s.sup.Stop(context.Background())
			return fmt.Errorf("commit healthy runtime: %w", err)
		}
		log.Printf("[service] supervisor running (PID: %d)", s.sup.Status().PID)
	}
	go s.monitorTUNAdapter(ctx)

	// Start named pipe listener
	listener, err := pipe.NewListener(s.cfg.PipeName)
	if err != nil {
		log.Printf("[service] WARNING: pipe listener failed: %v", err)
	} else {
		s.pipeListener = listener
		log.Printf("[service] pipe listening on %s", listener.Addr())

		// Accept connections in background
		go s.acceptConnections(ctx)
	}

	// Wait for stop signal
	select {
	case <-ctx.Done():
		log.Printf("[service] context cancelled")
	case <-s.stopCh:
		log.Printf("[service] stop requested")
	}

	return s.shutdown()
}

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
		if err := s.networkManager.Deactivate(shutdownCtx); err != nil {
			log.Printf("[service] restore TUN network state during shutdown: %v", err)
		}
		s.networkManager = nil
	}
	// Stop supervisor (which stops the currently selected core).
	if s.sup.State() != supervisor.StateStopped {
		if err := s.sup.Stop(shutdownCtx); err != nil {
			log.Printf("[service] supervisor stop error: %v", err)
		}
	}
	if err := s.tunManager.Destroy(shutdownCtx); err != nil {
		log.Printf("[service] release TUN adapter during shutdown: %v", err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	log.Printf("[service] shutdown complete")
	return nil
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
		ch.SetDeadline(time.Now().Add(30 * time.Second))

		var msg map[string]interface{}
		if err := ch.Receive(&msg); err != nil {
			if !isExpectedPipeDisconnect(err) {
				log.Printf("[service] receive error: %v", err)
			}
			return
		}

		log.Printf("[service] received: method=%v", msg["method"])

		// Dispatch based on method
		response := s.dispatch(ctx, msg)
		if response != nil {
			ch.Send(response)
		}
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
	method, _ := msg["method"].(string)
	requestID, _ := msg["request_id"].(string)

	switch method {
	case "core.start":
		return s.handleCoreStart(requestID, msg)
	case "core.stop":
		return s.handleCoreStop(requestID)
	case "core.status":
		return s.handleCoreStatus(requestID)
	case "core.list":
		return s.handleCoreList(requestID)
	case "core.select":
		return s.handleCoreSelect(ctx, requestID, msg)
	case "core.restart":
		return s.handleCoreRestart(requestID, msg)
	case "service.shutdown":
		// Respond first so the caller can observe a clean shutdown handshake.
		go func() {
			time.Sleep(50 * time.Millisecond)
			s.Stop()
		}()
		return response(requestID, map[string]interface{}{"status": "stopping"})
	case "tun.enable":
		return s.handleTUNEnable(requestID, msg)
	case "tun.disable":
		return s.handleTUNDisable(requestID)
	case "tun.status":
		return s.handleTUNStatus(requestID)
	case "tun.config":
		return s.handleTUNConfig(requestID, msg)
	case "capture.prepare":
		return s.handleCapturePrepare(ctx, requestID, msg)
	case "subscription.add":
		return s.handleSubAdd(requestID, msg)
	case "subscription.update":
		return s.handleSubUpdate(requestID, msg)
	case "subscription.remove":
		return s.handleSubRemove(requestID, msg)
	case "subscription.list":
		return s.handleSubList(requestID)
	case "subscription.refresh":
		return s.handleSubRefresh(ctx, requestID, msg)
	case "outbound.list":
		return s.handleOutboundList(requestID)
	case "outbound.select":
		return s.handleOutboundSelect(requestID, msg)
	case "outbound.create":
		return s.handleOutboundCreate(requestID, msg)
	case "outbound.delete":
		return s.handleOutboundDelete(requestID, msg)
	case "outbound.update":
		return s.handleOutboundUpdate(requestID, msg)
	case "outbound.test":
		return s.handleOutboundTest(requestID, msg)
	case "outbound.testAll":
		return s.handleOutboundTestAll(requestID)
	case "runtime.status":
		return s.handleRuntimeStatus(requestID)
	case "runtime.mode.set":
		return s.handleRuntimeModeSet(requestID, msg)
	case "metrics.current":
		return s.handleMetricsCurrent(requestID)
	case "ip.check":
		return s.handleIPCheck(requestID)
	case "network.recover":
		return s.handleNetworkRecover(ctx, requestID)
	case "diagnostics.export":
		return s.handleDiagnosticsExport(ctx, requestID)
	case "core.log.tail":
		return s.handleCoreLogTail(requestID)
	case "ai.rule.generate":
		return s.handleAIRuleGenerate(requestID, msg)
	case "ai.diagnose":
		return s.handleAIDiagnose(requestID)
	case "ai.explain":
		return s.handleAIExplain(requestID)
	case "ai.config.get":
		return s.handleAIConfigGet(requestID)
	case "ai.config.set":
		return s.handleAIConfigSet(requestID, msg)
	case "ai.config.test":
		return s.handleAIConfigTest(requestID)
	case "log.tail":
		return s.handleLogTail(requestID)
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

func (s *Service) handleCoreStart(requestID string, msg map[string]interface{}) map[string]interface{} {
	configPath, _ := msg["config_path"].(string)
	if configPath == "" {
		configPath = s.cfg.ConfigPath
	}

	status := s.sup.Status()
	if status.State == supervisor.StateRunning {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "RESPONSE",
			"payload": map[string]interface{}{
				"pid":    status.PID,
				"status": "already_running",
			},
		}
	}

	err := s.sup.Start(context.Background(), configPath)
	if err != nil {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "ERROR",
			"payload": map[string]interface{}{
				"code":    "CORE_002",
				"message": err.Error(),
			},
		}
	}
	if err := s.commitHealthyRuntime(context.Background()); err != nil {
		_ = s.sup.Stop(context.Background())
		return errorResponse(requestID, "CORE_COMMIT_FAILED", err)
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

func (s *Service) handleCoreStop(requestID string) map[string]interface{} {
	status := s.sup.Status()
	if status.State != supervisor.StateRunning {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "RESPONSE",
			"payload": map[string]interface{}{
				"status": "already_stopped",
			},
		}
	}

	err := s.sup.Stop(context.Background())
	if err != nil {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "ERROR",
			"payload": map[string]interface{}{
				"code":    "CORE_003",
				"message": err.Error(),
			},
		}
	}

	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"status": "stopped",
		},
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
	if wasRunning {
		if err := previousSupervisor.Stop(ctx); err != nil {
			return errorResponse(requestID, "CORE_SWITCH_FAILED", err)
		}
	}
	nextSupervisor := supervisor.NewSupervisor(nextHost, s.reconciler)
	s.runtime.CoreID = coreID
	s.host, s.sup = nextHost, nextSupervisor
	if err := s.applyRuntimeConfigLocked(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		s.runtime = previousRuntime
		s.host, s.sup = previousHost, previousSupervisor
		s.cfg.ConfigPath = previousConfigPath
		if wasRunning {
			_ = previousSupervisor.Start(context.Background(), previousConfigPath)
		}
		return errorResponse(requestID, "CORE_SWITCH_FAILED", err)
	}
	if wasRunning {
		if err := nextSupervisor.Start(ctx, s.cfg.ConfigPath); err != nil {
			s.runtime = previousRuntime
			s.host, s.sup = previousHost, previousSupervisor
			s.cfg.ConfigPath = previousConfigPath
			_ = s.saveRuntimeStateLocked(s.cfg.ConfigDir)
			_ = previousSupervisor.Start(context.Background(), previousConfigPath)
			return errorResponse(requestID, "CORE_SWITCH_FAILED", err)
		}
		if err := s.commitHealthyRuntimeLocked(ctx); err != nil {
			_ = nextSupervisor.Stop(context.Background())
			s.runtime = previousRuntime
			s.host, s.sup = previousHost, previousSupervisor
			s.cfg.ConfigPath = previousConfigPath
			_ = s.saveRuntimeStateLocked(s.cfg.ConfigDir)
			_ = previousSupervisor.Start(context.Background(), previousConfigPath)
			return errorResponse(requestID, "CORE_SWITCH_COMMIT_FAILED", err)
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

func (s *Service) handleCoreRestart(requestID string, msg map[string]interface{}) map[string]interface{} {
	configPath, _ := msg["config_path"].(string)
	if configPath == "" {
		configPath = s.cfg.ConfigPath
	}

	err := s.sup.Restart(context.Background(), configPath)
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

func (s *Service) handleTUNEnable(requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	if name == "" {
		name = "Navo"
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
	s.runtimeMu.Lock()
	s.runtime.TUNName, s.runtime.TUNMTU = name, int(mtu)
	s.runtimeMu.Unlock()
	return s.handleCapturePrepare(
		context.Background(), requestID,
		map[string]interface{}{"mode": capture.ModeTUN.String()},
	)
}

func (s *Service) handleTUNDisable(requestID string) map[string]interface{} {
	return s.handleCapturePrepare(
		context.Background(), requestID,
		map[string]interface{}{"mode": capture.ModeSystemProxy.String()},
	)
}

func (s *Service) handleTUNStatus(requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	enabled := s.runtime.TUNEnabled
	name := s.runtime.TUNName
	mtu := s.runtime.TUNMTU
	s.runtimeMu.Unlock()
	adapter := tun.InspectAdapter(context.Background(), name)
	faultID, fault := s.currentTUNFault()
	healthy := enabled && adapter.State == capture.AdapterEnabled
	routeCount := 0
	if healthy {
		routeCount = 1
	}
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"installed":       s.wintunAvailable(),
			"created":         adapter.State != capture.AdapterMissing,
			"enabled":         healthy,
			"name":            adapter.Name,
			"state":           adapter.State,
			"identifier":      adapter.InterfaceGUID,
			"interface_index": adapter.InterfaceIndex,
			"addresses":       []string{"172.19.0.1/30"},
			"mtu":             mtu,
			"route_count":     routeCount,
			"fault_id":        faultID,
			"last_error":      fault,
		},
	}
}

func (s *Service) handleTUNConfig(requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	s.runtimeMu.Lock()
	currentName := s.runtime.TUNName
	currentMTU := s.runtime.TUNMTU
	enabled := s.runtime.TUNEnabled
	s.runtimeMu.Unlock()
	if name == "" {
		name = currentName
	}
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
	if err := s.setTUNRuntime(
		context.Background(),
		enabled,
		name,
		currentMTU,
	); err != nil {
		return errorResponse(requestID, "NET_007", err)
	}
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload":    map[string]interface{}{"status": "ok"},
	}
}

// ── Subscription handlers ──

func (s *Service) handleSubAdd(requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	url, _ := msg["url"].(string)
	skipTLSVerify, _ := msg["skip_tls_verify"].(bool)
	if name == "" || url == "" {
		return map[string]interface{}{
			"request_id": requestID, "type": "ERROR",
			"payload": map[string]interface{}{"code": "INVALID", "message": "name and url required"},
		}
	}
	sub, err := s.subMgr.AddWithOptions(name, url, skipTLSVerify)
	if err != nil {
		return errorResponse(requestID, "SUB_001", err)
	}
	if boolField(msg, "wait") {
		outbounds, refreshErr := s.subMgr.Refresh(context.Background())
		if refreshErr != nil {
			return errorResponse(requestID, "SUB_002", refreshErr)
		}
		if len(outbounds) > 0 {
			if applyErr := s.applyRuntimeConfig(
				context.Background(),
				s.currentOutbounds(context.Background()),
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
	// Auto-refresh in background to fetch nodes immediately.
	go func() {
		ctx := context.Background()
		outbounds, err := s.subMgr.Refresh(ctx)
		if err != nil {
			log.Printf("[service] initial refresh for %q failed: %v", name, err)
			return
		}
		if len(outbounds) > 0 {
			s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", "")
		}
	}()
	return response(requestID, map[string]interface{}{"id": sub.ID, "status": "added"})
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

func (s *Service) handleSubRemove(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	ok, err := s.subMgr.Remove(id)
	if err != nil {
		return errorResponse(requestID, "SUB_006", err)
	}
	if !ok {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("subscription not found"))
	}
	// Apply config change in background.
	go s.applyRuntimeConfig(context.Background(), s.currentOutbounds(context.Background()), "", "")
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
	if boolField(msg, "wait") {
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
	// Respond immediately; fetch and apply in background.
	go func() {
		ctx := context.Background()
		outbounds, err := s.subMgr.Refresh(ctx)
		if err != nil {
			log.Printf("[service] subscription refresh failed: %v", err)
			return
		}
		if len(outbounds) > 0 {
			if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
				log.Printf("[service] apply after refresh failed: %v", err)
			}
		}
	}()
	return response(requestID, map[string]interface{}{"status": "refreshing"})
}

func (s *Service) handleOutboundList(requestID string) map[string]interface{} {
	outbounds := s.currentOutbounds(context.Background())
	s.runtimeMu.Lock()
	active := s.runtime.SelectedOutbound
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
			"source_type": sourceType,
			"available":   status.Available, "color": status.Color,
			"reason": status.Reason, "checked_at": status.CheckedAt,
			"latency_ms": status.LatencyMS,
		}
		list = append(list, item)
	}
	return response(requestID, map[string]interface{}{
		"outbounds": list,
		"active_id": active,
		"mode":      mode,
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
	payload := map[string]interface{}{
		"mode":               mode,
		"active_outbound":    activeID,
		"reachable":          s.sup.State() == supervisor.StateRunning,
		"available":          available,
		"unavailable_reason": reason,
		"core_name":          coreID,
		"latency_ms":         nil,
		"upload_bytes":       totalUp,
		"download_bytes":     totalDown,
		"connections":        connections,
	}
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": payload,
	}
}

func (s *Service) handleIPCheck(requestID string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Fetch source IP (direct, no proxy) and proxy IP in parallel
	var sourceResult, proxyResult *ipdetect.IPResult
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		res, err := s.directIPDetector.Check(ctx, "source")
		if err != nil {
			sourceResult = &ipdetect.IPResult{
				OutboundID: "source",
				IP:         "检测暂不可用",
				Error:      err.Error(),
			}
		} else {
			sourceResult = res
		}
	}()

	go func() {
		defer wg.Done()
		res, err := s.ipDetector.Check(ctx, "current")
		if err != nil {
			proxyResult = &ipdetect.IPResult{
				OutboundID: "current",
				IP:         "检测暂不可用",
				Error:      err.Error(),
			}
		} else {
			proxyResult = res
		}
	}()

	wg.Wait()
	s.diagnosticMu.Lock()
	s.exitIP = proxyResult.IP
	s.exitCountry = proxyResult.Country
	s.diagnosticMu.Unlock()

	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{
			"source": map[string]interface{}{
				"ip":         sourceResult.IP,
				"country":    sourceResult.Country,
				"city":       sourceResult.City,
				"asn":        sourceResult.ASN,
				"isp":        sourceResult.ISP,
				"network":    sourceResult.Network,
				"provider":   sourceResult.Provider,
				"mobile":     sourceResult.Mobile,
				"proxy":      sourceResult.Proxy,
				"hosting":    sourceResult.Hosting,
				"checked_at": sourceResult.CheckedAt,
				"error":      sourceResult.Error,
			},
			"proxy": map[string]interface{}{
				"ip":         proxyResult.IP,
				"country":    proxyResult.Country,
				"city":       proxyResult.City,
				"asn":        proxyResult.ASN,
				"isp":        proxyResult.ISP,
				"network":    proxyResult.Network,
				"provider":   proxyResult.Provider,
				"mobile":     proxyResult.Mobile,
				"proxy":      proxyResult.Proxy,
				"hosting":    proxyResult.Hosting,
				"checked_at": proxyResult.CheckedAt,
				"error":      proxyResult.Error,
			},
		},
	}
}

func (s *Service) handleAIRuleGenerate(requestID string, msg map[string]interface{}) map[string]interface{} {
	userReq, _ := msg["request"].(string)
	if userReq == "" {
		return map[string]interface{}{
			"request_id": requestID, "type": "ERROR",
			"payload": map[string]interface{}{"code": "INVALID", "message": "request text required"},
		}
	}

	s.aiMu.RLock()
	assistant := s.aiAssistant
	s.aiMu.RUnlock()
	gen := ai.NewRuleGen(assistant)
	rules, err := gen.Generate(context.Background(), userReq, nil, nil)
	if err != nil {
		return map[string]interface{}{
			"request_id": requestID, "type": "ERROR",
			"payload": map[string]interface{}{"code": "AI_001", "message": err.Error()},
		}
	}

	list := make([]interface{}, len(rules))
	for i, r := range rules {
		list[i] = r
	}
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{"rules": list},
	}
}

func (s *Service) handleAIDiagnose(requestID string) map[string]interface{} {
	stats := s.collector.Stats()
	snapshot := &ai.NetworkSnapshot{
		Outbounds: make([]ai.OutboundState, 0),
		Metrics:   make([]ai.MetricState, 0),
		Errors:    []string{},
	}
	for _, st := range stats {
		snapshot.Metrics = append(snapshot.Metrics, ai.MetricState{
			OutboundID: st.OutboundID,
			Upload:     st.Upload,
			Download:   st.Download,
		})
	}

	result := ai.QuickAnalyze(snapshot)
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{
			"severity": result.Severity, "summary": result.Summary,
			"issues": result.Issues, "suggestions": result.Suggestions,
		},
	}
}

func (s *Service) handleAIExplain(requestID string) map[string]interface{} {
	stats := s.collector.Stats()
	obState := make([]ai.OutboundState, 0)
	for _, st := range stats {
		obState = append(obState, ai.OutboundState{
			ID: st.OutboundID, Healthy: true,
		})
	}
	snapshot := &ai.NetworkSnapshot{Outbounds: obState}

	s.aiMu.RLock()
	assistant := s.aiAssistant
	s.aiMu.RUnlock()
	ex := ai.NewExplain(assistant)
	text, err := ex.ExplainNetwork(context.Background(), snapshot)
	if err != nil {
		text = "当前无法生成 AI 解释。请检查 AI 服务配置。"
	}
	return map[string]interface{}{
		"request_id": requestID, "type": "RESPONSE",
		"payload": map[string]interface{}{"text": text},
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
