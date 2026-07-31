package main

import (
	"fmt"
	"sort"
)

type trayMenuItem struct {
	Label     string
	Action    *trayAction
	Children  []trayMenuItem
	Disabled  bool
	Checked   bool
	Default   bool
	Separator bool
}

func buildTrayMenu(snapshot traySnapshot, snapshotErr error) []trayMenuItem {
	open := trayMenuItem{
		Label: "打开 Navo", Default: true,
		Action: &trayAction{Kind: trayActionOpen},
	}
	if snapshotErr != nil {
		return []trayMenuItem{
			open,
			{Separator: true},
			{
				Label: "状态",
				Children: []trayMenuItem{
					{Label: "🔴 状态读取失败", Disabled: true},
					{Label: trimMenuText(snapshotErr.Error(), 80), Disabled: true},
				},
			},
			{Separator: true},
			{Label: "退出", Action: &trayAction{Kind: trayActionExit}},
		}
	}

	status := []trayMenuItem{
		{Label: snapshot.ConnectionLabel, Disabled: true},
		{Label: "核心：" + defaultText(snapshot.CoreID, "无"), Disabled: true},
		{Label: "线路：" + defaultText(snapshot.ActiveName, "未选择"), Disabled: true},
		{Label: "接管：" + captureLabel(snapshot.CaptureMode), Disabled: true},
		{Label: "规则：" + routeLabel(snapshot.RouteMode), Disabled: true},
		{
			Label: fmt.Sprintf("连接数：%d  ↑%s  ↓%s",
				snapshot.Connections,
				formatBytes(snapshot.UploadBytes),
				formatBytes(snapshot.DownloadBytes),
			),
			Disabled: true,
		},
	}
	if snapshot.CoreError != "" {
		status = append(status, trayMenuItem{
			Label:    "原因：" + trimMenuText(snapshot.CoreError, 80),
			Disabled: true,
		})
	}
	if snapshot.ExitIP != "" {
		status = append(status, trayMenuItem{
			Label:    "出口：" + snapshot.ExitIP + " " + snapshot.ExitCountry,
			Disabled: true,
		})
	}
	if snapshot.LatencyMS > 0 {
		status = append(status, trayMenuItem{
			Label:    fmt.Sprintf("延迟：%d ms", snapshot.LatencyMS),
			Disabled: true,
		})
	}

	connect := []trayMenuItem{
		actionItem("开启代理", trayAction{Kind: trayActionConnect}, snapshot.Connected),
		actionItem(
			"关闭代理",
			trayAction{Kind: trayActionDisconnect},
			snapshot.CoreState == "stopped" && snapshot.CaptureMode == "off",
		),
		actionItem(
			"重启核心",
			trayAction{Kind: trayActionRestart},
			snapshot.CoreState != "running",
		),
	}

	coreItems := make([]trayMenuItem, 0, len(snapshot.Cores))
	for _, core := range snapshot.Cores {
		label := colorIcon(core.Color) + " " + core.Name
		if core.Reason != "" {
			label += " — " + core.Reason
		}
		coreItems = append(coreItems, checkedActionItem(
			label,
			trayAction{Kind: trayActionSelectCore, Value: core.ID},
			core.ID == snapshot.CoreID,
			!core.Installed || core.Color == "yellow",
		))
	}

	captureItems := []trayMenuItem{
		checkedActionItem(
			"关闭",
			trayAction{Kind: trayActionSelectCapture, Value: "off"},
			snapshot.CaptureMode == "off",
			false,
		),
		checkedActionItem(
			"系统代理"+proxySuffix(snapshot.ProxyServer),
			trayAction{Kind: trayActionSelectCapture, Value: "system_proxy"},
			snapshot.CaptureMode == "system_proxy",
			false,
		),
		checkedActionItem(
			"TUN"+tunSuffix(snapshot.TUNName),
			trayAction{Kind: trayActionSelectCapture, Value: "tun"},
			snapshot.CaptureMode == "tun",
			snapshot.CoreID == "xray",
		),
	}

	routeItems := []trayMenuItem{
		checkedActionItem(
			"全局代理",
			trayAction{Kind: trayActionSelectRoute, Value: "global"},
			snapshot.RouteMode == "global",
			false,
		),
		checkedActionItem(
			"规则代理",
			trayAction{Kind: trayActionSelectRoute, Value: "rule"},
			snapshot.RouteMode == "rule",
			false,
		),
		checkedActionItem(
			"全局直连",
			trayAction{Kind: trayActionSelectRoute, Value: "direct"},
			snapshot.RouteMode == "direct",
			false,
		),
	}

	diagnostics := []trayMenuItem{
		actionItem(
			"测试当前线路",
			trayAction{Kind: trayActionTestEndpoint, Value: snapshot.ActiveEndpoint},
			snapshot.ActiveEndpoint == "",
		),
		actionItem("测试出口 IP", trayAction{Kind: trayActionTestExitIP}, false),
		actionItem("查看核心日志", trayAction{Kind: trayActionShowCoreLogs}, false),
		actionItem("查看连接日志", trayAction{Kind: trayActionShowConnectionLog}, false),
		actionItem("网络恢复", trayAction{Kind: trayActionRecoverNetwork}, false),
		actionItem("导出诊断包", trayAction{Kind: trayActionExportDiagnostics}, false),
	}

	return []trayMenuItem{
		open,
		{Separator: true},
		{Label: "状态", Children: status},
		{Label: "连接控制", Children: connect},
		{Label: "线路选择", Children: buildEndpointMenus(snapshot)},
		{Label: "内核模式", Children: coreItems},
		{Label: "流量接管", Children: captureItems},
		{Label: "规则模式", Children: routeItems},
		{Label: "诊断工具", Children: diagnostics},
		{Label: "设置", Action: &trayAction{Kind: trayActionSettings}},
		{Separator: true},
		{Label: "退出", Action: &trayAction{Kind: trayActionExit}},
	}
}

func buildEndpointMenus(snapshot traySnapshot) []trayMenuItem {
	subscriptionNames := make(map[string]string, len(snapshot.Subscriptions))
	for _, subscription := range snapshot.Subscriptions {
		subscriptionNames[subscription.ID] = subscription.Name
	}
	airport := map[string][]trayEndpoint{}
	var upstream []trayEndpoint
	for _, endpoint := range snapshot.Endpoints {
		if endpoint.SourceType == "upstream_proxy" {
			upstream = append(upstream, endpoint)
			continue
		}
		provider := subscriptionNames[endpoint.ProviderID]
		if provider == "" {
			provider = defaultText(endpoint.ProviderID, "未分组机场")
		}
		airport[provider] = append(airport[provider], endpoint)
	}

	airportProviders := make([]string, 0, len(airport))
	for provider := range airport {
		airportProviders = append(airportProviders, provider)
	}
	sort.Strings(airportProviders)
	airportMenus := make([]trayMenuItem, 0, len(airportProviders))
	for _, provider := range airportProviders {
		airportMenus = append(airportMenus, trayMenuItem{
			Label:    provider,
			Children: endpointItems(airport[provider]),
		})
	}
	if len(airportMenus) == 0 {
		airportMenus = append(airportMenus, trayMenuItem{
			Label: "暂无机场节点", Disabled: true,
		})
	}
	if len(upstream) == 0 {
		upstream = nil
	}
	upstreamMenus := endpointItems(upstream)
	if len(upstreamMenus) == 0 {
		upstreamMenus = []trayMenuItem{{Label: "暂无上游代理", Disabled: true}}
	}
	return []trayMenuItem{
		{Label: "机场订阅", Children: airportMenus},
		{Label: "上游代理", Children: upstreamMenus},
	}
}

func endpointItems(endpoints []trayEndpoint) []trayMenuItem {
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Name < endpoints[j].Name
	})
	result := make([]trayMenuItem, 0, len(endpoints))
	for _, endpoint := range endpoints {
		label := colorIcon(endpoint.Color) + " " + endpoint.Name
		if endpoint.Reason != "" && endpoint.Color != "green" {
			label += " — " + endpoint.Reason
		}
		result = append(result, checkedActionItem(
			trimMenuText(label, 100),
			trayAction{Kind: trayActionSelectEndpoint, Value: endpoint.ID},
			endpoint.Active,
			endpoint.Color == "red",
		))
	}
	return result
}

func actionItem(label string, action trayAction, disabled bool) trayMenuItem {
	return trayMenuItem{Label: label, Action: &action, Disabled: disabled}
}

func checkedActionItem(
	label string,
	action trayAction,
	checked bool,
	disabled bool,
) trayMenuItem {
	return trayMenuItem{
		Label: label, Action: &action, Checked: checked, Disabled: disabled,
	}
}

func colorIcon(color string) string {
	switch color {
	case "green":
		return "🟢"
	case "red":
		return "🔴"
	default:
		return "🟡"
	}
}

func captureLabel(mode string) string {
	switch mode {
	case "system_proxy":
		return "系统代理"
	case "tun":
		return "TUN"
	default:
		return "关闭"
	}
}

func routeLabel(mode string) string {
	switch mode {
	case "rule":
		return "规则代理"
	case "direct":
		return "全局直连"
	default:
		return "全局代理"
	}
}

func proxySuffix(server string) string {
	if server == "" {
		return ""
	}
	return "（" + server + "）"
}

func tunSuffix(name string) string {
	if name == "" {
		return ""
	}
	return "（" + name + "）"
}

func formatBytes(value int64) string {
	const (
		kib = int64(1024)
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case value >= gib:
		return fmt.Sprintf("%.1f GiB", float64(value)/float64(gib))
	case value >= mib:
		return fmt.Sprintf("%.1f MiB", float64(value)/float64(mib))
	case value >= kib:
		return fmt.Sprintf("%.1f KiB", float64(value)/float64(kib))
	default:
		return fmt.Sprintf("%d B", value)
	}
}

func defaultText(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func trimMenuText(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}
