package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDashboardSnapshotUsesCachedStatus(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 5)
	serviceRequestIDs := make(map[string]struct{}, 5)
	instance, err := New(Config{
		ProxyPort: 12080,
		SendToServiceFn: func(message map[string]interface{}) (map[string]interface{}, error) {
			method, _ := message["method"].(string)
			calls = append(calls, method)
			requestID, _ := message["request_id"].(string)
			if requestID == "dashboard" {
				t.Fatalf("dashboard parent request ID leaked to Service")
			}
			if _, exists := serviceRequestIDs[requestID]; exists {
				t.Fatalf("dashboard reused Service request ID %q", requestID)
			}
			serviceRequestIDs[requestID] = struct{}{}
			payloads := map[string]map[string]interface{}{
				"core.status":    {"core_id": "sing-box", "state": "running"},
				"core.list":      {"cores": []map[string]interface{}{{"id": "sing-box"}}},
				"runtime.status": {"mode": "bypass_mainland", "exit_ip": "203.0.113.20", "exit_country": "Test"},
				"tun.status":     {"installed": true, "enabled": false},
				"metrics.current": {
					"reachable": true,
				},
			}
			return agentResponse(requestID, payloads[method]), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.lastIPProbe = time.Now()

	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "dashboard",
		"method":     "dashboard.snapshot",
	})
	if isErrorResponse(response) {
		t.Fatalf("snapshot failed: %#v", response)
	}
	if response["request_id"] != "dashboard" {
		t.Fatalf("snapshot response request_id = %q", response["request_id"])
	}
	payload, _ := response["payload"].(map[string]interface{})
	ip, _ := payload["ip"].(map[string]interface{})
	if ip["proxy_ip"] != "203.0.113.20" {
		t.Fatalf("proxy_ip = %#v", ip["proxy_ip"])
	}
	proxy, _ := payload["proxy"].(map[string]interface{})
	if proxy["server"] != "127.0.0.1" || proxy["port"] != 12080 {
		t.Fatalf("proxy = %#v", proxy)
	}
	if len(calls) != 5 {
		t.Fatalf("service calls = %v", calls)
	}
	for _, method := range calls {
		if method == "ip.check" {
			t.Fatal("dashboard snapshot performed a synchronous IP check")
		}
	}
}

func TestUIShowDispatch(t *testing.T) {
	t.Parallel()

	called := false
	instance, err := New(Config{
		ShowUIFn: func() error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "show",
		"method":     "ui.show",
	})
	if isErrorResponse(response) || !called {
		t.Fatalf("response = %#v, called = %t", response, called)
	}
}

func TestUIExitDispatchUsesLauncherCoordinator(t *testing.T) {
	t.Parallel()

	called := make(chan struct{}, 1)
	instance, err := New(Config{RequestExitFn: func() { called <- struct{}{} }})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "exit",
		"method":     "ui.exit",
	})
	if isErrorResponse(response) {
		t.Fatalf("response = %#v", response)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("launcher shutdown coordinator was not called")
	}
}

func TestUIExitDispatchFailsClosedWithoutCoordinator(t *testing.T) {
	t.Parallel()

	instance, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "exit",
		"method":     "ui.exit",
	})
	if !isErrorResponse(response) {
		t.Fatalf("response = %#v, want error", response)
	}
}

func TestUIHideDispatchRequiresConfirmedTrayRefresh(t *testing.T) {
	t.Parallel()

	refreshCalls := 0
	instance, err := New(Config{MinimizeToTrayFn: func() error {
		refreshCalls++
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "hide",
		"method":     "ui.hide",
	})
	if isErrorResponse(response) || refreshCalls != 1 {
		t.Fatalf("response = %#v, refreshCalls = %d", response, refreshCalls)
	}
}

func TestUIHideDispatchFailsClosedWhenTrayRefreshFails(t *testing.T) {
	t.Parallel()

	instance, err := New(Config{MinimizeToTrayFn: func() error {
		return errors.New("Explorer tray unavailable")
	}})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "hide",
		"method":     "ui.hide",
	})
	if !isErrorResponse(response) {
		t.Fatalf("response = %#v, want error", response)
	}
}

func TestDashboardProxyEndpoint(t *testing.T) {
	t.Parallel()

	host, port := dashboardProxyEndpoint("127.0.0.1:14000", 12080)
	if host != "127.0.0.1" || port != 14000 {
		t.Fatalf("endpoint = %s:%d", host, port)
	}
	host, port = dashboardProxyEndpoint("", 12080)
	if host != "127.0.0.1" || port != 12080 {
		t.Fatalf("fallback endpoint = %s:%d", host, port)
	}
}
