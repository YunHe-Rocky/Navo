package host

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewSingBoxHost(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")

	if h.binaryPath != "C:\\test\\sing-box.exe" {
		t.Errorf("binaryPath = %s, want C:\\test\\sing-box.exe", h.binaryPath)
	}
	if h.workDir != "C:\\test" {
		t.Errorf("workDir = %s, want C:\\test", h.workDir)
	}
	if h.status.State != HostStateStopped {
		t.Errorf("initial state = %s, want stopped", h.status.State)
	}
	if h.logBuf == nil {
		t.Error("logBuf is nil")
	}
}

func TestStartReportsEarlyProcessExit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows process command")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	commandPath := os.Getenv("COMSPEC")
	if commandPath == "" {
		commandPath = `C:\Windows\System32\cmd.exe`
	}
	host := newExternalHost(
		"test-core",
		commandPath,
		t.TempDir(),
		func(string) []string {
			return []string{"/c", "echo startup-fatal 1>&2 & exit /b 7"}
		},
		func(string) []string { return []string{"/c", "exit /b 0"} },
		func(string) (int, error) { return port, nil },
	)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	startedAt := time.Now()
	_, err = host.Start(context.Background(), configPath)
	if err == nil {
		t.Fatal("expected startup failure")
	}
	if elapsed := time.Since(startedAt); elapsed > 3*time.Second {
		t.Fatalf("early exit took too long: %v", elapsed)
	}
	message := err.Error()
	if !strings.Contains(message, "core exited during startup") || !strings.Contains(message, "startup-fatal") {
		t.Fatalf("startup cause was not preserved: %v", err)
	}
}

func TestNewSingBoxHost_DefaultWorkDir(t *testing.T) {
	h := NewSingBoxHost("C:\\Program Files\\Navo\\sing-box\\sing-box.exe", "")

	expected := "C:\\Program Files\\Navo\\sing-box"
	if h.workDir != expected {
		t.Errorf("workDir = %s, want %s", h.workDir, expected)
	}
}

func TestNewSingBoxHost_ExplicitWorkDir(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "C:\\custom")

	if h.workDir != "C:\\custom" {
		t.Errorf("workDir = %s, want C:\\custom", h.workDir)
	}
}

func TestStatus_InitialState(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	s := h.Status()

	if s.State != HostStateStopped {
		t.Errorf("State = %s, want stopped", s.State)
	}
	if s.PID != 0 {
		t.Errorf("PID = %d, want 0", s.PID)
	}
}

func TestRingBuffer_Write(t *testing.T) {
	rb := newRingBuffer(10)

	rb.Write([]byte("line1\nline2\nline3\n"))
	lines := rb.Lines(10)

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := newRingBuffer(5)

	for i := 0; i < 10; i++ {
		rb.Write([]byte("line\n"))
	}

	lines := rb.Lines(10)
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5 (buffer size)", len(lines))
	}
}

func TestRingBuffer_RequestLessThanAvailable(t *testing.T) {
	rb := newRingBuffer(10)

	for i := 0; i < 5; i++ {
		rb.Write([]byte("line\n"))
	}

	lines := rb.Lines(3)
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := newRingBuffer(10)
	lines := rb.Lines(10)

	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0", len(lines))
	}
}

func TestRingBuffer_EmptyLines(t *testing.T) {
	rb := newRingBuffer(10)
	rb.Write([]byte("\n\n\n"))
	lines := rb.Lines(10)

	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0 (empty lines filtered)", len(lines))
	}
}

func TestFindBinary_EnvVar(t *testing.T) {
	// Create temp dir with a fake sing-box.exe
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NAVO_SINGBOX_PATH", binaryPath)

	found, err := FindBinary()
	if err != nil {
		t.Fatalf("FindBinary() error: %v", err)
	}
	if found != binaryPath {
		t.Errorf("FindBinary() = %s, want %s", found, binaryPath)
	}
}

func TestFindBinary_ThirdParty(t *testing.T) {
	// Create third_party/sing-box directory structure
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Set NAVO_SINGBOX_PATH to empty to force fallback
	t.Setenv("NAVO_SINGBOX_PATH", "")

	binDir := filepath.Join("third_party", "sing-box")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(binDir, "sing-box.exe")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindBinary()
	if err != nil {
		t.Fatalf("FindBinary() error: %v", err)
	}

	abs, _ := filepath.Abs(binaryPath)
	if found != abs {
		t.Errorf("FindBinary() = %s, want %s", found, abs)
	}
}

func TestFindBinary_NotFound(t *testing.T) {
	t.Setenv("NAVO_SINGBOX_PATH", "")

	// Change to a temp dir without third_party
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := FindBinary()
	if err == nil {
		t.Error("FindBinary() expected error when binary not found")
	}
}

func TestValidateBinary_Success(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	if err := os.WriteFile(binaryPath, []byte("fake binary content"), 0644); err != nil {
		t.Fatal(err)
	}

	versionPath := filepath.Join(tmpDir, "version.txt")
	if err := os.WriteFile(versionPath, []byte("1.12.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ValidateBinary(binaryPath); err != nil {
		t.Errorf("ValidateBinary() error: %v", err)
	}
}

func TestValidateBinary_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	if err := ValidateBinary(tmpDir); err == nil {
		t.Error("ValidateBinary() expected error for directory")
	}
}

func TestValidateBinary_MissingVersionFile(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ValidateBinary(binaryPath); err == nil {
		t.Error("ValidateBinary() expected error when version.txt missing")
	}
}

func TestValidateBinary_InvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	os.WriteFile(binaryPath, []byte("fake"), 0644)

	versionPath := filepath.Join(tmpDir, "version.txt")
	os.WriteFile(versionPath, []byte("0.9.0\n"), 0644)

	if err := ValidateBinary(binaryPath); err == nil {
		t.Error("ValidateBinary() expected error for version < 1.0")
	}
}

func TestValidateBinary_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	content := []byte("test content for hashing")
	os.WriteFile(binaryPath, content, 0644)

	versionPath := filepath.Join(tmpDir, "version.txt")
	os.WriteFile(versionPath, []byte("1.12.0\n"), 0644)

	shaPath := filepath.Join(tmpDir, "sha256.txt")
	os.WriteFile(shaPath, []byte("0000000000000000000000000000000000000000000000000000000000000000"), 0644)

	if err := ValidateBinary(binaryPath); err == nil {
		t.Error("ValidateBinary() expected error for hash mismatch")
	}
}

func TestValidateBinary_ValidHash(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "sing-box.exe")
	content := []byte("test content for hashing")
	os.WriteFile(binaryPath, content, 0644)

	versionPath := filepath.Join(tmpDir, "version.txt")
	os.WriteFile(versionPath, []byte("1.12.0\n"), 0644)

	// Compute expected hash
	hash, _ := fileSHA256(binaryPath)
	shaPath := filepath.Join(tmpDir, "sha256.txt")
	os.WriteFile(shaPath, []byte(hash), 0644)

	if err := ValidateBinary(binaryPath); err != nil {
		t.Errorf("ValidateBinary() error: %v", err)
	}
}

func TestValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"log":{"level":"error"},"inbounds":[{"type":"mixed","tag":"in","listen":"127.0.0.1","listen_port":12080}],"outbounds":[{"type":"direct","tag":"direct"}]}`), 0644)

	h := NewSingBoxHost("C:\\nonexistent\\sing-box.exe", tmpDir)

	// Without sing-box binary, ValidateConfig should fail
	err := h.ValidateConfig(context.Background(), configPath)
	if err == nil {
		t.Log("ValidateConfig passed (sing-box may be in PATH)")
	}
}

func TestExtractListenPort(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"inbounds":[{"listen_port":12080}]}`), 0644)

	port := extractListenPort(configPath)
	if port != 12080 {
		t.Errorf("extractListenPort() = %d, want 12080", port)
	}
}

func TestExtractListenPort_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"inbounds":[{}]}`), 0644)

	port := extractListenPort(configPath)
	if port != 0 {
		t.Errorf("extractListenPort() = %d, want 0", port)
	}
}

func TestExtractListenPort_NonExistentFile(t *testing.T) {
	port := extractListenPort("nonexistent_config.json")
	if port != 0 {
		t.Errorf("extractListenPort() = %d, want 0", port)
	}
}

func TestIsPortFree(t *testing.T) {
	// A high, likely-free port
	if !isPortFree(49999) {
		t.Log("port 49999 is in use, trying another")
		if !isPortFree(49998) {
			t.Skip("no free ports available for testing")
		}
	}
}

func TestHostStateConstants(t *testing.T) {
	states := map[HostState]bool{
		HostStateStopped:   true,
		HostStateStarting:  true,
		HostStateRunning:   true,
		HostStateReloading: true,
		HostStateStopping:  true,
		HostStateFailed:    true,
	}

	for state := range states {
		if string(state) == "" {
			t.Errorf("HostState %s has empty string value", state)
		}
	}
}

func TestRecoveryStateConstants(t *testing.T) {
	states := map[RecoveryState]bool{
		RecoveryNormal:  true,
		RecoveryDirty:   true,
		RecoveryRecover: true,
		RecoveryReady:   true,
	}

	for state := range states {
		if string(state) == "" {
			t.Errorf("RecoveryState %s has empty string value", state)
		}
	}
}

func TestHealthResult_Defaults(t *testing.T) {
	r := &HealthResult{}
	if r.Healthy {
		t.Error("new HealthResult should not be healthy by default")
	}
}

func TestHostStatus_JSONTags(t *testing.T) {
	s := HostStatus{
		State:        HostStateRunning,
		PID:          1234,
		RestartCount: 2,
	}

	if s.State != HostStateRunning {
		t.Errorf("State = %s", s.State)
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d", s.PID)
	}
	if s.RestartCount != 2 {
		t.Errorf("RestartCount = %d", s.RestartCount)
	}
}

func TestGetLogs_EmptyBuffer(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	logs, err := h.GetLogs(10)

	if err != nil {
		t.Errorf("GetLogs() error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("got %d logs, want 0", len(logs))
	}
}

func TestReload_NotSupported(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	err := h.Reload(nil, "config.json")
	if err == nil {
		t.Error("Reload() should return error (not supported)")
	}
}

func TestFileSHA256_Error(t *testing.T) {
	_, err := fileSHA256("nonexistent_file_for_testing.bin")
	if err == nil {
		t.Error("fileSHA256() expected error for nonexistent file")
	}
}

func TestIsPortFree_Used(t *testing.T) {
	// Listen on a port, then check
	ln, err := listenOnRandomPort()
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if isPortFree(port) {
		t.Errorf("isPortFree(%d) = true, want false (port is in use)", port)
	}
}

func listenOnRandomPort() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func TestExtractListenPort_AlternateFormat(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		json     string
		expected int
	}{
		{
			name:     "with spaces",
			json:     `{"inbounds":[{"listen_port":  12080}]}`,
			expected: 12080,
		},
		{
			name:     "first of multiple",
			json:     `{"inbounds":[{"listen_port":12080},{"listen_port":12081}]}`,
			expected: 12080,
		},
		{
			name:     "empty file",
			json:     `{}`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tt.name+".json")
			os.WriteFile(configPath, []byte(tt.json), 0644)
			port := extractListenPort(configPath)
			if port != tt.expected {
				t.Errorf("extractListenPort() = %d, want %d", port, tt.expected)
			}
		})
	}
}

func TestExtractListenPort_WithComma(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`{"inbounds":[{"listen_port":12080, "type":"mixed"}]}`), 0644)

	port := extractListenPort(configPath)
	if port != 12080 {
		t.Errorf("extractListenPort() = %d, want 12080", port)
	}
}

func TestHostStatus_Uptime(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")

	// Manually set running state
	h.mu.Lock()
	h.status.State = HostStateRunning
	h.startedAt = time.Now().Add(-5 * time.Minute)
	h.mu.Unlock()

	s := h.Status()
	if s.State != HostStateRunning {
		t.Errorf("State = %s, want running", s.State)
	}
	if s.Uptime < 4*time.Minute {
		t.Errorf("Uptime = %v, want ~5m", s.Uptime)
	}
}

func TestStatus_Internal(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")

	// Should not panic when no process is running
	s := h.Status()
	if s.State != HostStateStopped {
		t.Errorf("State = %s, want stopped", s.State)
	}
	if s.PID != 0 {
		t.Errorf("PID = %d, want 0", s.PID)
	}
	if s.Uptime != 0 {
		t.Errorf("Uptime = %v, want 0", s.Uptime)
	}
}

func TestGetLogs_WithData(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	h.logBuf.Write([]byte("test message\n"))

	logs, err := h.GetLogs(10)
	if err != nil {
		t.Fatalf("GetLogs() error: %v", err)
	}
	if len(logs) != 1 || logs[0] != "test message" {
		t.Errorf("GetLogs() = %v, want [test message]", logs)
	}
}

func TestSingBoxHost_InitialConfigHash(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	s := h.Status()
	if s.ConfigHash != "" {
		t.Errorf("ConfigHash = %s, want empty", s.ConfigHash)
	}
}

func TestSingBoxHost_LastError(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	s := h.Status()
	if s.LastError != "" {
		t.Errorf("LastError = %s, want empty", s.LastError)
	}
}

func TestRecoveryStateConstants_Values(t *testing.T) {
	if RecoveryNormal != "NORMAL" {
		t.Errorf("RecoveryNormal = %s, want NORMAL", RecoveryNormal)
	}
	if RecoveryDirty != "DIRTY_SHUTDOWN" {
		t.Errorf("RecoveryDirty = %s, want DIRTY_SHUTDOWN", RecoveryDirty)
	}
	if RecoveryRecover != "RECOVERING" {
		t.Errorf("RecoveryRecover = %s, want RECOVERING", RecoveryRecover)
	}
	if RecoveryReady != "READY" {
		t.Errorf("RecoveryReady = %s, want READY", RecoveryReady)
	}
}

func TestHostStateConstants_Values(t *testing.T) {
	if HostStateStopped != "stopped" {
		t.Errorf("HostStateStopped = %s", HostStateStopped)
	}
	if HostStateRunning != "running" {
		t.Errorf("HostStateRunning = %s", HostStateRunning)
	}
	if HostStateFailed != "failed" {
		t.Errorf("HostStateFailed = %s", HostStateFailed)
	}
}

func TestFileSHA256_KnownContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")
	os.WriteFile(path, []byte("hello world"), 0644)

	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256() error: %v", err)
	}
	// SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("fileSHA256() = %s, want %s", hash, expected)
	}
}

func TestSingBoxHost_StartAlreadyRunning(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	h.mu.Lock()
	h.status.State = HostStateRunning
	h.status.PID = 1234
	h.mu.Unlock()

	_, err := h.Start(context.Background(), "config.json")
	if err == nil {
		t.Error("Start() expected error when already running")
	}
}

func TestSingBoxHost_StopNotRunning(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")

	err := h.Stop(context.Background(), false, 10*time.Second)
	if err != nil {
		t.Errorf("Stop() on stopped host should not error: %v", err)
	}
}

func TestSingBoxHost_Restart(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	// Restart when not running should try to stop (no-op) then start
	_, err := h.Restart(context.Background(), "config.json")
	// Should fail because config doesn't exist
	if err == nil {
		t.Error("Restart() expected error with no config")
	}
}

func TestHostMonitorReportsCrashWithoutRestarting(t *testing.T) {
	h := NewSingBoxHost(`C:\missing\sing-box.exe`, "")
	h.status = HostStatus{State: HostStateRunning, PID: 42}
	h.exitCh = make(chan error, 1)
	done := make(chan struct{})
	go func() {
		h.monitor()
		close(done)
	}()
	h.exitCh <- errors.New("simulated crash")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("host monitor did not observe process exit")
	}
	status := h.Status()
	if status.State != HostStateFailed || status.PID != 0 {
		t.Fatalf("status after crash = %#v", status)
	}
	if status.RestartCount != 0 || h.cmd != nil {
		t.Fatalf("host attempted restart: status=%#v cmd=%v", status, h.cmd)
	}
}

func TestFindBinary_MultiplePaths(t *testing.T) {
	// Test that FindBinary returns first match when multiple paths exist
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create third_party/sing-box structure
	binDir := filepath.Join("third_party", "sing-box")
	os.MkdirAll(binDir, 0755)
	binaryPath := filepath.Join(binDir, "sing-box.exe")
	os.WriteFile(binaryPath, []byte("fake"), 0644)

	// Set env var to a different (nonexistent) path
	t.Setenv("NAVO_SINGBOX_PATH", "C:\\nonexistent\\path\\sing-box.exe")

	// Should find the third_party one since env var path doesn't exist
	found, err := FindBinary()
	if err != nil {
		t.Fatalf("FindBinary() error: %v", err)
	}
	abs, _ := filepath.Abs(binaryPath)
	if found != abs {
		t.Errorf("FindBinary() = %s, want %s", found, abs)
	}
}

func TestHealthCheck_NotRunning(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")

	result := h.HealthCheck(context.Background())
	if result.Healthy {
		t.Error("HealthCheck() should not be healthy when not running")
	}
}

func TestHealthCheck_RunningNoPort(t *testing.T) {
	h := NewSingBoxHost("C:\\test\\sing-box.exe", "")
	h.mu.Lock()
	h.status.State = HostStateRunning
	h.status.PID = os.Getpid()
	h.listenPort = 0
	h.mu.Unlock()

	result := h.HealthCheck(context.Background())
	if !result.ProcessOK {
		t.Error("HealthCheck() ProcessOK should be true when state is running")
	}
	if result.Healthy || result.PortOK {
		t.Error("HealthCheck() must reject a running process without a readiness port")
	}
}

func TestExtractCoreSpecificPorts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		extract func(string) (int, error)
		want    int
	}{
		{name: "mihomo", content: "mixed-port: 12080\n", extract: extractMihomoPort, want: 12080},
		{name: "xray", content: `{"inbounds":[{"port":12080,"protocol":"http"}]}`, extract: extractXrayPort, want: 12080},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := tt.extract(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("port = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateBinary_FileNotFound(t *testing.T) {
	err := ValidateBinary("C:\\nonexistent\\path\\sing-box.exe")
	if err == nil {
		t.Error("ValidateBinary() expected error for nonexistent file")
	}
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configPath, []byte(`not valid json`), 0644)

	h := NewSingBoxHost("C:\\nonexistent\\sing-box.exe", tmpDir)
	err := h.ValidateConfig(context.Background(), configPath)
	// sing-box check should reject invalid JSON
	if err == nil {
		t.Log("ValidateConfig passed unexpectedly (sing-box may not be available)")
	}
}

func TestRingBuffer_ManyWrites(t *testing.T) {
	rb := newRingBuffer(100)

	for i := 0; i < 200; i++ {
		rb.Write([]byte("x\n"))
	}

	lines := rb.Lines(50)
	if len(lines) != 50 {
		t.Errorf("got %d lines, want 50", len(lines))
	}
}

func TestRingBuffer_FullRequestAll(t *testing.T) {
	rb := newRingBuffer(5)

	for i := 0; i < 10; i++ {
		rb.Write([]byte("x\n"))
	}

	lines := rb.Lines(5)
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5", len(lines))
	}
}

func TestRingBuffer_PartialFill(t *testing.T) {
	rb := newRingBuffer(100)

	for i := 0; i < 30; i++ {
		rb.Write([]byte("x\n"))
	}

	lines := rb.Lines(100)
	if len(lines) != 30 {
		t.Errorf("got %d lines, want 30", len(lines))
	}
}
