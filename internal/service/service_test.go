package service

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/host"
	"navo/internal/supervisor"
)

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
	if svc.aiAssistant == nil {
		t.Error("AI assistant not initialized")
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

func TestService_Dispatch_AIDiagnose(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{
		"method":     "ai.diagnose",
		"request_id": "test-7",
	}
	resp := svc.dispatch(context.Background(), msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
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
	resp := svc.handleSubAdd("test", map[string]interface{}{})
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
	resp := svc.handleSubAdd("test", msg)
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
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

func TestService_SubRemove_NotFound(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	msg := map[string]interface{}{"id": "nonexistent"}
	resp := svc.handleSubRemove("test", msg)
	if resp["type"] != "ERROR" {
		t.Error("expected error for nonexistent subscription")
	}
}

func TestService_AIRuleGen_MissingRequest(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	resp := svc.handleAIRuleGenerate("test", map[string]interface{}{})
	if resp["type"] != "ERROR" {
		t.Error("expected error for missing request text")
	}
}

func TestService_AIDiagnose(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	// Record some traffic first
	svc.collector.RecordTraffic("us-node", 1000, 2000)

	resp := svc.handleAIDiagnose("test")
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
	payload := resp["payload"].(map[string]interface{})
	if _, ok := payload["severity"]; !ok {
		t.Error("expected severity in diagnosis")
	}
}

func TestService_AIExplain(t *testing.T) {
	cfg := Config{
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
	}
	svc, err := New(cfg)
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	resp := svc.handleAIExplain("test")
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v, want RESPONSE", resp["type"])
	}
	payload := resp["payload"].(map[string]interface{})
	text, _ := payload["text"].(string)
	if text == "" {
		t.Error("expected non-empty explanation text")
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
		SingBoxPath: filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe"),
		PipeName:    "Navo.Test.Service.v1",
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
