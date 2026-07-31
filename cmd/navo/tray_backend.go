package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type trayActionKind string

const (
	trayActionOpen              trayActionKind = "open"
	trayActionSettings          trayActionKind = "settings"
	trayActionExit              trayActionKind = "exit"
	trayActionConnect           trayActionKind = "connect"
	trayActionDisconnect        trayActionKind = "disconnect"
	trayActionRestart           trayActionKind = "restart"
	trayActionSelectEndpoint    trayActionKind = "select_endpoint"
	trayActionSelectCore        trayActionKind = "select_core"
	trayActionSelectCapture     trayActionKind = "select_capture"
	trayActionSelectRoute       trayActionKind = "select_route"
	trayActionTestEndpoint      trayActionKind = "test_endpoint"
	trayActionTestExitIP        trayActionKind = "test_exit_ip"
	trayActionShowCoreLogs      trayActionKind = "show_core_logs"
	trayActionShowConnectionLog trayActionKind = "show_connection_logs"
	trayActionRecoverNetwork    trayActionKind = "recover_network"
	trayActionExportDiagnostics trayActionKind = "export_diagnostics"
)

type trayAction struct {
	Kind  trayActionKind
	Value string
}

type trayEndpoint struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"type"`
	ProviderID string `json:"provider_id"`
	SourceType string `json:"source_type"`
	Active     bool   `json:"active"`
	Available  bool   `json:"available"`
	Color      string `json:"color"`
	Reason     string `json:"reason"`
	LatencyMS  int64  `json:"latency_ms"`
}

type trayCore struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Color     string `json:"color"`
	Reason    string `json:"reason"`
}

type traySubscription struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type traySnapshot struct {
	CoreID          string
	CoreState       string
	CoreError       string
	Cores           []trayCore
	ActiveEndpoint  string
	ActiveName      string
	CaptureMode     string
	RouteMode       string
	ProxyServer     string
	TUNName         string
	Endpoints       []trayEndpoint
	Subscriptions   []traySubscription
	Connections     int
	UploadBytes     int64
	DownloadBytes   int64
	ExitIP          string
	ExitCountry     string
	LatencyMS       int64
	Connected       bool
	ConnectionLabel string
}

type trayBackend interface {
	Snapshot(ctx context.Context) (traySnapshot, error)
	Execute(ctx context.Context, action trayAction) (string, error)
}

type trayDispatch func(
	ctx context.Context,
	message map[string]interface{},
) map[string]interface{}

type agentTrayBackend struct {
	dispatch trayDispatch
	sequence atomic.Uint64
}

func newAgentTrayBackend(dispatch trayDispatch) *agentTrayBackend {
	return &agentTrayBackend{dispatch: dispatch}
}

func (b *agentTrayBackend) request(
	ctx context.Context,
	method string,
	payload map[string]interface{},
) (map[string]interface{}, error) {
	message := map[string]interface{}{
		"request_id": fmt.Sprintf(
			"tray-%d-%d",
			time.Now().UnixMilli(),
			b.sequence.Add(1),
		),
		"type":   "REQUEST",
		"method": method,
	}
	for key, value := range payload {
		message[key] = value
	}
	response := b.dispatch(ctx, message)
	if response == nil {
		return nil, fmt.Errorf("%s returned no response", method)
	}
	if response["type"] == "ERROR" {
		detail := mapValue(response["payload"])
		code := stringValue(detail["code"])
		if code == "" {
			code = "TRAY_REQUEST_FAILED"
		}
		return nil, fmt.Errorf("%s: %s", code, stringValue(detail["message"]))
	}
	return mapValue(response["payload"]), nil
}

func (b *agentTrayBackend) Snapshot(ctx context.Context) (traySnapshot, error) {
	payload, err := b.request(ctx, "tray.snapshot", nil)
	if err != nil {
		return traySnapshot{}, err
	}
	var wire struct {
		Core struct {
			CoreID    string `json:"core_id"`
			State     string `json:"state"`
			LastError string `json:"last_error"`
		} `json:"core.status"`
		CoreList struct {
			Cores []trayCore `json:"cores"`
		} `json:"core.list"`
		Runtime struct {
			Mode        string `json:"mode"`
			ActiveID    string `json:"active_id"`
			ExitIP      string `json:"exit_ip"`
			ExitCountry string `json:"exit_country"`
		} `json:"runtime.status"`
		Outbounds struct {
			Items []trayEndpoint `json:"outbounds"`
		} `json:"outbound.list"`
		TUN struct {
			Enabled bool   `json:"enabled"`
			Name    string `json:"name"`
		} `json:"tun.status"`
		Proxy struct {
			Enabled bool   `json:"enabled"`
			Server  string `json:"proxy_server"`
		} `json:"proxy.status"`
		Metrics struct {
			Connections   int   `json:"connections"`
			UploadBytes   int64 `json:"upload_bytes"`
			DownloadBytes int64 `json:"download_bytes"`
		} `json:"metrics.current"`
		Subscriptions struct {
			Items []traySubscription `json:"subscriptions"`
		} `json:"subscription.list"`
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return traySnapshot{}, fmt.Errorf("encode tray snapshot: %w", err)
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return traySnapshot{}, fmt.Errorf("decode tray snapshot: %w", err)
	}

	captureMode := "off"
	if wire.TUN.Enabled {
		captureMode = "tun"
	} else if wire.Proxy.Enabled {
		captureMode = "system_proxy"
	}
	activeName := ""
	activeAvailable := false
	activeLatency := int64(0)
	for _, endpoint := range wire.Outbounds.Items {
		if endpoint.ID == wire.Runtime.ActiveID {
			activeName = endpoint.Name
			activeAvailable = endpoint.Available
			activeLatency = endpoint.LatencyMS
			break
		}
	}
	connected := wire.Core.State == "running" &&
		wire.Runtime.ActiveID != "" &&
		captureMode != "off" &&
		activeAvailable
	label := "⚪ 未连接"
	switch {
	case wire.Core.State == "failed" || wire.Core.LastError != "":
		label = "🔴 连接失败"
	case connected:
		label = "🟢 已连接"
	case wire.Core.State == "running":
		label = "🟡 内核运行中"
	}
	return traySnapshot{
		CoreID: wire.Core.CoreID, CoreState: wire.Core.State,
		CoreError: wire.Core.LastError, Cores: wire.CoreList.Cores,
		ActiveEndpoint: wire.Runtime.ActiveID, ActiveName: activeName,
		CaptureMode: captureMode, RouteMode: wire.Runtime.Mode,
		ProxyServer: wire.Proxy.Server, TUNName: wire.TUN.Name,
		Endpoints:     wire.Outbounds.Items,
		Subscriptions: wire.Subscriptions.Items,
		Connections:   wire.Metrics.Connections,
		UploadBytes:   wire.Metrics.UploadBytes,
		DownloadBytes: wire.Metrics.DownloadBytes,
		ExitIP:        wire.Runtime.ExitIP,
		ExitCountry:   wire.Runtime.ExitCountry,
		LatencyMS:     activeLatency,
		Connected:     connected, ConnectionLabel: label,
	}, nil
}

func (b *agentTrayBackend) Execute(
	ctx context.Context,
	action trayAction,
) (string, error) {
	method := ""
	payload := map[string]interface{}{}
	switch action.Kind {
	case trayActionConnect:
		method = "connection.enable"
	case trayActionDisconnect:
		method = "connection.disable"
	case trayActionRestart:
		method = "connection.restart"
	case trayActionSelectEndpoint:
		method, payload["id"] = "outbound.select", action.Value
	case trayActionSelectCore:
		method, payload["core_id"] = "core.select", action.Value
	case trayActionSelectCapture:
		method, payload["mode"] = "capture.set", action.Value
	case trayActionSelectRoute:
		method, payload["mode"] = "runtime.mode.set", action.Value
	case trayActionTestEndpoint:
		method, payload["id"] = "outbound.test", action.Value
	case trayActionTestExitIP:
		method = "ip.check"
	case trayActionShowCoreLogs:
		method = "core.log.tail"
	case trayActionShowConnectionLog:
		method = "log.tail"
	case trayActionRecoverNetwork:
		method = "network.recover"
	case trayActionExportDiagnostics:
		method = "diagnostics.export"
	default:
		return "", fmt.Errorf("unsupported tray action %q", action.Kind)
	}
	result, err := b.request(ctx, method, payload)
	if err != nil {
		return "", err
	}
	return trayResultMessage(action, result), nil
}

func trayResultMessage(action trayAction, payload map[string]interface{}) string {
	switch action.Kind {
	case trayActionTestEndpoint:
		if boolValue(payload["reachable"]) {
			return fmt.Sprintf("线路可用，延迟 %d ms", int64Value(payload["latency_ms"]))
		}
		return "线路不可用：" + stringValue(payload["error"])
	case trayActionTestExitIP:
		proxy := mapValue(payload["proxy"])
		return fmt.Sprintf(
			"代理出口：%s\n地区：%s",
			stringValue(proxy["ip"]),
			stringValue(proxy["country"]),
		)
	case trayActionShowCoreLogs, trayActionShowConnectionLog:
		lines := stringSlice(payload["lines"])
		if len(lines) > 30 {
			lines = lines[len(lines)-30:]
		}
		return strings.Join(lines, "\n")
	case trayActionRecoverNetwork:
		return fmt.Sprintf(
			"网络恢复完成：修复 %d 项，未修复 %d 项",
			len(anySlice(payload["issues_fixed"])),
			len(anySlice(payload["issues_unfixed"])),
		)
	case trayActionExportDiagnostics:
		return "诊断文件已导出：\n" + stringValue(payload["path"])
	default:
		return "操作已完成"
	}
}

func mapValue(value interface{}) map[string]interface{} {
	if result, ok := value.(map[string]interface{}); ok {
		return result
	}
	data, _ := json.Marshal(value)
	result := map[string]interface{}{}
	_ = json.Unmarshal(data, &result)
	return result
}

func stringValue(value interface{}) string {
	result, _ := value.(string)
	return result
}

func boolValue(value interface{}) bool {
	result, _ := value.(bool)
	return result
}

func int64Value(value interface{}) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int:
		return int64(number)
	case int64:
		return number
	default:
		return 0
	}
}

func anySlice(value interface{}) []interface{} {
	switch values := value.(type) {
	case []interface{}:
		return values
	case []string:
		result := make([]interface{}, 0, len(values))
		for _, item := range values {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func stringSlice(value interface{}) []string {
	if values, ok := value.([]string); ok {
		return values
	}
	values := anySlice(value)
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, stringValue(value))
	}
	return result
}
