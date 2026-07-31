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
			{ID: "node", Name: "Node", Type: OutboundVLESS, Server: "example.com", Port: 443, UUID: "id", TLS: true, SNI: "example.com", Enabled: true},
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
			AutoRoute: true, StrictRoute: true,
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
	tunConfig, _ := decoded["tun"].(map[string]any)
	if tunConfig["auto-detect-interface"] != true {
		t.Fatalf("missing physical-interface detection: %#v", tunConfig)
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
