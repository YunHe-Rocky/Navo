// Package agent provides the User Agent that runs in the user's session.
// It manages the system tray, system proxy, and communicates with the
// Windows Service via Named Pipe.
//
// Architecture: Wails/Vue UI → Agent Connection Coordinator → Service domain executors
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/connection"
	"navo/internal/domain/capture"
	"navo/internal/logstore"
	"navo/internal/pipe"
	"navo/internal/selfheal"
)

const (
	defaultIPCRequestTimeout    = 30 * time.Second
	captureIPCRequestTimeout    = 2 * time.Minute
	coreSwitchIPCRequestTimeout = 4 * time.Minute
)

// Config holds the User Agent configuration.
type Config struct {
	ServicePipeName string // pipe to connect to the service
	UIPipeName      string // pipe for UI to connect to agent
	ProxyPort       int    // local proxy port (sing-box mixed inbound)

	// SendToServiceFn is an optional direct function that replaces the Named Pipe
	// relay to the service. When set, the agent calls this function directly
	// instead of opening a pipe connection.
	// SendToServiceContextFn is preferred because capture recovery and UI
	// cancellation must stop the Service mutation itself, not only its waiter.
	SendToServiceContextFn   func(context.Context, map[string]interface{}) (map[string]interface{}, error)
	SendToServiceFn          func(msg map[string]interface{}) (map[string]interface{}, error)
	IsElevatedFn             func() bool
	ProxyProbeFn             func(context.Context, string) error
	ShowUIFn                 func() error
	MinimizeToTrayFn         func() error
	RequestExitFn            func()
	CaptureJournalPath       string
	CaptureProbeFn           func(context.Context, capture.Mode) error
	CaptureRouteProbeFn      func(context.Context, capture.Mode, string) error
	CoreUpdateSessionTimeout time.Duration // optional bounded mutation window; defaults to four minutes
	ProxyManager             *systemproxy.Manager
}

// Agent is the user-session agent.
type Agent struct {
	cfg        Config
	proxy      *systemproxy.Manager
	proxyProbe func(context.Context, string) error

	serviceCh         *pipe.Channel // connection to service
	serviceMu         sync.Mutex    // serializes request/response pairs on serviceCh
	serviceSession    string        // isolates Service replay IDs from UI request IDs
	serviceSeq        atomic.Uint64
	captureStateMu    sync.RWMutex
	captureState      capture.Snapshot
	captureJournal    *capture.JournalStore
	captureProbe      func(context.Context, capture.Mode) error
	captureRouteProbe func(context.Context, capture.Mode, string) error
	uiListener        pipe.Listener // listener for UI connections
	coordinator       *connection.Coordinator
	recoveryMu        sync.RWMutex
	recoveryReport    selfheal.RecoveryReport

	coreUpdateMu      sync.Mutex
	coreUpdateSession *coreUpdateSession

	ipProbeMu      sync.Mutex
	ipProbeRunning bool
	lastIPProbe    time.Time

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	stopOnce sync.Once
}

// New creates a new User Agent.
func New(cfg Config) (*Agent, error) {
	if cfg.ServicePipeName == "" {
		cfg.ServicePipeName = "Navo.Agent.Service.v1"
	}
	if cfg.UIPipeName == "" {
		cfg.UIPipeName = "Navo.UI.Agent.v1"
	}
	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 12080
	}
	if cfg.IsElevatedFn == nil {
		cfg.IsElevatedFn = processIsElevated
	}
	serviceSessionBytes := make([]byte, 16)
	if cfg.CoreUpdateSessionTimeout <= 0 {
		cfg.CoreUpdateSessionTimeout = coreSwitchIPCRequestTimeout
	}
	if _, err := rand.Read(serviceSessionBytes); err != nil {
		return nil, fmt.Errorf("create Agent Service request session: %w", err)
	}

	proxyProbe := cfg.ProxyProbeFn
	if proxyProbe == nil {
		proxyProbe = probeHTTPProxy
	}
	proxyManager := cfg.ProxyManager
	if proxyManager == nil {
		proxyManager = systemproxy.NewManager()
	}
	agent := &Agent{
		cfg:            cfg,
		proxy:          proxyManager,
		proxyProbe:     proxyProbe,
		serviceSession: hex.EncodeToString(serviceSessionBytes),
		coordinator:    connection.NewCoordinator(),
		stopCh:         make(chan struct{}),
	}
	agent.initializeCaptureState()
	return agent, nil
}

// Run starts the agent. Connects to the service and listens for UI connections.
func (a *Agent) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent already running")
	}
	a.running = true
	a.mu.Unlock()

	log.Printf("[agent] starting Navo User Agent")
	log.Printf("[agent] service pipe: %s", a.cfg.ServicePipeName)
	log.Printf("[agent] UI pipe: %s", a.cfg.UIPipeName)

	// Connect to the Windows Service before accepting any UI connections.
	if a.cfg.SendToServiceContextFn == nil && a.cfg.SendToServiceFn == nil {
		a.waitForServicePipe(ctx, 10*time.Second)
	}

	// Expose the UI pipe before recovery. Recovery can legitimately take time
	// for stale Windows network state; the desktop must render that phase
	// instead of opening with an unavailable bridge.
	listener, err := pipe.NewListener(a.cfg.UIPipeName)
	if err != nil {
		return fmt.Errorf("UI pipe listener: %w", err)
	}
	a.uiListener = listener
	log.Printf("[agent] UI pipe listening on %s", listener.Addr())

	// Pre-create additional pipe instances so concurrent Flutter requests
	// (from multiple MethodChannel threads) don't get ERROR_PIPE_BUSY.
	listener.PreCreateInstances(32)

	// Accept UI connections
	go a.acceptUIConnections(ctx)

	if err := a.recoverCaptureOnStartup(ctx); err != nil {
		log.Printf("[agent] capture startup recovery: %v", err)
	}
	go a.monitorCaptureHealth(ctx)

	// Wait for stop
	select {
	case <-ctx.Done():
	case <-a.stopCh:
	}

	return a.shutdown()
}

// Stop stops the agent.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		a.stopOnce.Do(func() {
			close(a.stopCh)
		})
	}
}

// ── Service Connection ──

func (a *Agent) connectToService(ctx context.Context) error {
	ch, err := pipe.Dial(a.cfg.ServicePipeName)
	if err != nil {
		return fmt.Errorf("dial service: %w", err)
	}
	a.serviceCh = ch
	return nil
}

// waitForServicePipe blocks until the service pipe is reachable or the
// timeout expires. Returns immediately if the agent is stopped.
func (a *Agent) waitForServicePipe(ctx context.Context, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	backoff := []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}
		if err := a.connectToService(ctx); err == nil {
			log.Printf("[agent] service pipe connected on attempt %d", attempt+1)
			return
		}
		delay := backoff[min(attempt, len(backoff)-1)]
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-time.After(delay):
		}
	}
	log.Printf("[agent] WARNING: service pipe not available after %v", timeout)
}

// connectToServiceWithRetry keeps trying to connect to the service until
// the agent is stopped. It backs off between attempts.
func (a *Agent) connectToServiceWithRetry(ctx context.Context) {
	backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}
		if a.serviceCh != nil {
			return // already connected
		}
		if err := a.connectToService(ctx); err != nil {
			delay := backoff[min(attempt, len(backoff)-1)]
			log.Printf("[agent] service connection attempt %d failed: %v (retry in %v)", attempt+1, err, delay)
			select {
			case <-ctx.Done():
				return
			case <-a.stopCh:
				return
			case <-time.After(delay):
			}
			continue
		}
		log.Printf("[agent] connected to service on attempt %d", attempt+1)
		return
	}
}

// SendToService sends a message to the Windows Service with automatic retry.
func (a *Agent) SendToService(msg map[string]interface{}) (map[string]interface{}, error) {
	return a.SendToServiceContext(context.Background(), msg)
}

// SendToServiceContext sends one at-most-once identified request. Named Pipe
// reconnects reuse the request ID so Service replay protection can return the
// original response without repeating a mutation.
func (a *Agent) SendToServiceContext(
	ctx context.Context,
	msg map[string]interface{},
) (map[string]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	serviceRequest, parentRequestID := a.prepareServiceRequest(msg)
	if a.cfg.SendToServiceContextFn != nil {
		response, err := a.cfg.SendToServiceContextFn(ctx, serviceRequest)
		return restoreParentRequestID(response, parentRequestID), err
	}
	if a.cfg.SendToServiceFn != nil {
		response, err := a.cfg.SendToServiceFn(serviceRequest)
		return restoreParentRequestID(response, parentRequestID), err
	}

	if err := a.lockService(ctx); err != nil {
		return nil, err
	}
	defer a.serviceMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if a.serviceCh == nil {
			if err := a.connectToService(ctx); err != nil {
				lastErr = err
				if !waitForContext(ctx, time.Duration(attempt+1)*500*time.Millisecond) {
					return nil, ctx.Err()
				}
				continue
			}
		}
		deadline := time.Now().Add(agentIPCRequestTimeout(serviceRequest))
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		if err := a.serviceCh.SetDeadline(deadline); err != nil {
			lastErr = err
			a.dropServiceChannelLocked()
			continue
		}
		if err := a.serviceCh.Send(serviceRequest); err != nil {
			lastErr = err
			a.dropServiceChannelLocked()
			if !waitForContext(ctx, 200*time.Millisecond) {
				return nil, ctx.Err()
			}
			continue
		}

		var response map[string]interface{}
		if err := a.serviceCh.Receive(&response); err != nil {
			lastErr = err
			a.dropServiceChannelLocked()
			if !waitForContext(ctx, 200*time.Millisecond) {
				return nil, ctx.Err()
			}
			continue
		}
		return restoreParentRequestID(response, parentRequestID), nil
	}
	return nil, fmt.Errorf("service request failed after reconnect attempts: %w", lastErr)
}

// prepareServiceRequest establishes a separate correlation namespace for the
// Agent-to-Service hop. One logical call gets one ID, which is then retained by
// all Named Pipe reconnect attempts inside SendToServiceContext.
func (a *Agent) prepareServiceRequest(msg map[string]interface{}) (map[string]interface{}, string) {
	request := make(map[string]interface{}, len(msg)+1)
	for key, value := range msg {
		request[key] = value
	}
	parentRequestID, _ := request["request_id"].(string)
	request["request_id"] = fmt.Sprintf(
		"agent-%s-%d",
		a.serviceSession,
		a.serviceSeq.Add(1),
	)
	return request, parentRequestID
}

func restoreParentRequestID(response map[string]interface{}, parentRequestID string) map[string]interface{} {
	if response == nil {
		return nil
	}
	restored := make(map[string]interface{}, len(response)+1)
	for key, value := range response {
		restored[key] = value
	}
	restored["request_id"] = parentRequestID
	return restored
}

func (a *Agent) lockService(ctx context.Context) error {
	ticker := time.NewTicker(captureLockPollInterval)
	defer ticker.Stop()
	for {
		if a.serviceMu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (a *Agent) dropServiceChannelLocked() {
	if a.serviceCh == nil {
		return
	}
	_ = a.serviceCh.Close()
	a.serviceCh = nil
}

func waitForContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// ── System Proxy ──

// EnableProxy enables the system proxy.
func (a *Agent) EnableProxy() error {
	addr := fmt.Sprintf("127.0.0.1:%d", a.cfg.ProxyPort)
	probeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.proxyProbe(probeCtx, addr); err != nil {
		return fmt.Errorf("local HTTP proxy is not ready: %w", err)
	}
	log.Printf("[agent] enabling system proxy: %s", addr)
	return a.proxy.Enable(addr)
}

func probeHTTPProxy(ctx context.Context, address string) error {
	proxyURL, err := url.Parse("http://" + address)
	if err != nil {
		return fmt.Errorf("parse proxy endpoint: %w", err)
	}
	probeErrors := make([]error, 0, 3)
	endpoints := []string{
		"http://connectivitycheck.gstatic.com/generate_204",
		"http://cp.cloudflare.com/generate_204",
		"http://www.msftconnecttest.com/connecttest.txt",
	}
	for attempt, endpoint := range endpoints {
		if err := probeHTTPProxyOnce(ctx, proxyURL, address, endpoint); err == nil {
			return nil
		} else {
			probeErrors = append(probeErrors, err)
		}
		if attempt == len(endpoints)-1 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("end-to-end proxy readiness after attempt %d: %w", attempt+1, ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("all end-to-end proxy probes failed: %w", errors.Join(probeErrors...))
}

type proxyReadinessError struct {
	endpoint   string
	statusCode int
	cause      error
}

func (e *proxyReadinessError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s through local proxy: %v", e.endpoint, e.cause)
	}
	return fmt.Sprintf("%s through local proxy returned HTTP %d", e.endpoint, e.statusCode)
}

func (e *proxyReadinessError) Unwrap() error { return e.cause }

func probeHTTPProxyOnce(
	ctx context.Context,
	proxyURL *url.URL,
	address string,
	endpoint string,
) error {
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 6 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("build probe request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return &proxyReadinessError{endpoint: endpoint, cause: fmt.Errorf("request through %s: %w", address, err)}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return &proxyReadinessError{endpoint: endpoint, statusCode: response.StatusCode}
	}
	return nil
}

// DisableProxy disables the system proxy.
func (a *Agent) DisableProxy() error {
	log.Printf("[agent] disabling system proxy")
	return a.proxy.Disable()
}

// ToggleProxy toggles the system proxy on/off.
func (a *Agent) ToggleProxy() error {
	if a.proxy.IsActive() {
		return a.DisableProxy()
	}
	return a.EnableProxy()
}

// ProxyStatus returns the current proxy status.
func (a *Agent) ProxyStatus() systemproxy.ProxyConfig {
	return a.proxy.Status()
}

// ── UI Connection Handling ──

func (a *Agent) acceptUIConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}

		ch, err := a.uiListener.Accept()
		if err != nil {
			return
		}

		go a.handleUIConnection(ctx, ch)
	}
}

func (a *Agent) handleUIConnection(ctx context.Context, ch *pipe.Channel) {
	defer ch.Close()

	log.Printf("[agent] UI connection accepted")

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		default:
		}

		if err := ch.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			log.Printf("[agent] set UI read deadline: %v", err)
			return
		}

		var msg map[string]interface{}
		if err := ch.Receive(&msg); err != nil {
			return
		}

		log.Printf("[agent] UI request: method=%v", msg["method"])

		requestCtx, cancel := context.WithTimeout(ctx, agentIPCRequestTimeout(msg))
		response := a.dispatchUI(requestCtx, msg)
		cancel()
		if response != nil {
			if err := ch.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Printf("[agent] set UI write deadline: %v", err)
				return
			}
			if err := ch.Send(response); err != nil {
				log.Printf("[agent] UI send error: %v", err)
				return
			}
		}
	}
}

func agentIPCRequestTimeout(msg map[string]interface{}) time.Duration {
	method, _ := msg["method"].(string)
	switch method {
	case "core.select", "core.update.begin", "core.update.commit", "core.update.rollback",
		"core.update.stop", "core.update.start":
		return coreSwitchIPCRequestTimeout
	case "capture.set", "capture.verify", "capture.prepare", "tun.enable", "proxy.enable", "proxy.disable", "proxy.toggle", "network.recover",
		"runtime.mode.set", "runtime.rules.set", "runtime.list_mode.set",
		"outbound.select", "outbound.create", "outbound.delete",
		"subscription.add", "subscription.remove", "subscription.refresh":
		return captureIPCRequestTimeout
	default:
		return defaultIPCRequestTimeout
	}
}

func (a *Agent) dispatchUI(ctx context.Context, msg map[string]interface{}) map[string]interface{} {
	method, _ := msg["method"].(string)
	requestID, _ := msg["request_id"].(string)
	_ = logstore.Emit(logstore.LevelDebug, "Agent", "UIIPC", "request received", map[string]any{
		"method": method, "request_id": requestID,
	})

	switch method {
	case "dashboard.snapshot":
		return a.handleDashboardSnapshot(requestID)
	case "ui.show":
		if a.cfg.ShowUIFn == nil {
			return agentError(requestID, "UI_UNAVAILABLE", fmt.Errorf("desktop UI launcher is unavailable"))
		}
		if err := a.cfg.ShowUIFn(); err != nil {
			return agentError(requestID, "UI_START_FAILED", err)
		}
		return agentResponse(requestID, map[string]interface{}{"shown": true})
	case "ui.hide":
		if a.cfg.MinimizeToTrayFn == nil {
			return agentError(requestID, "TRAY_UNAVAILABLE", fmt.Errorf("system tray is unavailable; window remains visible"))
		}
		if err := a.cfg.MinimizeToTrayFn(); err != nil {
			return agentError(requestID, "TRAY_REFRESH_FAILED", err)
		}
		return agentResponse(requestID, map[string]interface{}{"tray_visible": true})
	case "ui.exit":
		if a.cfg.RequestExitFn == nil {
			return agentError(requestID, "UI_EXIT_UNAVAILABLE", fmt.Errorf("application shutdown coordinator is unavailable"))
		}
		// Return the IPC acknowledgement before the launcher begins tearing down
		// the UI pipe and WebView process.
		go func(requestExit func()) {
			time.Sleep(50 * time.Millisecond)
			requestExit()
		}(a.cfg.RequestExitFn)
		return agentResponse(requestID, map[string]interface{}{"accepted": true})
	case "tray.snapshot":
		return a.handleTraySnapshot(requestID)
	case "connection.enable":
		return a.handleConnectionEnable(ctx, requestID)
	case "connection.disable":
		return a.handleConnectionDisable(ctx, requestID)
	case "connection.restart":
		return a.handleConnectionRestart(ctx, requestID)
	case "network.recover":
		return a.handleNetworkRecover(ctx, requestID)
	case "capture.set":
		return a.setCaptureModeContext(ctx, requestID, msg)
	case "core.select":
		return a.selectCoreWithCapture(ctx, requestID, msg)
	case "core.update.begin":
		return a.handleCoreUpdateBegin(ctx, requestID, msg)
	case "core.update.commit":
		return a.handleCoreUpdateCommit(ctx, requestID, msg)
	case "core.update.rollback":
		return a.handleCoreUpdateRollback(requestID, msg)
	case "core.update.stop", "core.update.start":
		return agentError(requestID, "CORE_UPDATE_SESSION_REQUIRED", fmt.Errorf("raw core update steps are internal; begin a bounded core update session"))
	case "outbound.select":
		return a.selectOutboundWithCapture(ctx, requestID, msg)
	case "outbound.create", "outbound.delete",
		"subscription.add", "subscription.remove", "subscription.refresh":
		return a.mutateSourcesWithCapture(ctx, requestID, msg)
	case "subscription.update", "outbound.update":
		return a.forwardConnectionMutation(
			ctx, requestID, msg,
			connection.OperationSourceMutation,
		)
	case "tun.config", "runtime.mode.set", "runtime.rules.set", "runtime.list_mode.set":
		return a.forwardConnectionMutation(
			ctx, requestID, msg,
			connection.OperationPolicyChange,
		)
	case "capture.status":
		return agentResponse(requestID, a.captureStatusPayload())
	case "capture.verify":
		readiness, err := a.verifyCaptureReadiness(ctx)
		if err != nil {
			return agentError(requestID, "APPLICATION_READINESS_FAILED", err)
		}
		return agentResponse(requestID, map[string]interface{}{
			"readiness": readiness,
		})
	case "tun.enable":
		return a.setCaptureModeContext(ctx, requestID, map[string]interface{}{"mode": "tun"})
	case "proxy.enable":
		return a.setCaptureModeContext(ctx, requestID, map[string]interface{}{"mode": "system_proxy"})
	case "proxy.disable":
		return a.setCaptureModeContext(ctx, requestID, map[string]interface{}{"mode": "off"})
	case "proxy.toggle":
		mode := "system_proxy"
		if a.ProxyStatus().Enabled {
			mode = "off"
		}
		return a.setCaptureModeContext(ctx, requestID, map[string]interface{}{"mode": mode})
	case "proxy.status":
		status := a.ProxyStatus()
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "RESPONSE",
			"payload":    status,
		}
	case "core.status", "core.list",
		"tun.status",
		"subscription.list",
		"outbound.list", "outbound.test", "outbound.testAll", "outbound.failover_candidates",
		"runtime.status",
		"metrics.current", "ip.check", "ip.check_all",
		"diagnostics.export", "log.tail", "core.log.tail",
		"logs.query", "logs.services", "logs.levels", "logs.clear.persisted":
		// Forward to service
		resp, err := a.SendToServiceContext(ctx, msg)
		if err != nil {
			return map[string]interface{}{
				"request_id": requestID,
				"type":       "ERROR",
				"payload": map[string]interface{}{
					"code":    "AGENT_001",
					"message": fmt.Sprintf("service unreachable: %v", err),
				},
			}
		}
		return resp
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

// Dispatch is the application boundary used by the native Tray and Wails UI.
// Callers submit typed method names and IDs; Agent remains the only component
// allowed to coordinate user-session capture state with Service operations.
func (a *Agent) Dispatch(
	ctx context.Context,
	msg map[string]interface{},
) map[string]interface{} {
	return a.dispatchUI(ctx, msg)
}

func (a *Agent) setCaptureModeContext(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	mode, _ := msg["mode"].(string)
	target := capture.Mode(mode)
	if !target.Valid() {
		return agentError(requestID, "INVALID", fmt.Errorf("unsupported capture mode %q", mode))
	}
	if err := a.transitionCaptureMode(ctx, target); err != nil {
		if strings.HasPrefix(err.Error(), "TUN_REQUIRES_ADMIN:") {
			return agentError(requestID, "TUN_REQUIRES_ADMIN", err)
		}
		if errors.Is(err, errCaptureBusy) {
			return agentError(requestID, "CAPTURE_BUSY", err)
		}
		var serviceErr *serviceCaptureError
		if errors.As(err, &serviceErr) && serviceErr.code != "" {
			return agentError(requestID, serviceErr.code, err)
		}
		var readinessErr *proxyReadinessError
		if errors.As(err, &readinessErr) {
			return agentError(requestID, "PROXY_DATAPLANE_UNAVAILABLE", err)
		}
		return agentError(requestID, "CAPTURE_TRANSITION_FAILED", err)
	}
	return agentResponse(requestID, a.captureStatusPayload())
}

func (a *Agent) enableTUN(requestID string) map[string]interface{} {
	if !a.cfg.IsElevatedFn() {
		return agentError(
			requestID,
			"TUN_REQUIRES_ADMIN",
			fmt.Errorf("TUN requires Navo to run as administrator"),
		)
	}
	return a.callService(requestID, "tun.enable")
}

func (a *Agent) tunEnabled() (bool, error) {
	resp, err := a.SendToService(map[string]interface{}{
		"request_id": fmt.Sprintf("capture-status-%d", time.Now().UnixNano()),
		"method":     "tun.status",
	})
	if err != nil {
		return false, fmt.Errorf("query TUN state: %w", err)
	}
	if isErrorResponse(resp) {
		return false, fmt.Errorf("query TUN state: %s", responseMessage(resp))
	}
	payload, _ := resp["payload"].(map[string]interface{})
	enabled, _ := payload["enabled"].(bool)
	return enabled, nil
}

func (a *Agent) callService(requestID, method string) map[string]interface{} {
	return a.callServiceContext(context.Background(), requestID, method)
}

func (a *Agent) callServiceContext(ctx context.Context, requestID, method string) map[string]interface{} {
	resp, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID,
		"method":     method,
	})
	if err != nil {
		return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	return resp
}

func isErrorResponse(resp map[string]interface{}) bool {
	typeName, _ := resp["type"].(string)
	return typeName == "ERROR"
}

func responseMessage(resp map[string]interface{}) string {
	payload, _ := resp["payload"].(map[string]interface{})
	message, _ := payload["message"].(string)
	if message == "" {
		return "unknown service error"
	}
	return message
}

func responseCode(resp map[string]interface{}) string {
	payload, _ := resp["payload"].(map[string]interface{})
	code, _ := payload["code"].(string)
	return code
}

func agentError(requestID, code string, err error) map[string]interface{} {
	_ = logstore.Emit(logstore.LevelError, "Agent", "UIIPC", "request failed: "+code, map[string]any{
		"request_id": requestID, "error_code": code, "reason": err.Error(),
	})
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "ERROR",
		"payload": map[string]interface{}{
			"code":    code,
			"message": err.Error(),
		},
	}
}

func (a *Agent) proxyResponse(requestID string, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{
			"request_id": requestID,
			"type":       "ERROR",
			"payload": map[string]interface{}{
				"code":    "PROXY_001",
				"message": err.Error(),
			},
		}
	}

	status := a.ProxyStatus()
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload": map[string]interface{}{
			"enabled": status.Enabled,
			"proxy":   status.ProxyServer,
		},
	}
}

func (a *Agent) shutdown() error {
	log.Printf("[agent] shutting down...")

	a.abortCoreUpdateForShutdown()
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(), captureRecoveryTimeout,
	)
	defer shutdownCancel()
	transaction, transactionErr := a.beginConnection(
		shutdownCtx, "", connection.OperationRecovery, connection.OriginShutdown, "capture",
	)
	if transactionErr != nil {
		log.Printf("[agent] wait for connection transaction during shutdown: %v", transactionErr)
	} else {
		defer transaction.Close()
	}

	// Disable is ownership-aware and also recovers a record inherited after a
	// crash; call it even when this process did not set the in-memory flag.
	if err := a.proxy.Disable(); err != nil {
		log.Printf("[agent] restore system proxy during shutdown: %v", err)
	}

	// Close connections
	a.serviceMu.Lock()
	a.dropServiceChannelLocked()
	a.serviceMu.Unlock()
	if a.uiListener != nil {
		a.uiListener.Close()
	}

	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	log.Printf("[agent] shutdown complete")
	return nil
}

// ── Standalone Runner ──

// RunStandalone runs the agent in standalone mode until Ctrl+C.
func RunStandalone(agent *Agent) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		log.Println("[agent] received interrupt")
		cancel()
	}()

	return agent.Run(ctx)
}
