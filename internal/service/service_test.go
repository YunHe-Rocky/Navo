package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"navo/internal/compiler"
	"navo/internal/credential"
	"navo/internal/domain/capture"
	"navo/internal/host"
	"navo/internal/supervisor"
)

func TestDispatchRejectsExternalConfigPath(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath:     filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigDir:       t.TempDir(),
		CredentialStore: credential.NewMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := svc.Dispatch(context.Background(), map[string]interface{}{
		"request_id":  "untrusted-path",
		"method":      "core.start",
		"config_path": `C:\untrusted\config.json`,
	})
	if result["type"] != "ERROR" {
		t.Fatalf("external config path was accepted: %#v", result)
	}
	payload, _ := result["payload"].(map[string]interface{})
	if payload["code"] != "INVALID_ARGUMENT" {
		t.Fatalf("unexpected rejection: %#v", payload)
	}
}

func TestDispatchDoesNotExposeServiceShutdown(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath:     filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigDir:       t.TempDir(),
		CredentialStore: credential.NewMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := svc.Dispatch(context.Background(), map[string]interface{}{
		"request_id": "shutdown", "method": "service.shutdown",
	})
	if result["type"] != "ERROR" {
		t.Fatalf("service.shutdown unexpectedly exposed: %#v", result)
	}
}

func TestDispatchRejectsRequestIDReuseForDifferentRequest(t *testing.T) {
	svc := &Service{}
	requestID := "reused-request-id"

	svc.Dispatch(context.Background(), map[string]interface{}{
		"request_id": requestID,
		"method":     "unknown.first",
	})
	result := svc.Dispatch(context.Background(), map[string]interface{}{
		"request_id": requestID,
		"method":     "unknown.second",
	})

	payload, _ := result["payload"].(map[string]interface{})
	if result["type"] != "ERROR" || payload["code"] != "REQUEST_ID_REUSE" {
		t.Fatalf("request ID reuse was not rejected: %#v", result)
	}
}

func TestCoreSelectCancelsWhileConnectionTransactionIsBusy(t *testing.T) {
	svc := &Service{}
	svc.captureMu.Lock()
	defer svc.captureMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := svc.Dispatch(ctx, map[string]interface{}{
		"request_id": "busy-core-switch",
		"method":     "core.select",
		"core_id":    "mihomo",
	})
	payload, _ := result["payload"].(map[string]interface{})
	if result["type"] != "ERROR" || payload["code"] != "CORE_SWITCH_BUSY" {
		t.Fatalf("busy core switch response = %#v", result)
	}
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("busy core switch ignored caller cancellation for %v", elapsed)
	}
}

func TestDispatchRejectsPhysicalAdapterTargets(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath:     filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigDir:       t.TempDir(),
		CredentialStore: credential.NewMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"tun.enable", "tun.config"} {
		result := svc.Dispatch(context.Background(), map[string]interface{}{
			"request_id": "adapter-ownership-" + method, "method": method, "name": "以太网",
		})
		if result["type"] != "ERROR" {
			t.Fatalf("%s accepted a physical adapter target: %#v", method, result)
		}
		payload, _ := result["payload"].(map[string]interface{})
		if payload["code"] != "NET_ADAPTER_OWNERSHIP" {
			t.Fatalf("%s returned the wrong error: %#v", method, payload)
		}
	}
}

func TestTUNConfigRejectsLiveMutationButAllowsIdempotentReadback(t *testing.T) {
	svc := &Service{runtime: runtimeState{
		TUNEnabled: true,
		TUNName:    "Navo",
		TUNMTU:     1500,
	}}
	unchanged := svc.handleTUNConfig(context.Background(), "tun-config-same", map[string]interface{}{
		"name": "Navo", "mtu": float64(1500),
	})
	if unchanged["type"] != "RESPONSE" {
		t.Fatalf("idempotent TUN config = %#v", unchanged)
	}

	changed := svc.handleTUNConfig(context.Background(), "tun-config-change", map[string]interface{}{
		"name": "Navo", "mtu": float64(1400),
	})
	payload, _ := changed["payload"].(map[string]interface{})
	if changed["type"] != "ERROR" || payload["code"] != "TUN_RESTART_REQUIRED" {
		t.Fatalf("live TUN mutation = %#v", changed)
	}
}

func TestCombinedServiceBecomesReadyWithoutExternalPipe(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	svc, err := New(Config{
		SingBoxPath:     binary,
		ConfigPath:      filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:       t.TempDir(),
		CredentialStore: credential.NewMemoryStore(),
		DeferCoreStart:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()
	select {
	case <-svc.Ready():
		if svc.pipeListener != nil {
			t.Fatal("combined service exposed an external pipe")
		}
	case err := <-done:
		t.Fatalf("service exited before ready: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("service readiness timed out")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEndpointStatusTransitionsFromUntestedToHealthy(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	outbound := compiler.Outbound{
		ID: "node-1", Type: compiler.OutboundShadowsocks, Enabled: true,
	}
	if status := svc.endpointStatus(outbound); status.Color != "yellow" {
		t.Fatalf("untested color = %q, want yellow", status.Color)
	}

	svc.recordEndpointProbe(outbound.ID, true, "", 42*time.Millisecond)
	status := svc.endpointStatus(outbound)
	if !status.Available || status.Color != "green" || status.LatencyMS != 42 {
		t.Fatalf("healthy endpoint status = %#v", status)
	}
}

// TestService_New tests service creation with default config.
func TestService_New(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.subMgr == nil {
		t.Error("subscription manager not initialized")
	}
	if svc.collector == nil {
		t.Error("collector not initialized")
	}
	if svc.prober == nil {
		t.Error("prober not initialized")
	}
	if svc.ipDetector == nil {
		t.Error("IP detector not initialized")
	}
}

func TestService_New_EmptySingBoxPath(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty SingBoxPath")
	}
}

func TestService_New_DefaultPipeName(t *testing.T) {
	cfg := Config{
		SingBoxPath: "nonexistent.exe",
	}
	// Should fail because binary doesn't exist, not because of missing pipe name
	_, err := New(cfg)
	if err == nil {
		t.Skip("binary found somehow")
	}
	// Pipe name default is applied before binary check
}

func TestService_Status(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	status := svc.Status()
	if status.State == "" {
		t.Error("expected state in status")
	}
}

func TestService_Dispatch_CoreStatus(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "core.status",
		"request_id": "test-1",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
	payload, ok := resp["payload"].(map[string]interface{})
	if !ok {
		t.Fatal("payload is not a map")
	}
	if _, ok := payload["state"]; !ok {
		t.Error("expected state in payload")
	}
}

func TestCoreListReportsDetectedCaptureCapabilities(t *testing.T) {
	root := filepath.Join("..", "..", "third_party")
	svc, err := New(Config{
		SingBoxPath: filepath.Join(root, "sing-box", "sing-box.exe"),
		MihomoPath:  filepath.Join(root, "mihomo", "mihomo.exe"),
		XrayPath:    filepath.Join(root, "xray", "xray.exe"),
		ConfigDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	response := svc.handleCoreList("capabilities")
	payload := response["payload"].(map[string]interface{})
	items := payload["cores"].([]map[string]interface{})
	if len(items) != 3 {
		t.Fatalf("core count = %d, want 3", len(items))
	}
	byID := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		byID[item["id"].(string)] = item
		if item["installed"] != true {
			t.Fatalf("core %s was not detected as installed", item["id"])
		}
		if item["version"] == "" {
			t.Fatalf("core %s has no detected version", item["id"])
		}
	}
	if byID["sing-box"]["tun_supported"] != true || byID["mihomo"]["tun_supported"] != true {
		t.Fatal("sing-box and Mihomo must advertise TUN support")
	}
	if byID["xray"]["tun_supported"] != false {
		t.Fatal("Xray must explicitly advertise TUN as unsupported")
	}
}

func TestPrepareCaptureRejectsUnsupportedCoreBeforeMutation(t *testing.T) {
	root := filepath.Join("..", "..", "third_party")
	xrayPath := filepath.Join(root, "xray", "xray.exe")
	svc, err := New(Config{
		SingBoxPath: filepath.Join(root, "sing-box", "sing-box.exe"),
		XrayPath:    xrayPath,
		ConfigDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	xrayHost, err := newCoreHost(svc.cfg, "xray", svc.cfg.SingBoxPath)
	if err != nil {
		t.Fatal(err)
	}
	svc.host = xrayHost

	_, err = svc.prepareCaptureLocked(context.Background(), capture.ModeTUN)
	if err == nil || !strings.Contains(err.Error(), "does not support tun") {
		t.Fatalf("unsupported TUN error = %v", err)
	}
	if svc.sup.State() != supervisor.StateStopped {
		t.Fatalf("unsupported capture mutated core state to %s", svc.sup.State())
	}
}

func TestService_Dispatch_InvalidMethod(t *testing.T) {
	cfg := Config{
		SingBoxPath: "test.exe",
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("binary not found")
	}

	msg := map[string]interface{}{
		"method":     "nonexistent.method",
		"request_id": "test-2",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp["type"] != "ERROR" {
		t.Errorf("type = %v, want ERROR", resp["type"])
	}
}

func TestService_Dispatch_TUNStatus(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "tun.status",
		"request_id": "test-3",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
}

func TestService_Dispatch_SubList(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "subscription.list",
		"request_id": "test-4",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
}

func TestService_Dispatch_MetricsCurrent(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "metrics.current",
		"request_id": "test-5",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
}

func TestService_Dispatch_IPCheck(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "ip.check",
		"request_id": "test-6",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp["type"] == "RESPONSE" {
		payload := resp["payload"].(map[string]interface{})
		// New dual-IP format: "source" and "proxy" each have "ip", "country", etc.
		source, hasSource := payload["source"].(map[string]interface{})
		proxy, hasProxy := payload["proxy"].(map[string]interface{})
		if !hasSource || !hasProxy {
			t.Error("expected source and proxy in response")
		}
		if _, ok := source["ip"]; !ok {
			t.Error("expected source.ip in response")
		}
		if _, ok := proxy["ip"]; !ok {
			t.Error("expected proxy.ip in response")
		}
	}
	// May return ERROR if no network (expected in test environment)
}

func TestService_SubAdd_Validation(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	// Test missing fields
	resp := svc.handleSubAdd(context.Background(), "test", map[string]interface{}{})
	if resp["type"] != "ERROR" {
		t.Error("expected error for missing name/url")
	}
}

func TestService_SubAdd_Success(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"name":            "Test Subscription",
		"url":             "https://example.com/sub",
		"skip_tls_verify": true,
	}
	resp := svc.handleSubAdd(context.Background(), "test", msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
	addPayload := resp["payload"].(map[string]interface{})
	if addPayload["status"] != "added_pending_refresh" {
		t.Fatalf("add payload = %#v", addPayload)
	}

	// Verify it was added
	listResp := svc.handleSubList("test")
	payload := listResp["payload"].(map[string]interface{})
	subs := payload["subscriptions"].([]interface{})
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}
	sub := subs[0].(map[string]interface{})
	if sub["skip_tls_verify"] != true {
		t.Error("expected compatibility mode in subscription list")
	}

	updateResp := svc.handleSubUpdate("test-update", map[string]interface{}{
		"id":              sub["id"],
		"skip_tls_verify": false,
	})
	if updateResp["type"] != "RESPONSE" {
		t.Fatalf("update type = %v, want RESPONSE", updateResp["type"])
	}
	updatePayload := updateResp["payload"].(map[string]interface{})
	if updatePayload["skip_tls_verify"] != false {
		t.Error("expected compatibility mode to be disabled")
	}
}

func TestService_SubRefreshWaitCompletesBeforeResponse(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   t.TempDir(),
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := svc.handleSubRefresh(context.Background(), "test-wait", map[string]interface{}{
		"wait": true,
	})
	if resp["type"] != "RESPONSE" {
		t.Fatalf("response = %#v", resp)
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["status"] != "updated" || payload["node_count"] != 0 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestService_SubRefreshRequiresSynchronousAgentTransaction(t *testing.T) {
	svc, err := New(Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   t.TempDir(),
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := svc.handleSubRefresh(context.Background(), "test-no-wait", map[string]interface{}{})
	if resp["type"] != "ERROR" {
		t.Fatalf("response = %#v", resp)
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["code"] != "SUB_WAIT_REQUIRED" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestService_SubRemove_NotFound(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{"id": "nonexistent"}
	resp := svc.handleSubRemove(context.Background(), "test", msg)
	if resp["type"] != "ERROR" {
		t.Error("expected error for nonexistent subscription")
	}
}

func TestService_Stop(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	// Stop should not panic even if not running
	svc.Stop()
}

func TestService_StopIsConcurrentAndIdempotent(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}
	svc.running = true

	var callers sync.WaitGroup
	for i := 0; i < 32; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			svc.Stop()
		}()
	}
	callers.Wait()

	select {
	case <-svc.stopCh:
	default:
		t.Fatal("stop channel was not closed")
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}
	if cfg.PipeName != "" {
		t.Error("expected empty default PipeName")
	}
	if cfg.SingBoxPath != "" {
		t.Error("expected empty default SingBoxPath")
	}
}

func TestHostFindBinary(t *testing.T) {
	path, err := host.FindBinary()
	if err != nil {
		t.Logf("FindBinary returned error (expected if not installed): %v", err)
	} else {
		t.Logf("Found sing-box at: %s", path)
	}
}

func TestRunStandalone_Stop(t *testing.T) {
	cfg := Config{
		SingBoxPath:        filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		PipeName:           "Navo.Test.Service.v1",
		EnableExternalPipe: true,
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	svc.running = true
	go func() {
		time.Sleep(100 * time.Millisecond)
		svc.Stop()
	}()

	err = RunStandalone(svc)
	// May return error or nil depending on timing
	t.Logf("RunStandalone returned: %v", err)
}

func TestSupervisorIntegration(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	status := svc.sup.Status()
	if status.State != supervisor.StateStopped {
		t.Errorf("initial state = %s, want %s", status.State, supervisor.StateStopped)
	}
}

func TestNetworkObserveDispatchReturnsBoundedReadOnlySnapshot(t *testing.T) {
	svc := &Service{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	result := svc.Dispatch(ctx, map[string]interface{}{
		"request_id": "network-observe", "method": "network.observe",
	})
	if result["type"] != "RESPONSE" {
		t.Fatalf("network.observe response = %#v", result)
	}
	payload, ok := result["payload"].(map[string]interface{})
	if !ok || payload["environment"] == nil {
		t.Fatalf("network.observe payload = %#v", result["payload"])
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("network.observe ignored caller cancellation for %v", elapsed)
	}
}
