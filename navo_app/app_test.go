package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCaptureUIIPCRequestTimeoutCoversHardVerification(t *testing.T) {
	if got := uiIPCRequestTimeout("capture.set"); got != 2*time.Minute {
		t.Fatalf("capture timeout = %s", got)
	}
	if got := uiIPCRequestTimeout("runtime.rules.set"); got != 2*time.Minute {
		t.Fatalf("routing-rule timeout = %s", got)
	}
	if got := uiIPCRequestTimeout("runtime.list_mode.set"); got != 2*time.Minute {
		t.Fatalf("runtime.list_mode.set timeout = %v", got)
	}
	if got := uiIPCRequestTimeout("core.select"); got != 4*time.Minute {
		t.Fatalf("core.select timeout = %v", got)
	}
	if got := uiIPCRequestTimeout("core.status"); got != 45*time.Second {
		if got := uiIPCRequestTimeout("core.update.commit"); got != 4*time.Minute {
			t.Fatalf("core.update.commit timeout = %v", got)
		}
		t.Fatalf("default timeout = %s", got)
	}
}

func TestDashboardSnapshotDecoding(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"core":{"core_id":"sing-box","state":"running"},
		"cores":[{"id":"sing-box","installed":true,"active":true,"capture_modes":["off","system_proxy","tun"],"system_proxy_supported":true,"tun_supported":true}],
		"proxy":{"enabled":true,"server":"127.0.0.1","port":12080},
		"runtime":{"mode":"bypass_mainland","selected_id":"route-2","active_id":"route-1","candidate_id":"route-2","tun_enabled":false,"blacklist":["blocked.example"],"whitelist":["direct.example"]},
		"tun":{"installed":true,"created":true,"enabled":false,"name":"Navo","mtu":1500,"state":"disabled","identifier":"{test-guid}","interface_index":42},
		"capture":{"state":"running_system_proxy","phase":"running","desired_mode":"system_proxy","committed_mode":"system_proxy","transaction":{"busy":true,"id":"tx-1","operation":"node_switch","origin":"user","phase":"verifying","fault_domain":"node","queued":1},"recovery":{"id":"recovery-1","state":"failover","evidence":{"code":"NAVO-E2201","domain":"node","summary":"active node unavailable"},"rounds":[{"round":1,"action":"reapply_capture","recovered":false}],"candidates":[{"outbound_id":"node-b","source_type":"airport_subscription","reachable":true,"selected":true,"verified":false}]}},
		"metrics":{"reachable":true,"latency_ms":12},
		"ip":{"proxy_ip":"203.0.113.20","proxy_country":"Proxy Country","probe_pending":false}
	}`)

	var got Dashboard
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.IP.ProxyIP != "203.0.113.20" {
		t.Fatalf("ProxyIP = %q", got.IP.ProxyIP)
	}
	if got.Proxy.Port != 12080 || !got.Proxy.Enabled {
		t.Fatalf("Proxy = %#v", got.Proxy)
	}
	if got.Capture.CommittedMode != "system_proxy" || got.Capture.State != "running_system_proxy" {
		t.Fatalf("Capture = %#v", got.Capture)
	}
	if !got.Capture.Transaction.Busy || got.Capture.Transaction.Operation != "node_switch" ||
		got.Capture.Transaction.Phase != "verifying" {
		t.Fatalf("Capture transaction = %#v", got.Capture.Transaction)
	}
	if got.Capture.Recovery.State != "failover" || got.Capture.Recovery.Evidence.Domain != "node" ||
		len(got.Capture.Recovery.Rounds) != 1 || len(got.Capture.Recovery.Candidates) != 1 ||
		got.Capture.Recovery.Candidates[0].OutboundID != "node-b" {
		t.Fatalf("Capture recovery = %#v", got.Capture.Recovery)
	}
	if got.Runtime.ActiveID != "route-1" || got.Runtime.CandidateID != "route-2" ||
		got.Runtime.SelectedID != "route-2" {
		t.Fatalf("Runtime selection = %#v", got.Runtime)
	}
	if len(got.Runtime.Blacklist) != 1 || got.Runtime.Blacklist[0] != "blocked.example" || len(got.Runtime.Whitelist) != 1 {
		t.Fatalf("Runtime rules = %#v", got.Runtime)
	}
	if got.TUN.InterfaceIndex != 42 || got.TUN.State != "disabled" {
		t.Fatalf("TUN = %#v", got.TUN)
	}
	if len(got.Cores) != 1 || !got.Cores[0].Active {
		t.Fatalf("Cores = %#v", got.Cores)
	}
	if !got.Cores[0].SystemProxySupported || !got.Cores[0].TUNSupported || len(got.Cores[0].CaptureModes) != 3 {
		t.Fatalf("Core capabilities = %#v", got.Cores[0])
	}
}
