package agent

import (
	"context"
	"testing"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/ipdetect"
	"navo/internal/networkenv"
)

func TestNormalizeExternalSystemProxyURL(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"127.0.0.1:10808": "http://127.0.0.1:10808",
		"http=127.0.0.1:8080;https=127.0.0.1:10808": "http://127.0.0.1:10808",
		"socks=127.0.0.1:10809":                     "socks5://127.0.0.1:10809",
	}
	for input, expected := range tests {
		input, expected := input, expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			actual, err := normalizeExternalSystemProxyURL(input)
			if err != nil {
				t.Fatal(err)
			}
			if actual != expected {
				t.Fatalf("proxy URL = %q, want %q", actual, expected)
			}
		})
	}
	for _, invalid := range []string{"", "127.0.0.1", "http://user:secret@127.0.0.1:10808", "ftp://127.0.0.1:10808"} {
		if _, err := normalizeExternalSystemProxyURL(invalid); err == nil {
			t.Fatalf("proxy URL %q unexpectedly accepted", invalid)
		}
	}
}

func TestExternalSystemProxyIPCheckFeedsDashboardWithoutNavoCore(t *testing.T) {
	t.Parallel()

	serviceCalls := 0
	instance, err := New(Config{
		ProxyPort: 12080,
		SendToServiceContextFn: func(_ context.Context, message map[string]interface{}) (map[string]interface{}, error) {
			serviceCalls++
			method, _ := message["method"].(string)
			requestID, _ := message["request_id"].(string)
			payloads := map[string]map[string]interface{}{
				"core.status":    {"core_id": "sing-box", "state": "stopped"},
				"core.list":      {"cores": []map[string]interface{}{{"id": "sing-box"}}},
				"runtime.status": {"mode": "bypass_mainland", "selected_id": "candidate-v2", "candidate_id": "candidate-v2", "active_id": "", "exit_ip": ""},
				"tun.status":     {"installed": true, "enabled": false},
				"metrics.current": {
					"reachable": false, "available": false, "local_available": true,
				},
			}
			return agentResponse(requestID, payloads[method]), nil
		},
		ExternalIPCheckFn: func(_ context.Context, proxyURL string) (ipdetect.DualIPResult, error) {
			if proxyURL != "http://127.0.0.1:10808" {
				t.Fatalf("proxy URL = %q", proxyURL)
			}
			return ipdetect.DualIPResult{
				Source: &ipdetect.IPResult{OutboundID: "source", IP: "198.51.100.10", Provider: "direct-fixture", CheckedAt: time.Now()},
				Proxy:  &ipdetect.IPResult{OutboundID: "external-system-proxy", IP: "203.0.113.20", Country: "External", Provider: "proxy-fixture", CheckedAt: time.Now()},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.currentProxy = func() (systemproxy.ProxyConfig, error) {
		return systemproxy.ProxyConfig{Enabled: true, ProxyServer: "127.0.0.1:10808"}, nil
	}
	instance.environmentStore.Publish(networkenv.Snapshot{
		Version: networkenv.SnapshotVersion,
		SystemProxy: networkenv.SystemProxySnapshot{
			Enabled: true, ProxyServer: "127.0.0.1:10808", Ownership: networkenv.OwnerExternal,
		},
	})

	check := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "external-check",
		"method":     "ip.check",
	})
	if isErrorResponse(check) {
		t.Fatalf("external IP check failed: %#v", check)
	}
	checkPayload, _ := check["payload"].(map[string]interface{})
	if checkPayload["connection_kind"] != "external_system_proxy" {
		t.Fatalf("connection_kind = %#v", checkPayload["connection_kind"])
	}
	if serviceCalls != 0 {
		t.Fatalf("external IP check called Navo Service %d times", serviceCalls)
	}

	dashboard := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "dashboard-external",
		"method":     "dashboard.snapshot",
	})
	if isErrorResponse(dashboard) {
		t.Fatalf("dashboard failed: %#v", dashboard)
	}
	payload, _ := dashboard["payload"].(map[string]interface{})
	ip, _ := payload["ip"].(map[string]interface{})
	if ip["connection_kind"] != "external_system_proxy" {
		t.Fatalf("dashboard connection_kind = %#v", ip["connection_kind"])
	}
	if ip["proxy_ip"] != "203.0.113.20" || ip["direct_ip"] != "198.51.100.10" {
		t.Fatalf("dashboard IP evidence = %#v", ip)
	}
	if serviceCalls != 5 {
		t.Fatalf("dashboard Service calls = %d, want 5", serviceCalls)
	}
}
