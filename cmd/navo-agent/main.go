// navo-agent is the Phase 2 User Agent for Navo.
// It runs in the user session, manages system proxy, and communicates
// with the Windows Service via Named Pipe.
//
// Usage:
//
//	navo-agent run        Run the agent
//	navo-agent proxy on   Enable system proxy
//	navo-agent proxy off  Disable system proxy
//	navo-agent status     Show status
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"navo/internal/agent"
)

var (
	pipeName = flag.String("pipe", "Navo.Agent.Service.v1", "service pipe name")
	uiPipe   = flag.String("ui-pipe", "Navo.UI.Agent.v1", "UI pipe name")
	proxyPort = flag.Int("proxy-port", 12080, "local proxy port")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	ag, err := agent.New(agent.Config{
		ServicePipeName: *pipeName,
		UIPipeName:      *uiPipe,
		ProxyPort:       *proxyPort,
	})
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	switch args[0] {
	case "run":
		fmt.Printf("[INFO] Navo Agent starting\n")
		fmt.Printf("[INFO] Proxy port: %d\n", *proxyPort)
		fmt.Printf("[INFO] UI Pipe: \\\\.\\pipe\\%s\n", *uiPipe)
		fmt.Printf("[INFO] Service Pipe: \\\\.\\pipe\\%s\n", *pipeName)
		fmt.Printf("[INFO] Press Ctrl+C to stop\n\n")

		if err := agent.RunStandalone(ag); err != nil {
			log.Fatalf("agent error: %v", err)
		}

		fmt.Println("[OK] agent stopped")

	case "proxy":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: navo-agent proxy <on|off|toggle|status>\n")
			os.Exit(1)
		}
		switch args[1] {
		case "on":
			if err := ag.EnableProxy(); err != nil {
				log.Fatalf("enable proxy: %v", err)
			}
			fmt.Printf("[OK] system proxy enabled: 127.0.0.1:%d\n", *proxyPort)
		case "off":
			if err := ag.DisableProxy(); err != nil {
				log.Fatalf("disable proxy: %v", err)
			}
			fmt.Println("[OK] system proxy disabled")
		case "toggle":
			if err := ag.ToggleProxy(); err != nil {
				log.Fatalf("toggle proxy: %v", err)
			}
			status := ag.ProxyStatus()
			if status.Enabled {
				fmt.Printf("[OK] proxy ON: %s\n", status.ProxyServer)
			} else {
				fmt.Println("[OK] proxy OFF")
			}
		case "status":
			status := ag.ProxyStatus()
			if status.Enabled {
				fmt.Printf("Proxy: ON (%s)\n", status.ProxyServer)
			} else {
				fmt.Println("Proxy: OFF")
			}
		}
	case "status":
		proxyStatus := ag.ProxyStatus()
		fmt.Printf("Agent Status:\n")
		fmt.Printf("  Proxy: %s\n", boolToStatus(proxyStatus.Enabled))
		if proxyStatus.Enabled {
			fmt.Printf("  Server: %s\n", proxyStatus.ProxyServer)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		usage()
		os.Exit(1)
	}
}

func boolToStatus(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func usage() {
	fmt.Fprintf(os.Stderr, `navo-agent - Navo User Agent

Usage:
  navo-agent run                  Run the agent (standalone mode)
  navo-agent proxy <on|off|toggle|status>  Manage system proxy
  navo-agent status               Show agent status

Options:
  --pipe NAME          Service pipe name (default: Navo.Agent.Service.v1)
  --ui-pipe NAME       UI pipe name (default: Navo.UI.Agent.v1)
  --proxy-port PORT    Local proxy port (default: 12080)
`)
}
