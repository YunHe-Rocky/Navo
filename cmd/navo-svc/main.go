// navo-svc is the Phase 2 Windows Service for Navo.
// It manages the sing-box proxy core and exposes a Named Pipe IPC interface.
//
// Usage:
//
//	navo-svc run        Run in standalone mode (development)
//	navo-svc install    Install as Windows Service
//	navo-svc uninstall  Uninstall Windows Service
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"navo/internal/host"
	"navo/internal/logstore"
	"navo/internal/service"
)

var (
	singboxPath = flag.String("singbox", "", "path to sing-box.exe")
	mihomoPath  = flag.String("mihomo", "", "path to mihomo.exe")
	xrayPath    = flag.String("xray", "", "path to xray.exe")
	configPath  = flag.String("config", "configs/test_direct.json", "path to config JSON")
	pipeName    = flag.String("pipe", "Navo.Agent.Service.v1", "named pipe name")
	dataDir     = flag.String("data", "", "directory for persistent service settings")
	proxyPort   = flag.Int("proxy-port", 12080, "local mixed proxy port")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	binaryPath := *singboxPath
	if binaryPath == "" {
		var err error
		binaryPath, err = host.FindBinary()
		if err != nil {
			log.Printf("[WARN] sing-box not found: %v (specify --singbox)", err)
		}
	}
	if !filepath.IsAbs(binaryPath) {
		binaryPath, _ = filepath.Abs(binaryPath)
	}
	cfgPath := *configPath
	if !filepath.IsAbs(cfgPath) {
		cfgPath, _ = filepath.Abs(cfgPath)
	}
	if *dataDir != "" {
		if err := logstore.Configure(filepath.Join(*dataDir, "structured.log.jsonl")); err != nil {
			log.Fatalf("initialize structured logging: %v", err)
		}
	}

	svc, err := service.New(service.Config{
		SingBoxPath:        binaryPath,
		MihomoPath:         *mihomoPath,
		XrayPath:           *xrayPath,
		ConfigPath:         cfgPath,
		ConfigDir:          *dataDir,
		PipeName:           *pipeName,
		ProxyPort:          *proxyPort,
		EnableExternalPipe: true,
	})
	if err != nil {
		log.Fatalf("create service: %v", err)
	}

	switch args[0] {
	case "run":
		fmt.Printf("[INFO] Navo Service starting in standalone mode\n")
		fmt.Printf("[INFO] Pipe: \\\\.\\pipe\\%s\n", *pipeName)
		fmt.Printf("[INFO] Config: %s\n", *configPath)
		fmt.Printf("[INFO] Press Ctrl+C to stop\n\n")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		go func() {
			<-sigCh
			fmt.Println("\n[INFO] stopping...")
			cancel()
		}()

		if err := svc.Run(ctx); err != nil {
			log.Fatalf("service error: %v", err)
		}

		fmt.Println("[OK] service stopped")

	case "install":
		fmt.Println("[INFO] Installing Navo Windows Service...")
		fmt.Println("[WARN] Requires Administrator privileges")
		fmt.Println("[INFO] Run: sc create NavoService binPath= \"...\" start= auto")
		fmt.Println("[INFO] Or use the MSI installer in Phase 5")

	case "uninstall":
		fmt.Println("[INFO] Uninstalling Navo Windows Service...")
		fmt.Println("[WARN] Requires Administrator privileges")
		fmt.Println("[INFO] Run: sc delete NavoService")

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `navo-svc - Navo Windows Service

Usage:
  navo-svc run        Run in standalone mode
  navo-svc install    Install as Windows Service (admin required)
  navo-svc uninstall  Remove Windows Service (admin required)

Options:
  --singbox PATH      Path to sing-box.exe
  --config PATH       Path to sing-box config JSON
  --pipe NAME         Named pipe name (default: Navo.Agent.Service.v1)
  --data PATH         Persistent settings directory
`)
}
