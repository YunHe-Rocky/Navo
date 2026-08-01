package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"navo/internal/pipe"
)

const uiPipeName = "Navo.UI.Agent.v1"

type App struct {
	sequence         atomic.Uint64
	startedAt        time.Time
	context          context.Context
	benchmarkMu      sync.Mutex
	benchmarkRunning bool
	benchmarkCancel  context.CancelFunc
	coreUpdateMu     sync.RWMutex
	coreUpdateCache  CoreUpdateReport
}

type Dashboard struct {
	Core    CoreStatus    `json:"core"`
	Cores   []CoreOption  `json:"cores"`
	Proxy   ProxyStatus   `json:"proxy"`
	Runtime RuntimeStatus `json:"runtime"`
	TUN     TUNStatus     `json:"tun"`
	Capture CaptureStatus `json:"capture"`
	Metrics MetricsStatus `json:"metrics"`
	IP      IPStatus      `json:"ip"`
}

type CoreStatus struct {
	CoreID        string `json:"core_id"`
	State         string `json:"state"`
	PID           int    `json:"pid"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	ConfigHash    string `json:"config_hash"`
	RestartCount  int    `json:"restart_count"`
	LastError     string `json:"last_error"`
}

type CoreOption struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Installed            bool     `json:"installed"`
	Active               bool     `json:"active"`
	CaptureModes         []string `json:"capture_modes"`
	SystemProxySupported bool     `json:"system_proxy_supported"`
	TUNSupported         bool     `json:"tun_supported"`
	ControllerSupported  bool     `json:"controller_supported"`
	MetricsSupported     bool     `json:"metrics_supported"`
	DetectionError       string   `json:"detection_error"`
}

type ProxyStatus struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
}

type RuntimeStatus struct {
	Mode       string `json:"mode"`
	ActiveID   string `json:"active_id"`
	TUNEnabled bool   `json:"tun_enabled"`
}

type TUNStatus struct {
	Installed      bool   `json:"installed"`
	Created        bool   `json:"created"`
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	MTU            int    `json:"mtu"`
	State          string `json:"state"`
	Identifier     string `json:"identifier"`
	InterfaceIndex int    `json:"interface_index"`
	FaultID        string `json:"fault_id"`
	LastError      string `json:"last_error"`
}

type CaptureStatus struct {
	State         string `json:"state"`
	Phase         string `json:"phase"`
	DesiredMode   string `json:"desired_mode"`
	CommittedMode string `json:"committed_mode"`
	TransitionID  string `json:"transition_id"`
	FaultID       string `json:"fault_id"`
	LastError     string `json:"last_error"`
	CanRetryTUN   bool   `json:"can_retry_tun"`
}

type MetricsStatus struct {
	Reachable              bool   `json:"reachable"`
	Available              bool   `json:"available"`
	UnavailableReason      string `json:"unavailable_reason"`
	CoreName               string `json:"core_name"`
	LatencyMS              int64  `json:"latency_ms"`
	UploadBytes            int64  `json:"upload_bytes"`
	DownloadBytes          int64  `json:"download_bytes"`
	Connections            int    `json:"connections"`
	LocalAvailable         bool   `json:"local_available"`
	LocalUnavailableReason string `json:"local_unavailable_reason"`
	LocalUploadBPS         uint64 `json:"local_upload_bps"`
	LocalDownloadBPS       uint64 `json:"local_download_bps"`
	ProxyUploadBPS         uint64 `json:"proxy_upload_bps"`
	ProxyDownloadBPS       uint64 `json:"proxy_download_bps"`
	LocalUploadTotal       uint64 `json:"local_upload_total"`
	LocalDownloadTotal     uint64 `json:"local_download_total"`
	ProxyUploadTotal       uint64 `json:"proxy_upload_total"`
	ProxyDownloadTotal     uint64 `json:"proxy_download_total"`
	TrafficSourceState     string `json:"traffic_source_state"`
	TrafficSampledAt       string `json:"traffic_sampled_at"`
}

type IPStatus struct {
	ProxyIP      string `json:"proxy_ip"`
	ProxyCountry string `json:"proxy_country"`
	DirectIP     string `json:"direct_ip"`
	ProxyError   string `json:"proxy_error,omitempty"`
	DirectError  string `json:"direct_error,omitempty"`
	ProbePending bool   `json:"probe_pending,omitempty"`
}

type IPDetection struct {
	Source IPDetectionResult `json:"source"`
	Proxy  IPDetectionResult `json:"proxy"`
}

type IPDetectionResult struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	City      string `json:"city"`
	ASN       string `json:"asn"`
	ISP       string `json:"isp"`
	Network   string `json:"network"`
	Provider  string `json:"provider"`
	Mobile    bool   `json:"mobile"`
	Proxy     bool   `json:"proxy"`
	Hosting   bool   `json:"hosting"`
	CheckedAt string `json:"checked_at"`
	Error     string `json:"error"`
}

type RouteInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"type"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	ProviderID string `json:"provider_id"`
	SourceType string `json:"source_type"`
	Country    string `json:"country"`
	Active     bool   `json:"active"`
}

type Routes struct {
	Items    []RouteInfo `json:"outbounds"`
	ActiveID string      `json:"active_id"`
	Mode     string      `json:"mode"`
}

type SubscriptionInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Configured    bool   `json:"configured"`
	Enabled       bool   `json:"enabled"`
	NodeCount     int    `json:"node_count"`
	LastError     string `json:"last_error"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
}

type Subscriptions struct {
	Items []SubscriptionInfo `json:"subscriptions"`
}

type UpstreamRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"proto"`
	Server    string `json:"server"`
	Port      int    `json:"port"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	UDPPolicy string `json:"udp_policy,omitempty"`
}

type SubscriptionRequest struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
}

type TestResult struct {
	ID        string `json:"id"`
	Reachable bool   `json:"reachable"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error"`
}

type LogEntry struct {
	ID        uint64         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields"`
}

type LogQuery struct {
	Levels   []string `json:"levels"`
	Services []string `json:"services"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	AfterID  uint64   `json:"after_id"`
	Limit    int      `json:"limit"`
}

type LogQueryResult struct {
	Entries    []LogEntry `json:"entries"`
	NextCursor uint64     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

type LogMetadata struct {
	Levels   []string `json:"levels"`
	Services []string `json:"services"`
}

type wireResponse struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewApp() *App {
	return &App{startedAt: time.Now()}
}

func (a *App) startup(ctx context.Context) {
	a.context = ctx
}

func (a *App) GetDashboard() (Dashboard, error) {
	return call[Dashboard](a, "dashboard.snapshot", nil)
}

func (a *App) CheckIP() (IPDetection, error) {
	return call[IPDetection](a, "ip.check", nil)
}

func (a *App) ListRoutes() (Routes, error) {
	return call[Routes](a, "outbound.list", nil)
}

func (a *App) ListSubscriptions() (Subscriptions, error) {
	return call[Subscriptions](a, "subscription.list", nil)
}

func (a *App) SetCore(coreID string) error {
	_, err := call[struct{}](a, "core.select", map[string]any{"core_id": coreID})
	return err
}

func (a *App) SetSystemProxy(enabled bool) error {
	mode := "off"
	if enabled {
		mode = "system_proxy"
	}
	_, err := call[struct{}](a, "capture.set", map[string]any{"mode": mode})
	return err
}

func (a *App) SetTUN(enabled bool) error {
	mode := "system_proxy"
	if enabled {
		mode = "tun"
	}
	_, err := call[struct{}](a, "capture.set", map[string]any{"mode": mode})
	return err
}

func (a *App) SetCaptureMode(mode string) error {
	_, err := call[struct{}](a, "capture.set", map[string]any{"mode": mode})
	return err
}

func (a *App) SetRuntimeMode(mode string) error {
	_, err := call[struct{}](a, "runtime.mode.set", map[string]any{"mode": mode})
	return err
}

func (a *App) SelectRoute(id string) error {
	_, err := call[struct{}](a, "outbound.select", map[string]any{"id": id})
	return err
}

func (a *App) TestRoute(id string) (TestResult, error) {
	return call[TestResult](a, "outbound.test", map[string]any{"id": id})
}

func (a *App) CreateUpstream(request UpstreamRequest) error {
	_, err := call[struct{}](a, "outbound.create", request)
	return err
}

func (a *App) DeleteUpstream(id string) error {
	_, err := call[struct{}](a, "outbound.delete", map[string]any{"id": id})
	return err
}

func (a *App) AddSubscription(request SubscriptionRequest) error {
	payload := map[string]any{
		"name": request.Name, "url": request.URL,
		"skip_tls_verify": request.SkipTLSVerify, "wait": true,
	}
	_, err := call[struct{}](a, "subscription.add", payload)
	return err
}

func (a *App) RefreshSubscriptions() error {
	_, err := call[struct{}](a, "subscription.refresh", map[string]any{"wait": true})
	return err
}

func (a *App) RemoveSubscription(id string) error {
	_, err := call[struct{}](a, "subscription.remove", map[string]any{"id": id})
	return err
}

func (a *App) QueryLogs(query LogQuery) (LogQueryResult, error) {
	return call[LogQueryResult](a, "logs.query", query)
}

func (a *App) GetLogMetadata() (LogMetadata, error) {
	levels, err := call[struct {
		Levels []string `json:"levels"`
	}](a, "logs.levels", nil)
	if err != nil {
		return LogMetadata{}, err
	}
	services, err := call[struct {
		Services []string `json:"services"`
	}](a, "logs.services", nil)
	if err != nil {
		return LogMetadata{}, err
	}
	return LogMetadata{Levels: levels.Levels, Services: services.Services}, nil
}

func (a *App) ClearPersistedLogs() error {
	_, err := call[struct{}](a, "logs.clear.persisted", nil)
	return err
}

func call[T any](app *App, method string, payload any) (T, error) {
	var zero T
	raw, err := app.request(method, payload)
	if err != nil {
		return zero, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return zero, nil
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return zero, fmt.Errorf("decode %s response: %w", method, err)
	}
	return result, nil
}

func (a *App) request(method string, payload any) (json.RawMessage, error) {
	if strings.TrimSpace(method) == "" {
		return nil, fmt.Errorf("method is required")
	}
	request := make(map[string]any)
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode %s request: %w", method, err)
		}
		if err := json.Unmarshal(data, &request); err != nil {
			return nil, fmt.Errorf("normalize %s request: %w", method, err)
		}
	}
	delete(request, "request_id")
	delete(request, "method")
	delete(request, "type")
	request["request_id"] = fmt.Sprintf("ui-%d-%d", time.Now().UnixMilli(), a.sequence.Add(1))
	request["method"] = method
	request["type"] = "REQUEST"

	channel, err := pipe.Dial(uiPipeName)
	if err != nil {
		return nil, fmt.Errorf("connect to Navo Agent: %w", err)
	}
	defer channel.Close()
	if err := channel.SetDeadline(time.Now().Add(45 * time.Second)); err != nil {
		return nil, fmt.Errorf("set IPC deadline: %w", err)
	}
	if err := channel.Send(request); err != nil {
		return nil, fmt.Errorf("send %s request: %w", method, err)
	}
	var response wireResponse
	if err := channel.Receive(&response); err != nil {
		return nil, fmt.Errorf("receive %s response: %w", method, err)
	}
	if response.Type == "ERROR" {
		var detail wireError
		_ = json.Unmarshal(response.Payload, &detail)
		if detail.Code == "" {
			detail.Code = "IPC_ERROR"
		}
		if detail.Message == "" {
			detail.Message = "Navo Agent returned an unknown error"
		}
		return nil, fmt.Errorf("%s: %s", detail.Code, detail.Message)
	}
	return response.Payload, nil
}
