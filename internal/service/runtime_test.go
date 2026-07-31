package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"navo/internal/compiler"
)

func TestApplyRuntimeConfigPersistsSelectionAndMode(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	configDir := t.TempDir()
	service, err := New(Config{
		SingBoxPath: binary,
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   configDir,
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}
	outbounds := []compiler.Outbound{{
		ID: "node-1", Name: "Node 1", Type: compiler.OutboundShadowsocks,
		Server: "example.com", Port: 8388, Method: "aes-128-gcm",
		Password: "secret", Enabled: true,
	}}

	if err := service.applyRuntimeConfig(
		context.Background(),
		outbounds,
		"node-1",
		runtimeModeGlobal,
	); err != nil {
		t.Fatal(err)
	}

	if service.runtime.SelectedOutbound != "node-1" ||
		service.runtime.Mode != runtimeModeGlobal {
		t.Fatalf("runtime state = %#v", service.runtime)
	}
	data, err := os.ReadFile(service.cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var generated map[string]interface{}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	route := generated["route"].(map[string]interface{})
	if route["final"] != "node-1" {
		t.Fatalf("route.final = %v", route["final"])
	}

	if err := service.applyRuntimeConfig(
		context.Background(),
		outbounds,
		"",
		runtimeModeDirect,
	); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(service.cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	route = generated["route"].(map[string]interface{})
	if route["final"] != "direct" {
		t.Fatalf("direct route.final = %v", route["final"])
	}

	if err := service.setTUNRuntime(
		context.Background(),
		true,
		"Navo",
		1500,
	); err != nil {
		t.Fatalf("TUN runtime validation failed: %v", err)
	}

	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatal(err)
	}
	runtimeFiles := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "runtime.") &&
			strings.HasSuffix(entry.Name(), ".json") &&
			entry.Name() != "runtime_state.json" {
			runtimeFiles++
		}
	}
	if runtimeFiles != 1 {
		t.Fatalf("generated runtime files = %d, want 1", runtimeFiles)
	}
}

func TestApplyRuntimeConfigRejectsMissingSelection(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	service, err := New(Config{
		SingBoxPath: binary,
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = service.applyRuntimeConfig(
		context.Background(),
		nil,
		"missing",
		runtimeModeGlobal,
	)
	if err == nil {
		t.Fatal("expected missing outbound selection error")
	}
}

func TestPrepareRuntimeConfigUsesPersistedMihomoCoreWithoutOutbounds(t *testing.T) {
	configDir := t.TempDir()
	state := []byte(`{"core_id":"mihomo","mode":"global","tun_name":"Navo","tun_mtu":1500}`)
	if err := os.WriteFile(filepath.Join(configDir, "runtime_state.json"), state, 0600); err != nil {
		t.Fatal(err)
	}

	service, err := New(Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		MihomoPath:  filepath.Join("..", "..", "third_party", "mihomo", "mihomo.exe"),
		ConfigPath:  filepath.Join(configDir, "runtime.json"),
		ConfigDir:   configDir,
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.prepareRuntimeConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.runtime.CoreID != "mihomo" {
		t.Fatalf("core = %q, want mihomo", service.runtime.CoreID)
	}
	if service.cfg.ConfigPath == filepath.Join(configDir, "runtime.json") {
		t.Fatal("persisted core reused the sing-box bootstrap config")
	}

	data, err := os.ReadFile(service.cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var generated map[string]interface{}
	if err := yaml.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	if generated["mixed-port"] != 12080 {
		t.Fatalf("mihomo mixed-port = %v, want 12080", generated["mixed-port"])
	}
}

func TestValidRuntimeMode(t *testing.T) {
	for _, mode := range []string{"rule", "global", "direct"} {
		if !validRuntimeMode(mode) {
			t.Errorf("validRuntimeMode(%q) = false, want true", mode)
		}
	}
	for _, mode := range []string{"invalid", "", "proxy", "tun"} {
		if validRuntimeMode(mode) {
			t.Errorf("validRuntimeMode(%q) = true, want false", mode)
		}
	}
}

func TestLoadRuntimeState_Defaults(t *testing.T) {
	state := loadRuntimeState("")
	if state.Mode != runtimeModeGlobal {
		t.Errorf("default mode = %q, want global", state.Mode)
	}
	if state.TUNName != "Navo" {
		t.Errorf("default TUNName = %q", state.TUNName)
	}
	if state.TUNMTU != 1500 {
		t.Errorf("default TUNMTU = %d", state.TUNMTU)
	}
	if state.TUNEnabled {
		t.Error("TUNEnabled should default to false")
	}
}

func TestLoadRuntimeState_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	os.WriteFile(path, []byte(`{"mode":"invalid_mode"}`), 0600)
	state := loadRuntimeState(dir)
	if state.Mode != runtimeModeGlobal {
		t.Errorf("invalid mode should reset to global, got %q", state.Mode)
	}
}

func TestLoadRuntimeState_ZeroMTU(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	os.WriteFile(path, []byte(`{"mode":"global","tun_mtu":0}`), 0600)
	state := loadRuntimeState(dir)
	if state.TUNMTU != 1500 {
		t.Errorf("zero MTU should reset to 1500, got %d", state.TUNMTU)
	}
}

func TestHandleOutboundSelect_EmptyID(t *testing.T) {
	svc := &Service{}
	resp := svc.handleOutboundSelect("req-1", map[string]interface{}{})
	if resp["type"] != "ERROR" {
		t.Fatal("expected error for empty outbound id")
	}
}

func TestHandleRuntimeModeSet_InvalidMode(t *testing.T) {
	svc := &Service{}
	resp := svc.handleRuntimeModeSet("req-1", map[string]interface{}{
		"mode": "invalid",
	})
	if resp["type"] != "ERROR" {
		t.Fatal("expected error for invalid mode")
	}
}

func TestHandleRuntimeStatus(t *testing.T) {
	svc := &Service{runtime: runtimeState{
		Mode:             runtimeModeGlobal,
		SelectedOutbound: "node-1",
		TUNEnabled:       true,
	}}
	resp := svc.handleRuntimeStatus("req-1")
	if resp["type"] != "RESPONSE" {
		t.Fatal("expected RESPONSE")
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["mode"] != runtimeModeGlobal {
		t.Errorf("mode = %v", payload["mode"])
	}
	if payload["active_id"] != "node-1" {
		t.Errorf("active_id = %v", payload["active_id"])
	}
}

func TestHandleOutboundCreateUpstreamAppliesAndPersists(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	svc, err := New(Config{
		SingBoxPath: binary,
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   t.TempDir(),
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := svc.handleOutboundCreate("req-create", map[string]interface{}{
		"name":     "SOCKS Test",
		"proto":    "socks5",
		"server":   "example.com",
		"port":     float64(1080),
		"username": "test-user",
		"password": "test-password",
	})
	if resp["type"] != "RESPONSE" {
		t.Fatalf("response = %#v", resp)
	}
	outbounds := svc.currentOutbounds(context.Background())
	if len(outbounds) != 1 {
		t.Fatalf("outbounds = %d, want 1", len(outbounds))
	}
	if outbounds[0].Type != compiler.OutboundSOCKS || outbounds[0].ProviderID != "upstream_proxy" {
		t.Fatalf("unexpected outbound = %+v", outbounds[0])
	}
	if svc.runtime.SelectedOutbound != outbounds[0].ID {
		t.Fatalf("selected = %q, want %q", svc.runtime.SelectedOutbound, outbounds[0].ID)
	}
	stored := svc.upstreamMgr.List()
	if len(stored) != 1 || stored[0].PasswordRef == nil || strings.Contains(*stored[0].PasswordRef, "test-password") {
		t.Fatalf("upstream credentials were not replaced with references: %+v", stored)
	}
}

func TestHandleOutboundCreateRejectsInvalidBeforePersistence(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	svc, err := New(Config{
		SingBoxPath: binary,
		ConfigDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := svc.handleOutboundCreate("req-invalid", map[string]interface{}{
		"name":   "Invalid",
		"proto":  "unsupported",
		"server": "example.com",
		"port":   float64(443),
	})
	if resp["type"] != "ERROR" {
		t.Fatalf("response = %#v", resp)
	}
	if len(svc.upstreamMgr.List()) != 0 {
		t.Fatal("invalid outbound was persisted")
	}
}

func TestCleanupGeneratedRuntimeFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a few runtime files
	for _, name := range []string{
		"runtime.1.json", "runtime.2.json", "runtime.3.json",
		"runtime.active.json", "other.json",
	} {
		os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0600)
	}
	active := filepath.Join(dir, "runtime.2.json")
	cleanupGeneratedRuntimeFiles(dir, active)

	entries, _ := os.ReadDir(dir)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
		t.Logf("  %s", e.Name())
	}
	// runtime.2.json (active), other.json should remain
	for _, name := range names {
		if name == "runtime.active.json" {
			t.Error("runtime.active.json should have been cleaned up")
		}
		if name == "runtime.1.json" || name == "runtime.3.json" {
			t.Errorf("%s should have been cleaned up", name)
		}
	}
}

func TestSaveRuntimeStateLocked(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{runtime: runtimeState{
		Mode:             "rule",
		SelectedOutbound: "direct",
		TUNName:          "Navo",
		TUNMTU:           1420,
	}}
	if err := svc.saveRuntimeStateLocked(dir); err != nil {
		t.Fatalf("saveRuntimeStateLocked: %v", err)
	}

	// Verify the file exists and is valid JSON
	data, err := os.ReadFile(filepath.Join(dir, "runtime_state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state runtimeState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Mode != "rule" || state.SelectedOutbound != "direct" {
		t.Fatalf("state = %#v", state)
	}
}
