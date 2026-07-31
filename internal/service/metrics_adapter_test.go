package service

import (
	"os"
	"path/filepath"
	"testing"

	"navo/internal/domain/core"
)

func TestMetricsRuntimeFromSingBoxConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"experimental":{"clash_api":{"external_controller":"127.0.0.1:13080","secret":"token"}}}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := metricsRuntimeFromConfig(core.TypeSingBox, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ControllerURL != "http://127.0.0.1:13080" || got.ControllerSecret != "token" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestMetricsRuntimeRejectsNonLoopback(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("external-controller: 0.0.0.0:9090\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := metricsRuntimeFromConfig(core.TypeMihomo, path); err == nil {
		t.Fatal("expected non-loopback controller rejection")
	}
}
