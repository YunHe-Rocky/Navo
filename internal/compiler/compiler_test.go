package compiler

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTestConfig() *Config {
	return &Config{
		SchemaVersion: 1,
		Log: LogConfig{
			Level:     "warn",
			Timestamp: true,
		},
		Inbounds: []InboundConfig{
			{
				Type:       "mixed",
				Tag:        "mixed-in",
				Listen:     "127.0.0.1",
				ListenPort: 12080,
			},
		},
		Outbounds: []Outbound{
			{
				ID:      "direct",
				Name:    "Direct",
				Type:    OutboundDirect,
				Enabled: true,
			},
			{
				ID:       "us-socks5",
				Name:     "US SOCKS5",
				Type:     OutboundSOCKS,
				Server:   "192.168.1.100",
				Port:     1080,
				Username: "user",
				Password: "pass",
				Enabled:  true,
			},
		},
		RoutingRules: []RoutingRule{
			{
				ID:         "rule-openai",
				Name:       "OpenAI to US",
				Priority:   1,
				RuleType:   RuleDomainSuffix,
				Values:     []string{"openai.com", "chatgpt.com"},
				OutboundID: "us-socks5",
				Enabled:    true,
			},
		},
	}
}

func TestGenerate_DirectOnly(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "error", Timestamp: true},
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
	}

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check log
	log, ok := result["log"].(map[string]interface{})
	if !ok {
		t.Fatal("log section missing")
	}
	if log["level"] != "error" {
		t.Errorf("log.level = %v, want error", log["level"])
	}

	// Check inbounds
	inbounds, ok := result["inbounds"].([]interface{})
	if !ok || len(inbounds) != 1 {
		t.Fatal("inbounds missing or wrong count")
	}

	// Check outbounds
	outbounds, ok := result["outbounds"].([]interface{})
	if !ok || len(outbounds) < 1 {
		t.Fatal("outbounds missing")
	}
}

func TestGenerate_FinalOutbound(t *testing.T) {
	cfg := makeTestConfig()
	cfg.FinalOutbound = "us-socks5"

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	route := result["route"].(map[string]interface{})
	if route["final"] != "us-socks5" {
		t.Fatalf("route.final = %v, want us-socks5", route["final"])
	}
}

func TestGenerate_WithSOCKS5Outbound(t *testing.T) {
	cfg := makeTestConfig()

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	outbounds := result["outbounds"].([]interface{})
	if len(outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(outbounds))
	}

	// Second outbound should be SOCKS
	socks := outbounds[1].(map[string]interface{})
	if socks["type"] != "socks" {
		t.Errorf("type = %v, want socks", socks["type"])
	}
	if socks["server"] != "192.168.1.100" {
		t.Errorf("server = %v", socks["server"])
	}
}

func TestGenerate_WithRoutingRules(t *testing.T) {
	cfg := makeTestConfig()
	ResolveOutboundTags(cfg)

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	route := result["route"].(map[string]interface{})
	rules := route["rules"].([]interface{})
	if len(rules) != 1 {
		t.Fatalf("expected 1 route rule, got %d", len(rules))
	}

	rule := rules[0].(map[string]interface{})
	if rule["outbound"] != "us-socks5" {
		t.Errorf("rule outbound = %v, want us-socks5", rule["outbound"])
	}

	suffixes, ok := rule["domain_suffix"].([]interface{})
	if !ok {
		t.Fatal("domain_suffix missing")
	}
	if len(suffixes) != 2 {
		t.Errorf("expected 2 domain suffixes, got %d", len(suffixes))
	}
}

func TestGenerate_DNS(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "warn"},
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
		DNS: &DNSConfig{
			Enabled:  true,
			Strategy: DNSStrategyPreferIPv4,
			Servers: []DNSServer{
				{Tag: "cloudflare", Address: "tls://1.1.1.1"},
			},
		},
	}

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	dns, ok := result["dns"].(map[string]interface{})
	if !ok {
		t.Fatal("dns section missing")
	}
	if dns["strategy"] != "prefer_ipv4" {
		t.Errorf("strategy = %v, want prefer_ipv4", dns["strategy"])
	}
}

func TestValidateAcceptsModernUDPDNS(t *testing.T) {
	cfg := makeTestConfig()
	cfg.DNS = &DNSConfig{
		Enabled: true,
		Servers: []DNSServer{{
			Type: "udp", Tag: "dns-direct", Server: "223.5.5.5", ServerPort: 53,
		}},
		Final: "dns-direct",
	}
	if result := Validate(cfg); !result.Valid {
		t.Fatalf("modern UDP DNS rejected: %#v", result.Errors)
	}
}

func TestSingBoxBindsFrozenPhysicalInterface(t *testing.T) {
	cfg := makeTestConfig()
	cfg.OutboundInterface = "WLAN 2"
	data, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var generated map[string]any
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	route := generated["route"].(map[string]any)
	if route["default_interface"] != "WLAN 2" || route["auto_detect_interface"] != false {
		t.Fatalf("route interface binding = %#v", route)
	}
}

func TestValidateRejectsUnsupportedOutbound(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Outbounds: []Outbound{{
			ID: "bad", Type: OutboundType("unsupported"),
			Server: "example.com", Port: 443,
		}},
	}
	if result := Validate(cfg); result.Valid {
		t.Fatal("unsupported outbound type passed validation")
	}
}

func TestGenerateString(t *testing.T) {
	cfg := makeTestConfig()

	s, err := GenerateString(cfg)
	if err != nil {
		t.Fatalf("GenerateString() error: %v", err)
	}

	if !strings.Contains(s, `"type": "mixed"`) {
		t.Error("output missing expected content")
	}
}

func TestResolveOutboundTags(t *testing.T) {
	cfg := makeTestConfig()

	err := ResolveOutboundTags(cfg)
	if err != nil {
		t.Fatalf("ResolveOutboundTags() error: %v", err)
	}

	if cfg.RoutingRules[0].OutboundTag != "us-socks5" {
		t.Errorf("OutboundTag = %s, want us-socks5", cfg.RoutingRules[0].OutboundTag)
	}
}

func TestResolveOutboundTags_Direct(t *testing.T) {
	cfg := &Config{
		RoutingRules: []RoutingRule{
			{ID: "r1", OutboundID: "direct", Values: []string{"test.com"}},
		},
	}

	ResolveOutboundTags(cfg)

	if cfg.RoutingRules[0].OutboundTag != "direct" {
		t.Errorf("OutboundTag = %s, want direct", cfg.RoutingRules[0].OutboundTag)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := makeTestConfig()
	ResolveOutboundTags(cfg)

	vr := Validate(cfg)
	if !vr.Valid {
		t.Errorf("expected valid, got errors: %v", vr.Errors)
	}
}

func TestValidate_MissingInboundType(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Tag: "test", Listen: "127.0.0.1", ListenPort: 1080},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for missing inbound type")
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "test", Listen: "127.0.0.1", ListenPort: 99999},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for port > 65535")
	}
}

func TestValidate_MissingOutboundServer(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "o1", Type: OutboundSOCKS, Port: 1080},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for missing server on SOCKS outbound")
	}
}

func TestValidate_InvalidPortOnOutbound(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "o1", Type: OutboundSOCKS, Server: "1.2.3.4", Port: 0},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for port 0 on outbound")
	}
}

func TestValidate_RuleReferencesMissingOutbound(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
		RoutingRules: []RoutingRule{
			{ID: "r1", OutboundID: "nonexistent", Values: []string{"test.com"}},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for rule referencing missing outbound")
	}
}

func TestValidate_ShadowsocksMissingPassword(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "ss1", Type: OutboundShadowsocks, Server: "example.com", Port: 8388, Method: "aes-256-gcm"},
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for shadowsocks without password")
	}
}

func TestValidate_DirectOutboundOK(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
	}

	vr := Validate(cfg)
	if !vr.Valid {
		t.Errorf("expected valid for direct outbound, got: %v", vr.Errors)
	}
}

func TestValidate_DNSNoServers(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
		DNS: &DNSConfig{
			Enabled: true,
		},
	}

	vr := Validate(cfg)
	if vr.Valid {
		t.Error("expected invalid for DNS without servers")
	}
}

func TestValidateRejectsDuplicateAndDanglingReferences(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "duplicate", Listen: "127.0.0.1", ListenPort: 1080},
			{Type: "http", Tag: "duplicate", Listen: "127.0.0.1", ListenPort: 1081},
		},
		Outbounds: []Outbound{
			{ID: "same", Type: OutboundDirect}, {ID: "same", Type: OutboundDirect},
		},
		FinalOutbound: "missing",
		RoutingRules: []RoutingRule{
			{ID: "rule", RuleType: RulePort, Values: []string{"0-70000"}, OutboundID: "same"},
			{ID: "rule", RuleType: RuleDomainRegex, Values: []string{"["}, OutboundID: "same"},
		},
		DNS: &DNSConfig{Enabled: true, Final: "missing", Servers: []DNSServer{
			{Type: "udp", Tag: "duplicate", Server: "1.1.1.1", ServerPort: 53},
			{Type: "udp", Tag: "duplicate", Server: "8.8.8.8", ServerPort: 53},
		}},
	}
	if result := Validate(cfg); result.Valid || len(result.Errors) < 6 {
		t.Fatalf("invalid references passed validation: %#v", result.Errors)
	}
}

func TestValidateTransportAndTUNBoundaries(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Outbounds: []Outbound{{
			ID: "node", Type: OutboundTrojan, Server: "edge.example", Port: 443,
			Password: "secret", Network: "grpc",
		}},
		TUN: &TUNConfig{
			Enabled: true, InterfaceName: "Navo", MTU: 100,
			Address: []string{"not-a-cidr", "fd00::1/64"}, IPv6Enabled: false,
		},
	}
	if result := Validate(cfg); result.Valid || len(result.Errors) < 4 {
		t.Fatalf("invalid transport/TUN passed validation: %#v", result.Errors)
	}
}

func TestCompile_ValidConfig(t *testing.T) {
	c := NewDefaultCompiler("", "")
	cfg := makeTestConfig()

	result, err := c.Compile(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	if result.ConfigHash == "" {
		t.Error("ConfigHash is empty")
	}
	if len(result.JSON) == 0 {
		t.Error("JSON output is empty")
	}
}

func TestCompile_InvalidConfig(t *testing.T) {
	c := NewDefaultCompiler("", "")
	cfg := &Config{
		SchemaVersion: 99,
	}

	_, err := c.Compile(context.Background(), cfg)
	if err == nil {
		t.Error("Compile() expected error for invalid config")
	}
}

func TestApply(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewDefaultCompiler("", tmpDir)
	cfg := makeTestConfig()

	result, _ := c.Compile(context.Background(), cfg)

	rev, err := c.Apply(context.Background(), result)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if rev.Status != RevisionActive {
		t.Errorf("Status = %s, want %s", rev.Status, RevisionActive)
	}
	if rev.Version != 1 {
		t.Errorf("Version = %d, want 1", rev.Version)
	}

	// Check file exists
	if _, err := os.Stat(rev.ConfigPath); err != nil {
		t.Errorf("config file not found: %v", err)
	}

	// Check active revision
	active := c.GetActiveRevision()
	if active == nil || active.Version != 1 {
		t.Error("GetActiveRevision() not set")
	}
}

func TestApply_MultipleVersions(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewDefaultCompiler("", tmpDir)

	// Apply v1
	cfg1 := makeTestConfig()
	result1, _ := c.Compile(context.Background(), cfg1)
	c.Apply(context.Background(), result1)

	// Apply v2 (add another outbound)
	cfg2 := makeTestConfig()
	cfg2.Outbounds = append(cfg2.Outbounds, Outbound{
		ID: "jp-socks5", Name: "JP SOCKS5", Type: OutboundSOCKS, Server: "10.0.0.1", Port: 1080, Enabled: true,
	})
	result2, _ := c.Compile(context.Background(), cfg2)
	c.Apply(context.Background(), result2)

	// Check active is v2
	active := c.GetActiveRevision()
	if active.Version != 2 {
		t.Errorf("active version = %d, want 2", active.Version)
	}

	// Check revisions list
	revs := c.ListRevisions()
	if len(revs) != 2 {
		t.Fatalf("revisions count = %d, want 2", len(revs))
	}
	if revs[0].Version != 2 {
		t.Errorf("first revision version = %d, want 2", revs[0].Version)
	}
}

func TestRollback(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewDefaultCompiler("", tmpDir)

	cfg1 := makeTestConfig()
	result1, _ := c.Compile(context.Background(), cfg1)
	c.Apply(context.Background(), result1)

	cfg2 := makeTestConfig()
	result2, _ := c.Compile(context.Background(), cfg2)
	c.Apply(context.Background(), result2)

	// Rollback to v1
	rev, err := c.Rollback(context.Background(), 1)
	if err != nil {
		t.Fatalf("Rollback() error: %v", err)
	}

	if rev.RollbackFrom != 1 {
		t.Errorf("RollbackFrom = %d, want 1", rev.RollbackFrom)
	}
	if rev.Version != 3 {
		t.Errorf("rollback version = %d, want 3", rev.Version)
	}
}

func TestRollback_InvalidVersion(t *testing.T) {
	c := NewDefaultCompiler("", t.TempDir())

	_, err := c.Rollback(context.Background(), 999)
	if err == nil {
		t.Error("Rollback() expected error for invalid version")
	}
}

func TestWriteTempConfig(t *testing.T) {
	cfg := makeTestConfig()
	result, _ := Generate(cfg)

	cr := &CompileResult{
		Config:     cfg,
		JSON:       result,
		ConfigHash: "test1234",
		CompiledAt: time.Now(),
	}

	path, err := WriteTempConfig(cr)
	if err != nil {
		t.Fatalf("WriteTempConfig() error: %v", err)
	}
	defer os.Remove(path)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("temp config not found: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Error("temp config is empty")
	}
}

func TestGenerate_AllOutboundTypes(t *testing.T) {
	types := []struct {
		name     string
		ot       OutboundType
		wantType string
	}{
		{"socks", OutboundSOCKS, "socks"},
		{"http", OutboundHTTP, "http"},
		{"shadowsocks", OutboundShadowsocks, "shadowsocks"},
		{"trojan", OutboundTrojan, "trojan"},
		{"vless", OutboundVLESS, "vless"},
		{"direct", OutboundDirect, "direct"},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				SchemaVersion: 1,
				Log:           LogConfig{Level: "warn"},
				Inbounds: []InboundConfig{
					{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
				},
				Outbounds: []Outbound{
					{
						ID: "test", Type: tt.ot, Enabled: true,
						Server: "example.com", Port: 443,
						Password: "test", Method: "auto", UUID: "uuid",
					},
				},
			}

			data, err := Generate(cfg)
			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			var result map[string]interface{}
			json.Unmarshal(data, &result)
			outbounds := result["outbounds"].([]interface{})
			o := outbounds[0].(map[string]interface{})
			if o["type"] != tt.wantType {
				t.Errorf("type = %v, want %s", o["type"], tt.wantType)
			}
		})
	}
}

func TestGenerate_CommonSubscriptionProtocolsPassSingBoxCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box test binary is not available")
	}

	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "warn"},
		Inbounds: []InboundConfig{{
			Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1",
			ListenPort: 12080, Sniff: true,
		}},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
			{
				ID: "vmess-ws", Type: OutboundVMess, Server: "example.com", Port: 443,
				UUID: "bf000d23-0752-40b4-affe-68f7707a9661", Security: "auto",
				Network: "ws", TransportPath: "/ws", TransportHost: "example.com",
				TLS: true, SNI: "example.com", Enabled: true,
			},
			{
				ID: "vless-reality", Type: OutboundVLESS, Server: "example.com", Port: 443,
				UUID: "bf000d23-0752-40b4-affe-68f7707a9661",
				TLS:  true, SNI: "example.com", Fingerprint: "chrome",
				RealityPublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
				RealityShortID:   "0123456789abcdef", Enabled: true,
			},
			{
				ID: "trojan-grpc", Type: OutboundTrojan, Server: "example.com", Port: 443,
				Password: "secret", Network: "grpc", ServiceName: "TunService",
				TLS: true, SNI: "example.com", Enabled: true,
			},
			{
				ID: "hy2", Type: OutboundHysteria2, Server: "example.com", Port: 443,
				Password: "secret", TLS: true, SNI: "example.com",
				ObfsType: "salamander", ObfsPassword: "obfs", Enabled: true,
			},
			{
				ID: "tuic", Type: OutboundTUIC, Server: "example.com", Port: 443,
				UUID: "bf000d23-0752-40b4-affe-68f7707a9661", Password: "secret",
				CongestionControl: "bbr", TLS: true, SNI: "example.com", Enabled: true,
			},
		},
		RoutingRules: []RoutingRule{{
			ID: "direct-private", RuleType: RuleIP,
			Values:     []string{"127.0.0.0/8"},
			OutboundID: "direct", OutboundTag: "direct", Enabled: true,
		}},
		FinalOutbound: "vmess-ws",
	}
	data, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %s: %v", output, err)
	}
}

func TestGenerate_TUNConfig(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "warn"},
		Inbounds: []InboundConfig{
			{Type: "tun", Tag: "tun-in"},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
		TUN: &TUNConfig{
			Enabled:     true,
			MTU:         1500,
			Address:     []string{"10.0.0.1/24"},
			AutoRoute:   true,
			IPv6Enabled: true,
		},
	}

	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)
	inbounds := result["inbounds"].([]interface{})
	tunIn := inbounds[0].(map[string]interface{})

	if tunIn["mtu"] != float64(1500) {
		t.Errorf("mtu = %v, want 1500", tunIn["mtu"])
	}
	if tunIn["stack"] != "mixed" {
		t.Errorf("stack = %v, want mixed", tunIn["stack"])
	}

	dns := result["dns"].(map[string]interface{})
	servers := dns["servers"].([]interface{})
	server := servers[0].(map[string]interface{})
	if server["type"] != "udp" || server["server"] != "223.5.5.5" {
		t.Fatalf("unexpected TUN DNS server: %#v", server)
	}
	if _, exists := server["detour"]; exists {
		t.Fatalf("direct UDP DNS must use its native dialer, got detour: %#v", server)
	}

	route := result["route"].(map[string]interface{})
	if route["default_domain_resolver"] != "dns-direct" {
		t.Fatalf("default_domain_resolver = %v", route["default_domain_resolver"])
	}
	rules := route["rules"].([]interface{})
	var hasDNSHijack, hasDirectICMP bool
	for _, item := range rules {
		rule := item.(map[string]interface{})
		if rule["action"] == "hijack-dns" {
			hasDNSHijack = true
		}
		networks, _ := rule["network"].([]interface{})
		if len(networks) == 1 && networks[0] == "icmp" &&
			rule["outbound"] == "direct" {
			hasDirectICMP = true
		}
	}
	if !hasDNSHijack || !hasDirectICMP {
		t.Fatalf("missing TUN DNS/ICMP rules: %#v", rules)
	}
}

func TestGenerate_TUNProxyRoutesDoHThroughSelectedOutbound(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "warn"},
		Inbounds:      []InboundConfig{{Type: "tun", Tag: "tun-in"}},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
			{ID: "proxy", Type: OutboundSOCKS, Server: "127.0.0.1", Port: 1080, Enabled: true},
		},
		TUN:           &TUNConfig{Enabled: true, MTU: 1500, Address: []string{"172.19.0.1/30"}},
		FinalOutbound: "proxy",
	}

	data, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	dns := result["dns"].(map[string]interface{})
	server := dns["servers"].([]interface{})[0].(map[string]interface{})
	if server["type"] != "https" || server["server"] != "1.1.1.1" || server["path"] != "/dns-query" {
		t.Fatalf("unexpected proxied DoH server: %#v", server)
	}
	if server["detour"] != "proxy" {
		t.Fatalf("DoH bypasses selected outbound: %#v", server)
	}
	tls := server["tls"].(map[string]interface{})
	if tls["enabled"] != true || tls["server_name"] != "cloudflare-dns.com" {
		t.Fatalf("DoH TLS identity is incomplete: %#v", tls)
	}
	if result["route"].(map[string]interface{})["default_domain_resolver"] != "dns-proxy" {
		t.Fatalf("unexpected default resolver: %#v", result["route"])
	}
}

func TestRouteRule_PortType(t *testing.T) {
	cfg := &Config{
		SchemaVersion: 1,
		Log:           LogConfig{Level: "warn"},
		Inbounds: []InboundConfig{
			{Type: "mixed", Tag: "in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
		},
		RoutingRules: []RoutingRule{
			{ID: "r1", RuleType: RulePort, Values: []string{"80", "443"}, OutboundID: "direct", Enabled: true},
		},
	}

	ResolveOutboundTags(cfg)
	data, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)
	route := result["route"].(map[string]interface{})
	rules := route["rules"].([]interface{})
	rule := rules[0].(map[string]interface{})

	ports, ok := rule["port"].([]interface{})
	if !ok || len(ports) != 2 {
		t.Fatal("port rule not correctly generated")
	}
}

func TestConfigRevision(t *testing.T) {
	now := time.Now()
	rev := &Revision{
		ID:         "rev-1",
		Version:    1,
		Status:     RevisionActive,
		ConfigHash: "abc123",
		ConfigPath: filepath.Join(t.TempDir(), "config.json"),
		CreatedAt:  now,
	}

	if rev.Status != RevisionActive {
		t.Errorf("Status = %s", rev.Status)
	}
	if rev.Version != 1 {
		t.Errorf("Version = %d", rev.Version)
	}
}
