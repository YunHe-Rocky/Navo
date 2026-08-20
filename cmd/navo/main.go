// navo is the single-process Windows desktop entry point for Navo.
// It embeds the service, agent, and desktop UI launcher in one process.
//
// Architecture:
//
//	navo.exe (one process, two goroutines)
//	  ├── Service goroutine → sing-box.exe (child, data plane)
//	  ├── Agent goroutine   → system proxy + UI Named Pipe listener
//	  └── Wails UI          → navo_app.exe (child, desktop UI)
//
//go:generate go-winres simply --icon winres/navo.ico --product-name Navo --file-description "Navo Network Manager" --copyright "Copyright (C) 2026" --original-filename navo.exe --manifest gui
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"navo/internal/agent"
	"navo/internal/agent/systemproxy"
	"navo/internal/coreadapter"
	"navo/internal/coremanifest"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/fsatomic"
	runtimeconfig "navo/internal/infrastructure/config"
	"navo/internal/infrastructure/localstate"
	"navo/internal/initialization"
	"navo/internal/logstore"
	"navo/internal/pipe"
	"navo/internal/service"
	"navo/internal/winprocess"
)

var (
	proxyPort = flag.Int("proxy", 12080, "preferred local mixed-proxy port")
	silent    = flag.Bool("silent", false, "start minimized to system tray")
)

const (
	uiPipeName            = "Navo.UI.Agent.v1"
	serviceStartupTimeout = 45 * time.Second
)

func waitForServiceStartup(
	ready <-chan struct{},
	exited <-chan error,
	timeout time.Duration,
) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ready:
		return nil
	case err := <-exited:
		if err == nil {
			return fmt.Errorf("service exited before startup completed")
		}
		return fmt.Errorf("service startup failed: %w", err)
	case <-timer.C:
		return fmt.Errorf("service startup timed out after %s", timeout)
	}
}

type managedProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func main() {
	flag.Parse()
	defer func() {
		if r := recover(); r != nil {
			showFatalError(fmt.Sprintf("Navo crashed: %v", r))
		}
	}()

	executableDir := exeDir()
	dataDir := localDataDir(executableDir)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fatal("create data directory: %v", err)
	}
	if err := fsatomic.RepairTree(dataDir); err != nil {
		fatal("repair local profile access: %v", err)
	}
	initializationResult, err := initialization.Run(dataDir)
	if err != nil || !initializationResult.Ready {
		fatal("privacy initialization failed (%s): %v", initializationResult.ErrorCode, err)
	}
	if err := logstore.Configure(filepath.Join(dataDir, "structured.log.jsonl")); err != nil {
		fatal("initialize structured logging: %v", err)
	}
	_ = logstore.Emit(logstore.LevelInfo, "Launcher", "Initialization", "privacy initialization completed", map[string]any{
		"first_run": initializationResult.FirstRun, "migrated": initializationResult.Migrated,
		"foreign_context": initializationResult.ForeignContext, "privacy_reset": initializationResult.PrivacyReset,
	})
	if envPath, err := runtimeconfig.LoadDotEnv(dataDir); err != nil {
		fatal("load environment: %v", err)
	} else if envPath != "" {
		log.Printf("[navo] environment loaded: %s", envPath)
	}
	logFile, logPath, err := configureLogging(executableDir)
	if err != nil {
		fatal("initialize logging: %v", err)
	}
	defer logFile.Close()
	log.Printf("[navo] log: %s", logPath)
	log.Printf(
		"[initialization] ready first_run=%t migrated=%t foreign_context=%t privacy_reset=%t",
		initializationResult.FirstRun,
		initializationResult.Migrated,
		initializationResult.ForeignContext,
		initializationResult.PrivacyReset,
	)

	// A valid kill-on-close Job Object is mandatory: without it a launcher
	// crash can leave the desktop UI or a proxy core running in the background.
	if err := initJobObject(); err != nil {
		fatal("initialize mandatory process Job Object: %v", err)
	}

	instanceLock, alreadyRunning, err := acquireSingleInstance()
	if err != nil {
		fatal("acquire single-instance mutex: %v", err)
	}
	if alreadyRunning {
		log.Printf("[navo] existing instance detected, requesting desktop UI")
		if err := requestExistingUI(3 * time.Second); err != nil {
			log.Printf("[navo] existing instance UI request failed: %v", err)
			focusExistingWindow()
		}
		return
	}
	defer instanceLock.Close()

	selectionRepository, revisionRepository, err := localstate.Open(
		filepath.Join(dataDir, "state", "repositories.json"),
	)
	if err != nil {
		fatal("initialize local runtime repositories: %v", err)
	}
	singboxPort := findFreePort(*proxyPort, 100)
	configPath := filepath.Join(dataDir, "runtime.json")
	if err := writeConfig(configPath, singboxPort); err != nil {
		fatal("write runtime config: %v", err)
	}

	singboxPath := filepath.Join(executableDir, "third_party", "sing-box", "sing-box.exe")
	requireFile(singboxPath, "sing-box executable")
	mihomoPath := optionalFile(filepath.Join(executableDir, "third_party", "mihomo", "mihomo.exe"))
	xrayPath := optionalFile(filepath.Join(executableDir, "third_party", "xray", "xray.exe"))
	if err := verifyCoreInstallations(context.Background(), executableDir); err != nil {
		fatal("verify bundled cores: %v", err)
	}
	uiPath := findUIExecutable(executableDir)
	if !fileExists(uiPath) {
		fatal(
			"desktop UI executable not found: %s; build the complete application with scripts/package.ps1",
			uiPath,
		)
	}
	uiManager := newUIProcessManager(uiPath, logFile)

	// ── Service (in-process) ──
	svc, err := service.New(service.Config{
		SingBoxPath:         singboxPath,
		MihomoPath:          mihomoPath,
		XrayPath:            xrayPath,
		ConfigPath:          configPath,
		ConfigDir:           dataDir,
		ProxyPort:           singboxPort,
		SelectionRepository: selectionRepository,
		RevisionRepository:  revisionRepository,
		DeferCoreStart:      true,
	})
	if err != nil {
		fatal("create service: %v", err)
	}

	serviceExit := make(chan error, 1)
	go func() {
		serviceExit <- service.RunStandalone(svc)
	}()

	if err := waitForServiceStartup(svc.Ready(), serviceExit, serviceStartupTimeout); err != nil {
		fatal("%v", err)
	}
	log.Printf("[navo] service ready")

	// Agent owns the user-scoped UI pipe and system proxy integration.
	trayExit := make(chan struct{})
	var requestTrayExit sync.Once
	var trayRef atomic.Pointer[trayController]
	ag, err := agent.New(agent.Config{
		UIPipeName:         uiPipeName,
		ProxyPort:          singboxPort,
		ProxyManager:       systemproxy.NewManagerWithDirectory(filepath.Join(dataDir, "agent")),
		CaptureJournalPath: filepath.Join(dataDir, "agent", "capture_transition.json"),
		CaptureRouteProbeFn: func(ctx context.Context, mode capture.Mode, runtimeMode string) error {
			directRouting := strings.EqualFold(strings.TrimSpace(runtimeMode), "direct")
			switch mode {
			case capture.ModeSystemProxy:
				if directRouting {
					return systemproxy.ProbeDefaultDirectRouting(ctx)
				}
				return systemproxy.ProbeDefaultProxy(ctx)
			case capture.ModeTUN:
				if directRouting {
					return systemproxy.ProbeDirectRouting(ctx)
				}
				return systemproxy.ProbeDirect(ctx)
			default:
				return nil
			}
		},
		SendToServiceContextFn: func(ctx context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			return svc.Dispatch(ctx, msg), nil
		},
		ShowUIFn: uiManager.Show,
		MinimizeToTrayFn: func() error {
			tray := trayRef.Load()
			if tray == nil {
				return fmt.Errorf("native tray is not ready")
			}
			return tray.EnsureVisible()
		},
		RequestExitFn: func() { requestTrayExit.Do(func() { close(trayExit) }) },
	})
	if err != nil {
		fatal("create agent: %v", err)
	}

	agentCtx, cancelAgent := context.WithCancel(context.Background())
	defer cancelAgent()
	agentExit := make(chan error, 1)
	go func() {
		agentExit <- ag.Run(agentCtx)
	}()

	if !*silent {
		if err := waitForPipe(uiPipeName, 5*time.Second); err != nil {
			log.Printf("[navo] WARNING: UI control pipe not ready: %v", err)
		}
		if err := uiManager.Show(); err != nil {
			fatal("start desktop UI: %v", err)
		}
	}
	log.Printf("[navo] ready proxy=%d tray_only=%t", singboxPort, *silent)

	// ── Native Windows Tray ──
	// Runs a tray icon in the launcher itself, independent of the desktop UI.
	// Provides reliable left-click (show window) and right-click menu (show/exit).
	iconPath := filepath.Join(executableDir, "app_ui", "tray_icon.ico")
	trayBackend := newAgentTrayBackend(ag.Dispatch)
	tray, trayErr := startTray(iconPath,
		func() {
			if err := uiManager.Show(); err != nil {
				log.Printf("[navo] show desktop UI: %v", err)
			}
		},
		func() { requestTrayExit.Do(func() { close(trayExit) }) }, // menu Exit -> trigger shutdown
		trayBackend,
	)
	if trayErr == nil {
		trayRef.Store(tray)
	}
	if trayErr != nil {
		log.Printf("[navo] WARNING: tray icon failed: %v", trayErr)
		if err := uiManager.Show(); err != nil {
			fatal("start fallback desktop UI: %v", err)
		}
	} else {
		log.Printf("[navo] native tray ready")
	}

	// ── Wait ──
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	waitAgent := (<-chan error)(agentExit)
	waitService := (<-chan error)(serviceExit)
	var uiExit <-chan error
	if trayErr != nil {
		uiExit = uiManager.Events()
	}

	select {
	case sig := <-signalCh:
		log.Printf("[navo] signal: %s", sig)
	case <-trayExit:
		log.Printf("[navo] tray exit requested")
	case err := <-uiExit:
		log.Printf("[navo] fallback UI exited: %v", err)
	case err := <-agentExit:
		waitAgent = nil
		if err != nil {
			log.Printf("[navo] agent exited: %v", err)
		}
	case err := <-serviceExit:
		waitService = nil
		if err != nil {
			log.Printf("[navo] service exited: %v", err)
		}
	}

	log.Printf("[navo] shutting down")
	tray.Close(2 * time.Second)
	// The UI is part of the launcher's kill-on-close Job Object. Let the
	// launcher finish its own graceful shutdown first; terminating the Wails
	// process here can propagate WebView2's job teardown back to the parent.
	_ = ag.DisableProxy()
	cancelAgent()
	svc.Stop()
	uiManager.Stop(2 * time.Second)

	// Wait up to 3 seconds for graceful shutdown, then force exit.
	// The Job Object will clean up any remaining child processes.
	shutdownTimer := time.NewTimer(3 * time.Second)
	defer shutdownTimer.Stop()
	for waitAgent != nil || waitService != nil {
		select {
		case <-waitAgent:
			waitAgent = nil
		case <-waitService:
			waitService = nil
		case <-shutdownTimer.C:
			log.Printf("[navo] shutdown timed out, force exiting (Job Object will clean up)")
			waitAgent = nil
			waitService = nil
		}
	}
	if !shutdownTimer.Stop() {
		select {
		case <-shutdownTimer.C:
		default:
		}
	} else {
		log.Printf("[navo] graceful shutdown complete")
	}
	cleanRuntimeCache(dataDir)
	log.Printf("[navo] shutdown complete")
}

func verifyCoreInstallations(ctx context.Context, root string) error {
	manifestPath := filepath.Join(root, "CORE_MANIFEST.json")
	if !fileExists(manifestPath) {
		if cwd, err := os.Getwd(); err == nil && fileExists(filepath.Join(cwd, "CORE_MANIFEST.json")) {
			root = cwd
			manifestPath = filepath.Join(cwd, "CORE_MANIFEST.json")
		}
	}
	manifest, err := coremanifest.Load(manifestPath)
	if err != nil {
		return err
	}
	if err := manifest.VerifyFiles(root); err != nil {
		return err
	}

	registry := coreadapter.NewDefaultRegistry()
	for _, coreType := range core.All() {
		entry, _ := manifest.Find(coreType)
		adapter, err := registry.Get(coreType)
		if err != nil {
			return err
		}
		binaryPath := filepath.Join(root, filepath.FromSlash(entry.RelativePath))
		versionCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		version, detectErr := adapter.DetectVersion(versionCtx, binaryPath)
		cancel()
		if detectErr != nil {
			return fmt.Errorf("%s: %w", coreType, detectErr)
		}
		if strings.TrimPrefix(version.Raw, "v") != strings.TrimPrefix(entry.Version, "v") {
			return fmt.Errorf("%s version mismatch: manifest=%s binary=%s", coreType, entry.Version, version.Raw)
		}
	}
	return nil
}

func optionalFile(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ── managedProcess ──

func startManagedProcess(cmd *exec.Cmd, dir string, output io.Writer, hideWindow bool) (*managedProcess, error) {
	cmd.Dir = dir
	cmd.Stdout = output
	cmd.Stderr = output
	if hideWindow {
		winprocess.ConfigureHidden(cmd)
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &managedProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

func (p *managedProcess) waitError() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// ── Helpers ──

func waitForPipe(name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ch, err := pipe.Dial(name)
		if err == nil {
			return ch.Close()
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("pipe %s: %w", name, lastErr)
}

func configureLogging(executableDir string) (*os.File, string, error) {
	candidates := []string{
		filepath.Join(executableDir, "log"),
		filepath.Join(localDataDir(executableDir), "log"),
	}
	var lastErr error
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0700); err != nil {
			lastErr = err
			continue
		}
		path := filepath.Join(dir, "navo.log")
		rotateLog(path, 5*1024*1024)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			lastErr = err
			continue
		}
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
		log.SetOutput(io.MultiWriter(file, os.Stderr))
		logPath = path
		return file, path, nil
	}
	return nil, "", lastErr
}

func rotateLog(path string, maxSize int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxSize {
		return
	}
	_ = os.Remove(path + ".2")
	_ = os.Rename(path+".1", path+".2")
	_ = os.Rename(path, path+".1")
}

func localDataDir(executableDir string) string {
	if root := os.Getenv("LOCALAPPDATA"); root != "" {
		return filepath.Join(root, "Navo")
	}
	return filepath.Join(executableDir, "data")
}

func findUIExecutable(executableDir string) string {
	candidates := []string{
		filepath.Join(executableDir, "app_ui", "navo_app.exe"),
		filepath.Join(executableDir, "navo_app", "build", "bin", "navo_app.exe"),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return candidates[0]
}

func requireFile(path, label string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		fatal("%s not found: %s", label, path)
	}
}

func findFreePort(start, count int) int {
	for port := start; port < start+count; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	fatal("no free port in range %d-%d", start, start+count-1)
	return 0
}

func writeConfig(path string, port int) error {
	logPath := filepath.ToSlash(filepath.Join(filepath.Dir(path), "sing-box.log"))
	cfg := fmt.Sprintf(`{
  "inbounds": [{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": %d}],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "log": {"level": "info", "output": "%s", "timestamp": true}
}`, port, logPath)
	return fsatomic.WriteFile(path, []byte(cfg), 0600)
}

var jobObject syscall.Handle

type jobObjectBasicLimitInformation struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
}

type ioCounters struct {
	readOperationCount  uint64
	writeOperationCount uint64
	otherOperationCount uint64
	readTransferCount   uint64
	writeTransferCount  uint64
	otherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	basicLimitInformation jobObjectBasicLimitInformation
	ioInfo                ioCounters
	processMemoryLimit    uintptr
	jobMemoryLimit        uintptr
	peakProcessMemoryUsed uintptr
	peakJobMemoryUsed     uintptr
}

func initJobObject() error {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	createJob := k32.NewProc("CreateJobObjectW")
	assignJob := k32.NewProc("AssignProcessToJobObject")
	setInfo := k32.NewProc("SetInformationJobObject")
	closeHandle := k32.NewProc("CloseHandle")

	h, _, callErr := createJob.Call(0, 0)
	if h == 0 {
		return fmt.Errorf("CreateJobObjectW: %w", callErr)
	}
	fail := func(operation string, err error) error {
		closeHandle.Call(h)
		return fmt.Errorf("%s: %w", operation, err)
	}

	const (
		jobObjectExtendedLimitInformationClass = 9
		jobObjectLimitKillOnJobClose           = 0x00002000
	)
	info := jobObjectExtendedLimitInformation{}
	info.basicLimitInformation.limitFlags = jobObjectLimitKillOnJobClose
	if ok, _, callErr := setInfo.Call(
		h,
		jobObjectExtendedLimitInformationClass,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	); ok == 0 {
		return fail("SetInformationJobObject", callErr)
	}

	currentProcess, _, _ := k32.NewProc("GetCurrentProcess").Call()
	if ok, _, callErr := assignJob.Call(h, currentProcess); ok == 0 {
		return fail("AssignProcessToJobObject", callErr)
	}

	jobObject = syscall.Handle(h)
	return nil
}

func cleanRuntimeCache(dataDir string) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		// Generated configs are rebuilt on startup. Preserve runtime_state.json:
		// it stores the user's selected core, outbound, mode and TUN settings.
		if strings.HasPrefix(n, "runtime.") && strings.HasSuffix(n, ".json") {
			os.Remove(filepath.Join(dataDir, n))
		}
	}
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		fatal("resolve executable path: %v", err)
	}
	return filepath.Dir(exe)
}

func fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[navo] FATAL: %s", msg)
	showFatalError(msg)
	os.Exit(1)
}

func showFatalError(msg string) {
	// Write error to a standalone file so the user can copy the text freely
	errDir := filepath.Dir(logPath)
	errFile := filepath.Join(errDir, "navo-error.txt")
	fullMsg := fmt.Sprintf("Navo Fatal Error\n\n%s\n\nLog file: %s", msg, logPath)
	os.WriteFile(errFile, []byte(fullMsg), 0644)
	// Open in Notepad so the user can read and copy
	exec.Command("notepad.exe", errFile).Start()
}

var logPath string // set by configureLogging

func focusExistingWindow() bool {
	u32 := syscall.NewLazyDLL("user32.dll")
	class, _ := syscall.UTF16PtrFromString("NavoAppWindow")
	name, _ := syscall.UTF16PtrFromString("Navo")
	hwnd, _, _ := u32.NewProc("FindWindowW").Call(
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(name)))
	if hwnd == 0 {
		return false
	}
	u32.NewProc("ShowWindow").Call(hwnd, 9) // SW_RESTORE
	u32.NewProc("SetForegroundWindow").Call(hwnd)
	return true
}
