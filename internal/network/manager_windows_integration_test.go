//go:build windows

package network

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"navo/internal/winprocess"
)

// TestElevatedTUNAcceptance is intentionally opt-in: it launches the packaged
// desktop and exercises the real adapter, route, NRPT, firewall and data plane.
// A formal acceptance run must set NAVO_RUN_ELEVATED_TUN_TESTS=1 and must not
// treat a skip as a pass.
func TestElevatedTUNAcceptance(t *testing.T) {
	if os.Getenv("NAVO_RUN_ELEVATED_TUN_TESTS") != "1" {
		t.Skip("set NAVO_RUN_ELEVATED_TUN_TESTS=1 for real Windows TUN acceptance")
	}
	if err := checkAdministrator(); err != nil {
		t.Fatalf("elevated TUN acceptance requires administrator privileges: %v", err)
	}
	root := filepath.Join("..", "..")
	script := filepath.Join(root, "scripts", "test-tun-elevated.ps1")
	packageRoot := os.Getenv("NAVO_TUN_TEST_PACKAGE_ROOT")
	if packageRoot == "" {
		packageRoot = filepath.Join(root, "release", "Navo")
	}
	core := os.Getenv("NAVO_TUN_TEST_CORE")
	if core == "" {
		core = "sing-box"
	}
	failurePoint := os.Getenv("NAVO_TUN_FAILURE_POINT")
	if failurePoint == "" {
		failurePoint = "none"
	}
	crashPoint := os.Getenv("NAVO_TUN_CRASH_POINT")
	if crashPoint == "" {
		crashPoint = "none"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-PackageRoot", packageRoot, "-Core", core, "-FailurePoint", failurePoint, "-CrashPoint", crashPoint)
	winprocess.ConfigureHidden(command)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real TUN acceptance failed: %v\n%s", err, output)
	}
}
