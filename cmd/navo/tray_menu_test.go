package main

import "testing"

func TestBuildTrayMenuContainsAllSpecificationSections(t *testing.T) {
	snapshot := traySnapshot{
		CoreID: "sing-box", CoreState: "running",
		ActiveEndpoint: "node-1", ActiveName: "Node 1",
		CaptureMode: "system_proxy", RouteMode: "rule",
		Connected: true, ConnectionLabel: "🟢 已连接",
		Cores: []trayCore{
			{ID: "sing-box", Name: "sing-box", Installed: true, Color: "green"},
			{ID: "mihomo", Name: "Mihomo", Installed: true, Color: "green"},
			{ID: "xray", Name: "Xray-core", Installed: true, Color: "yellow"},
		},
		Endpoints: []trayEndpoint{{
			ID: "node-1", Name: "Node 1", ProviderID: "provider-1",
			SourceType: "airport_subscription", Active: true,
			Available: true, Color: "green",
		}},
		Subscriptions: []traySubscription{{ID: "provider-1", Name: "Provider A"}},
	}
	menu := buildTrayMenu(snapshot, nil)
	required := []string{
		"状态", "连接控制", "线路选择", "内核模式", "流量接管",
		"规则模式", "诊断工具", "设置", "退出",
	}
	for _, label := range required {
		if !hasTopLevelLabel(menu, label) {
			t.Errorf("missing menu section %q", label)
		}
	}
}

func TestEndpointMenuUsesServiceColorAndReason(t *testing.T) {
	menu := buildEndpointMenus(traySnapshot{
		Endpoints: []trayEndpoint{{
			ID: "bad", Name: "TUIC Node", ProviderID: "provider",
			SourceType: "airport_subscription", Color: "red",
			Reason: "xray does not support protocol tuic",
		}},
	})
	item := menu[0].Children[0].Children[0]
	if !item.Disabled {
		t.Fatal("red endpoint must not be selectable")
	}
	if item.Action == nil || item.Action.Value != "bad" {
		t.Fatalf("unexpected endpoint action: %#v", item.Action)
	}
}

func hasTopLevelLabel(items []trayMenuItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}
