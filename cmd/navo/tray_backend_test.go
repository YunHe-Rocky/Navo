package main

import (
	"context"
	"testing"
)

func TestAgentTrayBackendSnapshotUsesAgentRuntimeState(t *testing.T) {
	backend := newAgentTrayBackend(func(
		_ context.Context,
		message map[string]interface{},
	) map[string]interface{} {
		if message["method"] != "tray.snapshot" {
			t.Fatalf("unexpected method: %v", message["method"])
		}
		return map[string]interface{}{
			"type": "RESPONSE",
			"payload": map[string]interface{}{
				"core.status": map[string]interface{}{
					"core_id": "sing-box", "state": "running",
				},
				"core.list": map[string]interface{}{
					"cores": []map[string]interface{}{{
						"id": "sing-box", "name": "sing-box",
						"installed": true, "color": "green",
					}},
				},
				"runtime.status": map[string]interface{}{
					"mode": "rule", "active_id": "node-1",
					"exit_ip": "203.0.113.8", "exit_country": "US",
				},
				"outbound.list": map[string]interface{}{
					"outbounds": []map[string]interface{}{{
						"id": "node-1", "name": "Node 1",
						"active": true, "available": true,
						"color": "green", "latency_ms": int64(42),
					}},
				},
				"tun.status": map[string]interface{}{"enabled": false},
				"proxy.status": map[string]interface{}{
					"enabled": true, "proxy_server": "127.0.0.1:12080",
				},
				"metrics.current": map[string]interface{}{
					"connections": 2, "upload_bytes": int64(10),
					"download_bytes": int64(20),
				},
				"subscription.list": map[string]interface{}{
					"subscriptions": []map[string]interface{}{},
				},
			},
		}
	})

	snapshot, err := backend.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !snapshot.Connected {
		t.Fatal("Snapshot() must derive connected from runtime and capture state")
	}
	if snapshot.ExitIP != "203.0.113.8" || snapshot.LatencyMS != 42 {
		t.Fatalf(
			"Snapshot() diagnostics = ip:%q latency:%d",
			snapshot.ExitIP,
			snapshot.LatencyMS,
		)
	}
}

func TestAgentTrayBackendMapsActionsToAgentMethods(t *testing.T) {
	tests := []struct {
		action trayAction
		method string
		key    string
		value  string
	}{
		{trayAction{Kind: trayActionSelectEndpoint, Value: "node-1"}, "outbound.select", "id", "node-1"},
		{trayAction{Kind: trayActionSelectCore, Value: "mihomo"}, "core.select", "core_id", "mihomo"},
		{trayAction{Kind: trayActionSelectCapture, Value: "tun"}, "capture.set", "mode", "tun"},
		{trayAction{Kind: trayActionSelectRoute, Value: "global"}, "runtime.mode.set", "mode", "global"},
		{trayAction{Kind: trayActionShowCoreLogs}, "core.log.tail", "", ""},
		{trayAction{Kind: trayActionShowConnectionLog}, "log.tail", "", ""},
	}
	for _, test := range tests {
		t.Run(string(test.action.Kind), func(t *testing.T) {
			backend := newAgentTrayBackend(func(
				_ context.Context,
				message map[string]interface{},
			) map[string]interface{} {
				if message["method"] != test.method {
					t.Fatalf("method = %v, want %s", message["method"], test.method)
				}
				if test.key != "" && message[test.key] != test.value {
					t.Fatalf("%s = %v, want %s", test.key, message[test.key], test.value)
				}
				return map[string]interface{}{
					"type": "RESPONSE",
					"payload": map[string]interface{}{
						"lines": []string{"line"},
					},
				}
			})
			if _, err := backend.Execute(context.Background(), test.action); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}
