//go:build windows

package systemproxy

import "testing"

func TestWinINetChatGPTProbeContract(t *testing.T) {
	if len(winINetChatGPTProbes) != 5 {
		t.Fatalf("ChatGPT probe count = %d", len(winINetChatGPTProbes))
	}
	expected := map[string]bool{
		"chatgpt-web": true, "chatgpt-auth": true, "openai-api": true,
		"chatgpt-assets": true, "chatgpt-stream": true,
	}
	for _, probe := range winINetChatGPTProbes {
		if !expected[probe.name] || probe.endpoint == "" || len(probe.expected) == 0 {
			t.Fatalf("invalid ChatGPT probe = %#v", probe)
		}
	}
}

func TestWinINetRoutingProbeContract(t *testing.T) {
	if len(winINetDirectConnectivityEndpoints) != 2 ||
		winINetDirectConnectivityEndpoints[0] != "https://www.baidu.com/" ||
		winINetDirectConnectivityEndpoints[1] != "https://connect.rom.miui.com/generate_204" {
		t.Fatalf("direct routing endpoints = %#v", winINetDirectConnectivityEndpoints)
	}
	if len(winINetProxyConnectivityEndpoints) != 3 {
		t.Fatalf("proxy connectivity endpoints = %#v", winINetProxyConnectivityEndpoints)
	}
	if winINetApplicationProbeAttempts != 2 {
		t.Fatalf("application probe attempts = %d", winINetApplicationProbeAttempts)
	}
}

func TestWinINetStatusAccepted(t *testing.T) {
	if !winINetStatusAccepted(401, []uint32{401}) {
		t.Fatal("expected API unauthorized readiness status was rejected")
	}
	if !winINetStatusAccepted(403, []uint32{200, 403}) {
		t.Fatal("expected edge forbidden readiness status was rejected")
	}
	if winINetStatusAccepted(500, []uint32{200, 403}) {
		t.Fatal("server failure was accepted as readiness evidence")
	}
}

func TestValidateWinINetExitIdentity(t *testing.T) {
	if err := validateWinINetExitIdentity("203.0.113.20", "198.51.100.10", true); err != nil {
		t.Fatalf("distinct proxy exit rejected: %v", err)
	}
	if err := validateWinINetExitIdentity("198.51.100.10", "198.51.100.10", true); err == nil {
		t.Fatal("direct WinINet leak was accepted as proxy")
	}
	if err := validateWinINetExitIdentity("2001:db8::1", "2001:0db8:0:0:0:0:0:1", false); err != nil {
		t.Fatalf("equivalent intentional-direct IPv6 exits rejected: %v", err)
	}
	if err := validateWinINetExitIdentity("203.0.113.20", "198.51.100.10", false); err == nil {
		t.Fatal("changed intentional-direct exit was accepted")
	}
}
