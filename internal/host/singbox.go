package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"navo/internal/winprocess"
)

// SingBoxHost implements CoreHost for the sing-box proxy core.
type SingBoxHost struct {
	coreID      string
	binaryPath  string
	workDir     string
	runArgs     func(string) []string
	checkArgs   func(string) []string
	extractPort func(string) (int, error)
	configPath  string

	mu           sync.RWMutex
	status       HostStatus
	cmd          *exec.Cmd
	cancel       context.CancelFunc
	exitCh       chan error
	startedAt    time.Time
	restartCount int
	lastError    string

	// Crash recovery
	restartBackoff []time.Duration
	maxRestarts    int
	crashCh        chan struct{}

	// Log ring buffer
	logBuf *ringBuffer

	// Listen port extracted from config for health checks
	listenPort int
}

// ringBuffer is a fixed-size circular buffer for log lines.
type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	size  int
	pos   int
	full  bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

func (rb *ringBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Split input into lines and buffer each
	for _, line := range strings.Split(string(p), "\n") {
		if line == "" {
			continue
		}
		rb.lines[rb.pos] = line
		rb.pos = (rb.pos + 1) % rb.size
		if rb.pos == 0 {
			rb.full = true
		}
	}
	return len(p), nil
}

func (rb *ringBuffer) Lines(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.full {
		start := rb.pos - n
		if start < 0 {
			start = 0
		}
		result := make([]string, rb.pos-start)
		copy(result, rb.lines[start:rb.pos])
		return result
	}

	if n > rb.size {
		n = rb.size
	}
	result := make([]string, 0, n)
	start := (rb.pos - n) % rb.size
	if start < 0 {
		start += rb.size
	}
	for i := 0; i < n; i++ {
		result = append(result, rb.lines[(start+i)%rb.size])
	}
	return result
}

// NewSingBoxHost creates a new SingBoxHost.
// binaryPath is the path to sing-box.exe.
// workDir is the working directory for sing-box (defaults to binary's directory).
func NewSingBoxHost(binaryPath string, workDir string) *SingBoxHost {
	return newExternalHost(
		"sing-box",
		binaryPath,
		workDir,
		func(path string) []string { return []string{"run", "-c", path} },
		func(path string) []string { return []string{"check", "-c", path} },
		extractSingBoxPort,
	)
}

// NewMihomoHost creates a host for the Mihomo core.
func NewMihomoHost(binaryPath string, workDir string) *SingBoxHost {
	return newExternalHost(
		"mihomo",
		binaryPath,
		workDir,
		func(path string) []string { return []string{"-f", path} },
		func(path string) []string { return []string{"-t", "-f", path} },
		extractMihomoPort,
	)
}

// NewXrayHost creates a host for Xray-core.
func NewXrayHost(binaryPath string, workDir string) *SingBoxHost {
	return newExternalHost(
		"xray",
		binaryPath,
		workDir,
		func(path string) []string { return []string{"run", "-c", path} },
		func(path string) []string { return []string{"run", "-test", "-c", path} },
		extractXrayPort,
	)
}

func newExternalHost(
	coreID, binaryPath, workDir string,
	runArgs, checkArgs func(string) []string,
	extractPort func(string) (int, error),
) *SingBoxHost {
	if workDir == "" {
		workDir = filepath.Dir(binaryPath)
	}
	return &SingBoxHost{
		coreID:         coreID,
		binaryPath:     binaryPath,
		workDir:        workDir,
		runArgs:        runArgs,
		checkArgs:      checkArgs,
		extractPort:    extractPort,
		restartBackoff: []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second},
		maxRestarts:    3,
		crashCh:        make(chan struct{}, 1),
		logBuf:         newRingBuffer(1000),
		status:         HostStatus{State: HostStateStopped},
	}
}

func (h *SingBoxHost) ID() string { return h.coreID }

// ── Binary Management ──

// FindBinary locates the sing-box.exe binary.
// It checks, in order: NAVO_SINGBOX_PATH env var, ./third_party/sing-box/,
// and %PROGRAMFILES%/Navo/sing-box/.
func FindBinary() (string, error) {
	paths := []string{}

	if p := os.Getenv("NAVO_SINGBOX_PATH"); p != "" {
		paths = append(paths, p)
	}

	paths = append(paths,
		filepath.Join("third_party", "sing-box", "sing-box.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES"), "Navo", "sing-box", "sing-box.exe"),
		filepath.Join("..", "third_party", "sing-box", "sing-box.exe"),
	)

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, nil
		}
	}

	return "", fmt.Errorf("CORE_001: sing-box binary not found in any search path")
}

// ValidateBinary checks sing-box binary integrity and version.
func ValidateBinary(path string) error {
	// Check existence
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("CORE_001: %w", err)
	}
	if fi.IsDir() {
		return fmt.Errorf("CORE_001: path is a directory: %s", path)
	}

	// Check version.txt
	dir := filepath.Dir(path)
	verFile := filepath.Join(dir, "version.txt")
	verData, err := os.ReadFile(verFile)
	if err != nil {
		return fmt.Errorf("CORE_001: cannot read version.txt: %w", err)
	}
	version := strings.TrimSpace(string(verData))

	// Parse version - expect format "1.12.0" or similar
	if !strings.HasPrefix(version, "1.") {
		return fmt.Errorf("CORE_001: unexpected version format: %s", version)
	}

	// Check SHA256 if available
	shaFile := filepath.Join(dir, "sha256.txt")
	if shaData, err := os.ReadFile(shaFile); err == nil {
		expected := strings.TrimSpace(string(shaData))
		actual, err := fileSHA256(path)
		if err != nil {
			return fmt.Errorf("CORE_001: cannot compute hash: %w", err)
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("CORE_001: SHA256 mismatch\n  expected: %s\n  actual:   %s", expected, actual)
		}
	}

	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ── Lifecycle ──

// Start launches the sing-box process.
func (h *SingBoxHost) Start(ctx context.Context, configPath string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.status.State == HostStateRunning {
		return 0, fmt.Errorf("CORE_000: already running, pid=%d", h.status.PID)
	}

	// Validate config first
	if err := h.ValidateConfig(ctx, configPath); err != nil {
		h.lastError = err.Error()
		return 0, fmt.Errorf("CONFIG_005: %w", err)
	}
	listenPort, err := h.extractPort(configPath)
	if err != nil {
		h.lastError = err.Error()
		return 0, fmt.Errorf("CONFIG_005: determine %s readiness port: %w", h.coreID, err)
	}
	if listenPort < 1 || listenPort > 65535 {
		h.lastError = "no valid readiness port in compiled config"
		return 0, fmt.Errorf("CONFIG_005: no valid readiness port in %s config", h.coreID)
	}

	// Calculate config hash
	configHash, _ := fileSHA256(configPath)
	if len(configHash) > 16 {
		configHash = configHash[:16]
	}

	h.configPath = configPath
	h.status = HostStatus{
		CoreID:     h.coreID,
		State:      HostStateStarting,
		ConfigHash: configHash,
	}

	// Build command
	cmdCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.cmd = exec.CommandContext(cmdCtx, h.binaryPath, h.runArgs(configPath)...)
	h.exitCh = make(chan error, 1)
	h.cmd.Dir = h.workDir
	h.cmd.Stdout = h.logBuf
	h.cmd.Stderr = h.logBuf
	winprocess.ConfigureHidden(h.cmd)

	// Start the process
	if err := h.cmd.Start(); err != nil {
		h.status.State = HostStateFailed
		h.status.LastError = err.Error()
		h.lastError = err.Error()
		return 0, fmt.Errorf("CORE_002: failed to start sing-box: %w", err)
	}

	h.status.PID = h.cmd.Process.Pid
	// Reap the process immediately. Startup failures otherwise remain invisible
	// until after the readiness timeout and get misreported as CORE_004.
	go waitProcess(h.cmd, h.exitCh)

	// Extract listen port from config for health checks
	h.listenPort = listenPort

	// Wait for port to be ready
	if err := h.waitForPort(ctx, h.listenPort, 10*time.Second, h.exitCh); err != nil {
		h.status.State = HostStateFailed
		h.status.LastError = err.Error()
		h.lastError = err.Error()
		// Try to kill the partially-started process
		h.cmd.Process.Kill()
		return 0, fmt.Errorf("CORE_004: port %d not ready: %w", h.listenPort, err)
	}

	h.status.State = HostStateRunning
	h.startedAt = time.Now()
	h.status.StartedAt = h.startedAt
	h.restartCount = 0

	// Start crash monitor
	go h.monitor()

	return h.status.PID, nil
}

// Stop terminates the sing-box process.
func (h *SingBoxHost) Stop(ctx context.Context, force bool, timeout time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.status.State != HostStateRunning && h.status.State != HostStateFailed {
		return nil // already stopped
	}

	h.status.State = HostStateStopping
	exitCh := h.exitCh

	if force {
		// Cancel context to force kill
		if h.cancel != nil {
			h.cancel()
		}
		if h.cmd != nil && h.cmd.Process != nil {
			_ = h.cmd.Process.Kill()
		}
	} else {
		// monitor owns exec.Cmd.Wait; Stop only waits for its exit notification.
		if h.cancel != nil {
			h.cancel()
		}

		select {
		case <-exitCh:
			// Process exited
		case <-ctx.Done():
			if h.cmd != nil && h.cmd.Process != nil {
				_ = h.cmd.Process.Kill()
			}
			return ctx.Err()
		case <-time.After(timeout):
			// Timeout - force kill
			if h.cmd != nil && h.cmd.Process != nil {
				_ = h.cmd.Process.Kill()
			}
			if exitCh != nil {
				<-exitCh
			}
		}
	}

	// Ensure context is cancelled
	if h.cancel != nil {
		h.cancel()
	}

	// Wait for port to be released
	if h.listenPort > 0 {
		if err := h.waitForPortFree(ctx, h.listenPort, 10*time.Second); err != nil {
			// Port not released is a warning in Phase 0
			h.lastError = fmt.Sprintf("port %d may not be released: %v", h.listenPort, err)
		}
	}

	h.cmd = nil
	h.cancel = nil
	h.exitCh = nil
	h.status = HostStatus{
		CoreID: h.coreID,
		State:  HostStateStopped,
	}
	h.listenPort = 0

	return nil
}

// Restart stops and restarts the proxy core.
func (h *SingBoxHost) Restart(ctx context.Context, configPath string) (int, error) {
	if err := h.Stop(ctx, false, 10*time.Second); err != nil {
		return 0, fmt.Errorf("stop failed during restart: %w", err)
	}
	return h.Start(ctx, configPath)
}

// Reload attempts a hot reload. sing-box 1.12+ does not support hot reload
// of the full config via signal, so this falls back to restart.
func (h *SingBoxHost) Reload(ctx context.Context, configPath string) error {
	return fmt.Errorf("hot reload not supported by sing-box; use Restart instead")
}

// ── Status & Health ──

// Status returns the current cached status.
func (h *SingBoxHost) Status() HostStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	s := h.status
	if s.State == HostStateRunning {
		s.Uptime = time.Since(h.startedAt)
	}
	return s
}

// HealthCheck performs a single health check.
func (h *SingBoxHost) HealthCheck(ctx context.Context) *HealthResult {
	result := &HealthResult{
		CheckedAt: time.Now(),
	}

	h.mu.RLock()
	pid := h.status.PID
	port := h.listenPort
	state := h.status.State
	h.mu.RUnlock()

	// Check 1: Process alive
	if state != HostStateRunning {
		result.Error = fmt.Sprintf("core not running, state=%s", state)
		return result
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		result.Error = fmt.Sprintf("cannot find process %d: %v", pid, err)
		return result
	}
	// On Windows, FindProcess always succeeds, so we need a different check.
	// We use the port check as the primary liveness indicator.
	_ = process

	result.ProcessOK = true

	// Check 2: Port listening
	if port > 0 {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
		if err != nil {
			result.Error = fmt.Sprintf("port %d not reachable: %v", port, err)
			return result
		}
		conn.Close()
		result.LatencyMs = time.Since(start).Milliseconds()
		result.PortOK = true
	} else {
		result.Error = "no readiness port configured"
		return result
	}

	result.Healthy = result.ProcessOK && result.PortOK
	return result
}

// ── Config Validation ──

// ValidateConfig calls "sing-box check" to validate a config file.
func (h *SingBoxHost) ValidateConfig(ctx context.Context, configPath string) error {
	cmd := exec.CommandContext(ctx, h.binaryPath, h.checkArgs(configPath)...)
	cmd.Dir = h.workDir
	winprocess.ConfigureHidden(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("config validation failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ── Logs ──

// GetLogs returns the last N lines of sing-box output.
func (h *SingBoxHost) GetLogs(lines int) ([]string, error) {
	return h.logBuf.Lines(lines), nil
}

// ── Reconciliation ──

// Reconcile performs network state cleanup.
// Phase 0: simplified version that only checks ports and zombie processes.
func (h *SingBoxHost) Reconcile(ctx context.Context) (*ReconcileResult, error) {
	result := &ReconcileResult{}

	// Check recovery state file
	stateFile := filepath.Join(os.TempDir(), "navo-recovery.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		// No state file means normal shutdown
		result.RecoveryState = RecoveryNormal
		return result, nil
	}

	stateStr := strings.TrimSpace(string(data))
	if stateStr == string(RecoveryNormal) {
		result.RecoveryState = RecoveryNormal
		return result, nil
	}

	// DIRTY_SHUTDOWN detected - perform cleanup
	result.RecoveryState = RecoveryDirty
	result.IssuesFound = append(result.IssuesFound, "dirty shutdown detected")

	// Check for zombie sing-box processes
	// On Windows, list processes with "tasklist"
	zombies := findZombieProcesses()
	for _, z := range zombies {
		result.IssuesFound = append(result.IssuesFound, fmt.Sprintf("zombie sing-box process PID=%d", z))
	}

	// Check if the configured port is still in use
	if h.listenPort > 0 {
		if !isPortFree(h.listenPort) {
			result.IssuesFound = append(result.IssuesFound, fmt.Sprintf("port %d still in use", h.listenPort))
		}
	}

	// Mark recovery complete
	result.RecoveryState = RecoveryReady
	if err := os.WriteFile(stateFile, []byte(RecoveryReady), 0644); err != nil {
		result.IssuesUnfixed = append(result.IssuesUnfixed, fmt.Sprintf("cannot write recovery state: %v", err))
	} else {
		result.IssuesFixed = append(result.IssuesFixed, "recovery state updated to READY")
	}

	return result, nil
}

// ── Internal Helpers ──

// monitor watches the sing-box process and handles crash recovery.
func (h *SingBoxHost) monitor() {
	h.mu.RLock()
	exitCh := h.exitCh
	h.mu.RUnlock()
	if exitCh == nil {
		return
	}

	err := <-exitCh

	h.mu.Lock()
	if h.status.State == HostStateStopping || h.status.State == HostStateStopped {
		h.mu.Unlock()
		return
	}

	if err != nil {
		h.lastError = err.Error()
	} else {
		h.lastError = "process exited with code 0 unexpectedly"
	}

	if h.restartCount >= h.maxRestarts {
		h.status.State = HostStateFailed
		h.status.LastError = h.lastError
		h.mu.Unlock()
		return
	}

	h.restartCount++
	h.status.RestartCount = h.restartCount
	backoff := h.restartBackoff[min(h.restartCount-1, len(h.restartBackoff)-1)]

	cfgPath := h.configPath
	h.cmd = nil
	h.cancel = nil
	h.mu.Unlock()

	time.Sleep(backoff)

	if cfgPath != "" {
		if _, startErr := h.Start(context.Background(), cfgPath); startErr != nil {
			h.mu.Lock()
			h.lastError = startErr.Error()
			h.status.State = HostStateFailed
			h.mu.Unlock()
		}
		return
	}

	h.mu.Lock()
	h.status.State = HostStateFailed
	h.status.LastError = h.lastError
	h.mu.Unlock()
}

// waitForPort polls until the given port is accepting connections.
func (h *SingBoxHost) waitForPort(
	ctx context.Context,
	port int,
	timeout time.Duration,
	exitCh <-chan error,
) error {
	if port == 0 {
		return nil // no port to wait for
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-exitCh:
			output := strings.Join(h.logBuf.Lines(8), " | ")
			if output == "" {
				output = "no process output"
			}
			return fmt.Errorf("core exited during startup: %v; output: %s", err, output)
		case <-deadline:
			return fmt.Errorf("timeout waiting for port %d after %v", port, timeout)
		case <-ticker.C:
			conn, err := net.DialTimeout(
				"tcp",
				fmt.Sprintf("127.0.0.1:%d", port),
				150*time.Millisecond,
			)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

func waitProcess(cmd *exec.Cmd, exitCh chan<- error) {
	err := cmd.Wait()
	exitCh <- err
	close(exitCh)
}

// waitForPortFree polls until the given port is released.
func (h *SingBoxHost) waitForPortFree(ctx context.Context, port int, timeout time.Duration) error {
	if port == 0 {
		return nil
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for port %d to be released after %v", port, timeout)
		case <-ticker.C:
			if isPortFree(port) {
				return nil // Port is free
			}
		}
	}
}

// isPortFree returns true if the port is not in use.
func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false // Port is in use
	}
	ln.Close()
	return true
}

// extractListenPort attempts to find the first listen port from a sing-box config.
// Phase 0: simple string parsing approach (not full JSON parsing for minimal deps).
func extractListenPort(configPath string) int {
	port, _ := extractSingBoxPort(configPath)
	return port
}

func extractSingBoxPort(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	var config struct {
		Inbounds []struct {
			Type       string `json:"type"`
			ListenPort int    `json:"listen_port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return 0, fmt.Errorf("parse sing-box config: %w", err)
	}
	for _, inbound := range config.Inbounds {
		if inbound.Type != "tun" && inbound.ListenPort > 0 {
			return inbound.ListenPort, nil
		}
	}
	return 0, nil
}

func extractMihomoPort(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	var config struct {
		MixedPort int `yaml:"mixed-port"`
		HTTPPort  int `yaml:"port"`
		SOCKSPort int `yaml:"socks-port"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return 0, fmt.Errorf("parse Mihomo config: %w", err)
	}
	for _, port := range []int{config.MixedPort, config.HTTPPort, config.SOCKSPort} {
		if port > 0 {
			return port, nil
		}
	}
	return 0, nil
}

func extractXrayPort(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	var config struct {
		Inbounds []struct {
			Port int `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return 0, fmt.Errorf("parse Xray config: %w", err)
	}
	for _, inbound := range config.Inbounds {
		if inbound.Port > 0 {
			return inbound.Port, nil
		}
	}
	return 0, nil
}

// findZombieProcesses checks for lingering sing-box processes.
// Phase 0: returns empty; full implementation requires Windows API.
func findZombieProcesses() []int {
	// Phase 0 simplified: rely on port check instead
	return nil
}
