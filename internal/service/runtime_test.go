package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"navo/internal/compiler"
	"navo/internal/credential"
	"navo/internal/network"
)

func TestPinSelectedOutboundPreservesTLSAndTransportIdentity(t *testing.T) {
	original := compiler.Outbound{ID: "node", Server: "proxy.example", SNI: "tls.example", TransportHost: "ws.example", TransportPath: "/socket", ServiceName: "grpc-service", RealityPublicKey: "public-key", RealityShortID: "short", Enabled: true}
	plan := network.TUNActivationPlan{SelectedOutboundID: "node", OriginalServerHost: "proxy.example", PinnedServerIP: "203.0.113.7"}
	pinned, err := pinSelectedOutbound([]compiler.Outbound{original}, plan)
	if err != nil {
		t.Fatal(err)
	}
	got := pinned[0]
	if got.Server != "203.0.113.7" || got.SNI != original.SNI || got.TransportHost != original.TransportHost || got.TransportPath != original.TransportPath || got.ServiceName != original.ServiceName || got.RealityPublicKey != original.RealityPublicKey || got.RealityShortID != original.RealityShortID {
		t.Fatalf("pinning changed protocol identity: %#v", got)
	}
	if original.Server != "proxy.example" {
		t.Fatal("pinning mutated the persisted outbound")
	}
}

func TestPinSelectedOutboundDerivesTLSNameBeforeReplacingHost(t *testing.T) {
	original := compiler.Outbound{
		ID: "node", Server: "proxy.example", TLS: true, Enabled: true,
	}
	pinned, err := pinSelectedOutbound([]compiler.Outbound{original}, network.TUNActivationPlan{
		SelectedOutboundID: "node",
		OriginalServerHost: "proxy.example",
		PinnedServerIP:     "203.0.113.7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pinned[0].Server != "203.0.113.7" || pinned[0].SNI != "proxy.example" {
		t.Fatalf("TLS identity was not preserved: %#v", pinned[0])
	}
}

func TestPreferredEndpointIPPrefersIPv4ForColdStartCompatibility(t *testing.T) {
	got := preferredEndpointIP([]net.IP{
		net.ParseIP("2001:db8::7"),
		net.ParseIP("203.0.113.7"),
	})
	if got != "203.0.113.7" {
		t.Fatalf("preferred IP = %q", got)
	}
}

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
		service.runtime.Mode != runtimeModeGlobal ||
		!service.runtime.RoutingModeConfigured {
		t.Fatalf("runtime state = %#v", service.runtime)
	}
	if service.runtime.RevisionStatus != "candidate" || service.runtime.LastKnownGood != "" {
		t.Fatalf("unverified stopped-core revision was committed: %#v", service.runtime)
	}
	if activeOutboundID(service.runtime) != "" ||
		candidateOutboundID(service.runtime) != "node-1" {
		t.Fatalf("candidate was exposed as active: %#v", service.runtime)
	}
	if err := service.commitHealthyRuntime(context.Background()); err != nil {
		t.Fatalf("commit healthy runtime: %v", err)
	}
	if activeOutboundID(service.runtime) != "node-1" ||
		candidateOutboundID(service.runtime) != "" {
		t.Fatalf("healthy candidate was not committed: %#v", service.runtime)
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
	state := []byte(`{"core_id":"mihomo","mode":"global","routing_mode_configured":true,"tun_name":"Navo","tun_mtu":1500}`)
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
	for _, mode := range []string{"bypass_mainland", "global", "direct"} {
		if !validRuntimeMode(mode) {
			t.Errorf("validRuntimeMode(%q) = false, want true", mode)
		}
	}
	for _, mode := range []string{"invalid", "", "proxy", "tun", "rule", "blacklist", "whitelist"} {
		if validRuntimeMode(mode) {
			t.Errorf("validRuntimeMode(%q) = true, want false", mode)
		}
	}
}

func TestLoadRuntimeState_Defaults(t *testing.T) {
	state := loadRuntimeState("")
	if state.Mode != runtimeModeBypassMainland {
		t.Errorf("default mode = %q, want bypass_mainland", state.Mode)
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
	if state.RoutingListMode != routingListModeOff {
		t.Fatalf("default list mode = %q, want off", state.RoutingListMode)
	}
	if len(state.BlacklistRules) == 0 || len(state.WhitelistRules) == 0 {
		t.Fatalf("default user rule lists were not initialized: %#v", state)
	}
}

func TestLoadRuntimeState_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	os.WriteFile(path, []byte(`{"mode":"invalid_mode"}`), 0600)
	state := loadRuntimeState(dir)
	if state.Mode != runtimeModeBypassMainland {
		t.Errorf("invalid mode should reset to bypass_mainland, got %q", state.Mode)
	}
}

func TestLoadRuntimeStateDiscardsUnownedAdapterName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	if err := os.WriteFile(path, []byte(`{"tun_name":"以太网","tun_mtu":1500}`), 0600); err != nil {
		t.Fatal(err)
	}

	state := loadRuntimeState(dir)
	if state.TUNName != network.OwnedTUNAdapterName {
		t.Fatalf("unowned persisted adapter survived: %q", state.TUNName)
	}
}

func TestNormalizeOwnedTUNNameRejectsPhysicalAdapters(t *testing.T) {
	for _, name := range []string{"Ethernet", "以太网", "Wi-Fi", "Local Area Connection"} {
		if _, err := normalizeOwnedTUNName(name); err == nil {
			t.Fatalf("physical adapter name %q was accepted", name)
		}
	}
	if name, err := normalizeOwnedTUNName("navo"); err != nil || name != network.OwnedTUNAdapterName {
		t.Fatalf("canonical adapter was not normalized: name=%q err=%v", name, err)
	}
}

func TestLoadRuntimeStateMigratesLegacyForcedGlobalMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	if err := os.WriteFile(path, []byte(`{"mode":"global"}`), 0600); err != nil {
		t.Fatal(err)
	}
	state := loadRuntimeState(dir)
	if state.Mode != runtimeModeBypassMainland || state.RoutingModeConfigured {
		t.Fatalf("legacy runtime state = %#v, want unconfigured bypass_mainland mode", state)
	}
}

func TestLoadRuntimeStateMigratesLegacyRoutesButLeavesListsInactive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	if err := os.WriteFile(path, []byte(`{"mode":"whitelist","routing_mode_configured":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	state := loadRuntimeState(dir)
	if state.Mode != runtimeModeGlobal || !state.RoutingModeConfigured || state.RoutingListMode != routingListModeOff {
		t.Fatalf("legacy whitelist state = %#v, want configured global route and whitelist list mode", state)
	}
	if err := os.WriteFile(path, []byte(`{"mode":"blacklist","routing_mode_configured":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	state = loadRuntimeState(dir)
	if state.Mode != runtimeModeDirect || !state.RoutingModeConfigured || state.RoutingListMode != routingListModeOff {
		t.Fatalf("legacy blacklist state = %#v, want configured direct route and blacklist list mode", state)
	}
}

func TestLoadRuntimeStatePreservesExplicitRoutingRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_state.json")
	if err := os.WriteFile(path, []byte(`{
		"routing_rules_configured":true,
		"blacklist_rules":["*.Example.COM","203.0.113.7/24"],
		"whitelist_rules":[]
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	state := loadRuntimeState(dir)
	if !state.RoutingRulesConfigured {
		t.Fatal("explicit routing rules lost their configured marker")
	}
	if strings.Join(state.BlacklistRules, ",") != "example.com,203.0.113.0/24" {
		t.Fatalf("normalized blacklist = %#v", state.BlacklistRules)
	}
	if state.WhitelistRules == nil || len(state.WhitelistRules) != 0 {
		t.Fatalf("explicit empty whitelist = %#v", state.WhitelistRules)
	}
}

func TestNormalizeRoutingRuleEntries(t *testing.T) {
	got, err := normalizeRoutingRuleEntries([]string{
		" *.Example.COM ", "example.com", "203.0.113.7/24", "2001:db8::1/64",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "example.com,203.0.113.0/24,2001:db8::/64" {
		t.Fatalf("normalized rules = %#v", got)
	}
	for _, invalid := range []string{"https://example.com", "example", "bad_domain.example"} {
		if _, err := normalizeRoutingRuleEntries([]string{invalid}); err == nil {
			t.Fatalf("invalid rule %q was accepted", invalid)
		}
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
	resp := svc.handleOutboundSelect(context.Background(), "req-1", map[string]interface{}{})
	if resp["type"] != "ERROR" {
		t.Fatal("expected error for empty outbound id")
	}
}

func TestHandleRuntimeModeSet_InvalidMode(t *testing.T) {
	svc := &Service{}
	resp := svc.handleRuntimeModeSet(context.Background(), "req-1", map[string]interface{}{
		"mode": "invalid",
	})
	if resp["type"] != "ERROR" {
		t.Fatal("expected error for invalid mode")
	}
}

func TestHandleRuntimeRulesSetRejectsInvalidEntriesBeforeMutation(t *testing.T) {
	svc := &Service{runtime: runtimeState{
		BlacklistRules: []string{"existing.example"},
		WhitelistRules: []string{"direct.example"},
	}}
	resp := svc.handleRuntimeRulesSet(context.Background(), "req-rules", map[string]interface{}{
		"blacklist": []interface{}{"https://invalid.example/path"},
		"whitelist": []interface{}{"direct.example"},
	})
	if resp["type"] != "ERROR" {
		t.Fatalf("invalid routing rules were accepted: %#v", resp)
	}
	if strings.Join(svc.runtime.BlacklistRules, ",") != "existing.example" {
		t.Fatalf("invalid request mutated runtime rules: %#v", svc.runtime.BlacklistRules)
	}
}

func TestHandleRuntimeStatus(t *testing.T) {
	svc := &Service{runtime: runtimeState{
		Mode:             runtimeModeGlobal,
		SelectedOutbound: "node-1",
		TUNEnabled:       true,
		BlacklistRules:   []string{"blocked.example"},
		WhitelistRules:   []string{"direct.example"},
		RoutingListMode:  routingListModeBlacklist,
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
	if got := payload["blacklist"].([]string); len(got) != 1 || got[0] != "blocked.example" {
		t.Fatalf("blacklist = %#v", got)
	}
	if payload["list_mode"] != routingListModeBlacklist {
		t.Fatalf("list_mode = %v", payload["list_mode"])
	}
}

func TestHandleOutboundCreateUpstreamAppliesAndPersists(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	svc, err := New(Config{
		SingBoxPath:     binary,
		ConfigPath:      filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:       t.TempDir(),
		ProxyPort:       12080,
		CredentialStore: credential.NewMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := svc.handleOutboundCreate(context.Background(), "req-create", map[string]interface{}{
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

	resp := svc.handleOutboundCreate(context.Background(), "req-invalid", map[string]interface{}{
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
		Mode:             runtimeModeBypassMainland,
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
	if state.Mode != runtimeModeBypassMainland || state.SelectedOutbound != "direct" {
		t.Fatalf("state = %#v", state)
	}
}

func TestRuntimeListModeSetSameValueIsIdempotent(t *testing.T) {
	service := &Service{runtime: runtimeState{RoutingListMode: routingListModeOff}}
	result := service.handleRuntimeListModeSet(context.Background(), "list-mode-noop", map[string]interface{}{
		"mode": routingListModeOff,
	})
	if result["type"] == "ERROR" {
		t.Fatalf("idempotent list-mode response = %#v", result)
	}
	payload, ok := result["payload"].(map[string]interface{})
	if !ok || payload["mode"] != routingListModeOff || payload["changed"] != false || payload["verified"] != true {
		t.Fatalf("idempotent list-mode payload = %#v", result["payload"])
	}
}
func TestRuntimeRoutingPolicy(t *testing.T) {
	tests := []struct {
		mode      string
		wantFinal string
	}{
		{runtimeModeBypassMainland, "node-1"},
		{runtimeModeGlobal, "node-1"},
		{runtimeModeDirect, "direct"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			final, rules := runtimeRoutingPolicy(
				test.mode, routingListModeOff, "node-1", []string{"blocked.example"}, []string{"direct.example"},
			)
			if final != test.wantFinal {
				t.Fatalf("final = %q, want %q", final, test.wantFinal)
			}
			for _, rule := range rules {
				if rule.ID == "direct-whitelist" || rule.ID == "proxy-blacklist" {
					t.Fatalf("inactive lists leaked into route mode %s: %#v", test.mode, rules)
				}
			}
			if test.mode == runtimeModeBypassMainland && (len(rules) < 2 || rules[1].ID != "mainland-domains") {
				t.Fatalf("bypass-mainland base rule missing after list overrides: %#v", rules)
			}
		})
	}
}

func TestRuntimeRoutingPolicySplitsDomainAndCIDRRules(t *testing.T) {
	_, rules := runtimeRoutingPolicy(
		runtimeModeDirect,
		routingListModeBlacklist,
		"node-1",
		[]string{"blocked.example", "203.0.113.0/24"},
		nil,
	)
	var domainRule, ipRule *compiler.RoutingRule
	for index := range rules {
		switch rules[index].ID {
		case "proxy-blacklist":
			domainRule = &rules[index]
		case "proxy-blacklist-ip":
			ipRule = &rules[index]
		}
	}
	if domainRule == nil || domainRule.RuleType != compiler.RuleDomainSuffix || domainRule.Values[0] != "blocked.example" {
		t.Fatalf("domain rule = %#v", domainRule)
	}
	if ipRule == nil || ipRule.RuleType != compiler.RuleIP || ipRule.Values[0] != "203.0.113.0/24" {
		t.Fatalf("IP rule = %#v", ipRule)
	}
}

func TestRuntimeRoutingPolicyActivatesOnlySelectedListMode(t *testing.T) {
	for _, test := range []struct{ mode, want, reject string }{
		{routingListModeBlacklist, "proxy-blacklist", "direct-whitelist"},
		{routingListModeWhitelist, "direct-whitelist", "proxy-blacklist"},
	} {
		_, rules := runtimeRoutingPolicy(runtimeModeBypassMainland, test.mode, "node-1", []string{"same.example"}, []string{"same.example"})
		seen := map[string]bool{}
		for _, rule := range rules {
			seen[rule.ID] = true
		}
		if !seen[test.want] || seen[test.reject] {
			t.Fatalf("list mode %s leaked peer rules: %#v", test.mode, rules)
		}
	}
}

func TestRuntimeRoutingPolicySemanticMatrix(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		listMode      string
		wantFinal     string
		wantListRule  string
		wantListRoute string
		wantMainland  bool
	}{
		{"bypass/off", runtimeModeBypassMainland, routingListModeOff, "node-1", "", "", true},
		{"bypass/blacklist", runtimeModeBypassMainland, routingListModeBlacklist, "node-1", "proxy-blacklist", "node-1", true},
		{"bypass/whitelist", runtimeModeBypassMainland, routingListModeWhitelist, "node-1", "direct-whitelist", "direct", true},
		{"global/off", runtimeModeGlobal, routingListModeOff, "node-1", "", "", false},
		{"global/blacklist", runtimeModeGlobal, routingListModeBlacklist, "node-1", "proxy-blacklist", "node-1", false},
		{"global/whitelist", runtimeModeGlobal, routingListModeWhitelist, "node-1", "direct-whitelist", "direct", false},
		{"direct/off", runtimeModeDirect, routingListModeOff, "direct", "", "", false},
		{"direct/blacklist", runtimeModeDirect, routingListModeBlacklist, "direct", "proxy-blacklist", "node-1", false},
		{"direct/whitelist", runtimeModeDirect, routingListModeWhitelist, "direct", "direct-whitelist", "direct", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			final, rules := runtimeRoutingPolicy(
				test.mode, test.listMode, "node-1",
				[]string{"proxy.example"}, []string{"direct.example"},
			)
			if final != test.wantFinal {
				t.Fatalf("final = %q, want %q", final, test.wantFinal)
			}
			seen := make(map[string]compiler.RoutingRule, len(rules))
			for _, rule := range rules {
				seen[rule.ID] = rule
			}
			if private, ok := seen["private-networks"]; !ok || private.OutboundID != "direct" {
				t.Fatalf("private networks must always be direct: %#v", rules)
			}
			_, hasMainland := seen["mainland-domains"]
			if hasMainland != test.wantMainland {
				t.Fatalf("mainland rule present = %t, want %t: %#v", hasMainland, test.wantMainland, rules)
			}
			for _, id := range []string{"proxy-blacklist", "direct-whitelist"} {
				rule, present := seen[id]
				if id == test.wantListRule {
					if !present || rule.OutboundID != test.wantListRoute {
						t.Fatalf("list rule %q = %#v, want outbound %q", id, rule, test.wantListRoute)
					}
				} else if present {
					t.Fatalf("inactive list rule %q leaked into policy: %#v", id, rules)
				}
			}
		})
	}
}

func TestRuntimeRoutingPolicyDoesNotInventProxyRuleWithoutProxySelection(t *testing.T) {
	final, rules := runtimeRoutingPolicy(
		runtimeModeDirect, routingListModeBlacklist, "direct",
		[]string{"proxy.example"}, nil,
	)
	if final != "direct" {
		t.Fatalf("final = %q", final)
	}
	for _, rule := range rules {
		if rule.ID == "proxy-blacklist" || rule.OutboundID != "direct" {
			t.Fatalf("direct-only policy invented a proxy route: %#v", rules)
		}
	}
}
func TestProxiedRuntimeDNSUsesSelectedOutbound(t *testing.T) {
	dns := proxiedRuntimeDNS("node-1")
	if dns.Final != "dns-proxy" || len(dns.Servers) != 1 || dns.Servers[0].Detour != "node-1" {
		t.Fatalf("proxied DNS = %#v", dns)
	}
}

func TestCandidateSelectionPreservesActiveAndRoutingPolicy(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}
	service, err := New(Config{
		SingBoxPath: binary,
		ConfigPath:  filepath.Join("..", "..", "configs", "test_direct.json"),
		ConfigDir:   t.TempDir(),
		ProxyPort:   12080,
	})
	if err != nil {
		t.Fatal(err)
	}
	outbounds := []compiler.Outbound{
		{
			ID: "old-node", Name: "Old", Type: compiler.OutboundShadowsocks,
			Server: "old.example", Port: 8388, Method: "aes-128-gcm", Password: "old-secret", Enabled: true,
		},
		{
			ID: "new-node", Name: "New", Type: compiler.OutboundShadowsocks,
			Server: "new.example", Port: 8389, Method: "aes-128-gcm", Password: "new-secret", Enabled: true,
		},
	}
	if err := service.applyRuntimeConfig(
		context.Background(), outbounds, "old-node", runtimeModeGlobal,
	); err != nil {
		t.Fatal(err)
	}
	if err := service.commitHealthyRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	service.runtimeMu.Lock()
	service.runtime.RoutingListMode = routingListModeWhitelist
	service.runtime.RoutingRulesConfigured = true
	service.runtime.BlacklistRules = []string{"blocked.example"}
	service.runtime.WhitelistRules = []string{"direct.example"}
	service.runtimeMu.Unlock()

	if err := service.applyRuntimeConfig(
		context.Background(), outbounds, "new-node", "",
	); err != nil {
		t.Fatal(err)
	}
	service.runtimeMu.Lock()
	state := service.runtime
	service.runtimeMu.Unlock()
	if activeOutboundID(state) != "old-node" || candidateOutboundID(state) != "new-node" {
		t.Fatalf("active/candidate state = %#v", state)
	}
	if state.Mode != runtimeModeGlobal || state.RoutingListMode != routingListModeWhitelist {
		t.Fatalf("routing mode changed with candidate: %#v", state)
	}
	if !reflect.DeepEqual(state.BlacklistRules, []string{"blocked.example"}) ||
		!reflect.DeepEqual(state.WhitelistRules, []string{"direct.example"}) {
		t.Fatalf("routing rules changed with candidate: %#v", state)
	}
}
