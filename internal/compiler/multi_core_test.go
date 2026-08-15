package compiler

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateForCore(t *testing.T) {
	cfg := &Config{
		Log: LogConfig{Level: "info"},
		Inbounds: []InboundConfig{{
			Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080,
		}},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
			{ID: "node", Name: "Node", Type: OutboundVLESS, Server: "example.com", Port: 443, UUID: "bf000d23-0752-40b4-affe-68f7707a9661", TLS: true, SNI: "example.com", Enabled: true},
		},
		FinalOutbound: "node",
	}
	for _, coreID := range []string{CoreSingBox, CoreMihomo, CoreXray} {
		t.Run(coreID, func(t *testing.T) {
			data, err := GenerateForCore(coreID, cfg)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if coreID == CoreMihomo {
				if err := yaml.Unmarshal(data, &decoded); err != nil {
					t.Fatalf("generated configuration is not valid YAML: %v", err)
				}
			} else if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("generated configuration is not valid JSON: %v", err)
			}
		})
	}
}

func TestXrayRejectsUnsupportedOutbound(t *testing.T) {
	cfg := &Config{
		Outbounds: []Outbound{{
			ID: "hy2", Type: OutboundHysteria2, Server: "example.com", Port: 443,
		}},
	}
	if _, err := GenerateForCore(CoreXray, cfg); err == nil {
		t.Fatal("expected Xray compiler to reject Hysteria2")
	}
}

func TestMihomoTUNIncludesRoutingAndDNSGuards(t *testing.T) {
	cfg := &Config{
		Log: LogConfig{Level: "info"},
		TUN: &TUNConfig{
			Enabled: true, InterfaceName: "NAVO-TUN", MTU: 1500,
			Address:   []string{"172.19.0.1/30"},
			AutoRoute: false, StrictRoute: false,
			OutboundInterface: "Ethernet 2",
		},
	}
	data, err := GenerateMihomo(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, exists := decoded["tun"]; exists {
		t.Fatal("Mihomo top-level TUN can replace Navo's frozen address plan")
	}
	listeners, _ := decoded["listeners"].([]any)
	if len(listeners) != 1 {
		t.Fatalf("Mihomo TUN listener missing: %#v", decoded["listeners"])
	}
	tunConfig, _ := listeners[0].(map[string]any)
	if tunConfig["name"] != "NAVO-TUN" || tunConfig["device"] != "NAVO-TUN" || tunConfig["type"] != "tun" {
		t.Fatalf("Mihomo TUN listener identity mismatch: %#v", tunConfig)
	}
	if decoded["interface-name"] != "Ethernet 2" {
		t.Fatalf("Mihomo physical egress is not frozen: %#v", decoded)
	}
	if tunConfig["auto-detect-interface"] != true {
		t.Fatalf("missing physical-interface detection: %#v", tunConfig)
	}
	if tunConfig["auto-route"] != false || tunConfig["strict-route"] != false {
		t.Fatalf("Mihomo took ownership of Navo routes: %#v", tunConfig)
	}
	if _, ok := tunConfig["dns-hijack"]; !ok {
		t.Fatalf("missing DNS hijack: %#v", tunConfig)
	}
	addresses, ok := tunConfig["inet4-address"].([]any)
	if !ok || len(addresses) != 1 || addresses[0] != "172.19.0.1/30" {
		t.Fatalf("Mihomo TUN address is not aligned with the route manager: %#v", tunConfig)
	}
	dnsConfig, _ := decoded["dns"].(map[string]any)
	if dnsConfig["enable"] != true {
		t.Fatalf("missing enabled DNS module: %#v", dnsConfig)
	}
	nameservers, _ := dnsConfig["nameserver"].([]any)
	if len(nameservers) != 1 || nameservers[0] != "dhcp://system" {
		t.Fatalf("Mihomo DNS is not bound to frozen physical egress: %#v", dnsConfig)
	}
}

func TestMihomoTUNProxyRoutesDoHThroughSelectedGroup(t *testing.T) {
	cfg := &Config{
		Log: LogConfig{Level: "info"},
		TUN: &TUNConfig{
			Enabled: true, InterfaceName: "Navo", MTU: 1500,
			Address: []string{"172.19.0.1/30"}, OutboundInterface: "Ethernet",
		},
		Outbounds: []Outbound{
			{ID: "direct", Type: OutboundDirect, Enabled: true},
			{ID: "proxy", Type: OutboundSOCKS, Server: "127.0.0.1", Port: 1080, Enabled: true},
		},
		FinalOutbound: "proxy",
	}
	data, err := GenerateMihomo(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	dns := decoded["dns"].(map[string]any)
	nameservers := dns["nameserver"].([]any)
	if len(nameservers) != 1 || nameservers[0] != "https://1.1.1.1/dns-query#NAVO" {
		t.Fatalf("Mihomo target DNS bypasses selected group: %#v", dns)
	}
	bootstrap := dns["proxy-server-nameserver"].([]any)
	if len(bootstrap) != 1 || bootstrap[0] != "dhcp://system" {
		t.Fatalf("Mihomo bootstrap DNS is missing: %#v", dns)
	}
}

func TestXrayRejectsTUNUntilVersionedAdapterIsImplemented(t *testing.T) {
	cfg := &Config{
		Log: LogConfig{Level: "info"},
		TUN: &TUNConfig{Enabled: true},
	}
	if _, err := GenerateXray(cfg); err == nil {
		t.Fatal("Xray silently accepted an unsupported TUN configuration")
	}
}

func TestWireGuardIsRejectedUntilCanonicalFieldsArePreserved(t *testing.T) {
	outbound := Outbound{ID: "wg", Type: OutboundWireGuard, Server: "198.51.100.1", Port: 51820}
	for _, coreID := range []string{CoreSingBox, CoreMihomo, CoreXray} {
		if Compatible(coreID, outbound) {
			t.Fatalf("%s advertised incomplete WireGuard support", coreID)
		}
	}
	if ValidateOutbound(&outbound).Valid {
		t.Fatal("validator accepted incomplete WireGuard model")
	}
	if _, err := GenerateMihomo(&Config{Outbounds: []Outbound{outbound}}); err == nil {
		t.Fatal("Mihomo compiler accepted incomplete WireGuard")
	}
}

func TestMihomoGRPCDoesNotGenerateWebSocketOptions(t *testing.T) {
	data, err := GenerateMihomo(&Config{Outbounds: []Outbound{{
		ID: "grpc", Type: OutboundTrojan, Server: "edge.example", Port: 443,
		Password: "secret", Network: "grpc", ServiceName: "Tunnel",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	proxy := decoded["proxies"].([]any)[0].(map[string]any)
	if _, exists := proxy["ws-opts"]; exists {
		t.Fatalf("gRPC was emitted as WebSocket: %#v", proxy)
	}
	if _, exists := proxy["grpc-opts"]; !exists {
		t.Fatalf("gRPC options missing: %#v", proxy)
	}
}

func TestXrayCompilesCanonicalRoutingRules(t *testing.T) {
	data, err := GenerateXray(&Config{
		Outbounds: []Outbound{{ID: "direct", Type: OutboundDirect}},
		RoutingRules: []RoutingRule{{
			ID: "domains", RuleType: RuleDomainSuffix, Values: []string{"example.com"},
			OutboundID: "direct", Enabled: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	routing := decoded["routing"].(map[string]any)
	rules := routing["rules"].([]any)
	first := rules[0].(map[string]any)
	if first["outboundTag"] != "direct" || first["domain"].([]any)[0] != "domain:example.com" {
		t.Fatalf("canonical rule was ignored: %#v", first)
	}
}
