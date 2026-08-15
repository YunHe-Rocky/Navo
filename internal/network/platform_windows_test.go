//go:build windows

package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("navo"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	const digest = "2e07e62ae87441c37a9ec6cf1b5be49587ee5ef92e4ce87539bf495607542df0"
	if err := verifyFileSHA256(path, digest); err != nil {
		t.Fatalf("verifyFileSHA256: %v", err)
	}
	if err := verifyFileSHA256(path, BundledWintunSHA256); err == nil {
		t.Fatal("verifyFileSHA256 accepted the wrong digest")
	}
}

func TestInspectAdapterSnapshotUsesBoundedNativeAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	_, err := InspectAdapterSnapshot(ctx, "Navo-test-adapter-that-does-not-exist")
	if err == nil {
		t.Fatal("non-existent adapter unexpectedly resolved")
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("native adapter inspection exhausted its deadline: %s", elapsed)
	}
	if strings.Contains(strings.ToLower(err.Error()), "powershell") {
		t.Fatalf("adapter readiness still depends on PowerShell: %v", err)
	}
}

func TestPhysicalRouteQueryHasValidPowerShellSyntax(t *testing.T) {
	route, err := (windowsEndpointResolver{executor: NewSystemExecutor()}).FindPhysicalRoute(context.Background(), "1.1.1.1", "Navo")
	if err != nil && (strings.Contains(err.Error(), "ParserError") || strings.Contains(err.Error(), "UnexpectedToken")) {
		t.Fatalf("physical route query has invalid PowerShell syntax: %v", err)
	}
	if err == nil && (route.InterfaceAlias == "" || route.NextHop == "") {
		t.Fatalf("physical route contains an empty candidate: %#v", route)
	}
}
