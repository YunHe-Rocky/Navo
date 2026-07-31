package ipc

import "time"

// ── Core Control Messages ──

// CoreStartRequest requests starting the proxy core.
type CoreStartRequest struct {
	ConfigPath string `json:"config_path"`
}

// CoreStartResponse is the result of starting the core.
type CoreStartResponse struct {
	PID    int    `json:"pid"`
	Status string `json:"status"`
}

// CoreStopRequest requests stopping the proxy core.
type CoreStopRequest struct {
	Force          bool `json:"force"`
	TimeoutSeconds int  `json:"timeout_seconds"`
}

// CoreStatusResponse reports the current core status.
type CoreStatusResponse struct {
	State         string `json:"state"`
	PID           int    `json:"pid"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	ConfigHash    string `json:"config_hash"`
	RestartCount  int    `json:"restart_count"`
	LastError     string `json:"last_error,omitempty"`
}

// ── Outbound Management Messages ──

// OutboundInfo describes a single outbound.
type OutboundInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	SourceType   string `json:"source_type"`
	Server       string `json:"server"`
	Port         int    `json:"port"`
	Enabled      bool   `json:"enabled"`
	ProviderName string `json:"provider_name,omitempty"`
	Country      string `json:"country,omitempty"`
}

// OutboundListResponse lists all outbounds.
type OutboundListResponse struct {
	Outbounds []OutboundInfo `json:"outbounds"`
}

// OutboundCreateRequest creates an independent HTTP/HTTPS/SOCKS5 upstream proxy.
// Airport protocol nodes can only enter through the subscription pipeline.
type OutboundCreateRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"proto"`
	Server    string `json:"server"`
	Port      int    `json:"port"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	UDPPolicy string `json:"udp_policy"`
}

// OutboundCreateResponse is the result of creating an outbound.
type OutboundCreateResponse struct {
	ID string `json:"id"`
}

// OutboundTestRequest tests connectivity of an outbound.
type OutboundTestRequest struct {
	OutboundID string `json:"outbound_id"`
}

// OutboundTestResponse shows test results.
type OutboundTestResponse struct {
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// ── Rule Management Messages ──

// RuleCreateRequest creates a routing rule.
type RuleCreateRequest struct {
	Name         string   `json:"name"`
	Priority     int      `json:"priority"`
	Domains      []string `json:"domains,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	ProcessNames []string `json:"process_names,omitempty"`
	OutboundID   string   `json:"outbound_id"`
}

// RuleSimulateRequest simulates which rule would match a given request.
type RuleSimulateRequest struct {
	ProcessName string `json:"process_name"`
	Domain      string `json:"domain"`
}

// RuleSimulateResponse shows the simulation result.
type RuleSimulateResponse struct {
	Matched    bool   `json:"matched"`
	OutboundID string `json:"outbound_id,omitempty"`
	RuleName   string `json:"rule_name,omitempty"`
}

// ── Config Management Messages ──

// ActiveSelectionDTO is the only runtime selection accepted by the compiler.
type ActiveSelectionDTO struct {
	CoreID          string `json:"core_id"`
	SourceType      string `json:"source_type"`
	ProviderID      string `json:"provider_id,omitempty"`
	EndpointID      string `json:"endpoint_id,omitempty"`
	UpstreamProxyID string `json:"upstream_proxy_id,omitempty"`
	CaptureMode     string `json:"capture_mode"`
	RuntimeMode     string `json:"runtime_mode"`
}

// ConfigCompileRequest compiles one typed selection for one explicit core.
type ConfigCompileRequest struct {
	Selection ActiveSelectionDTO `json:"selection"`
}

// ConfigCompileResponse is the compilation result.
type ConfigCompileResponse struct {
	ConfigHash string `json:"config_hash"`
	JSON       string `json:"json"`
	Validated  bool   `json:"validated"`
}

// ConfigRollbackRequest rolls back to a previous revision.
type ConfigRollbackRequest struct {
	ToVersion int `json:"to_version"`
}

// ConfigRollbackResponse is the rollback result.
type ConfigRollbackResponse struct {
	NewVersion int    `json:"new_version"`
	ConfigHash string `json:"config_hash"`
}

// ConfigRevisionInfo describes a config revision.
type ConfigRevisionInfo struct {
	ID           string     `json:"id"`
	Version      int        `json:"version"`
	Status       string     `json:"status"`
	ConfigHash   string     `json:"config_hash"`
	CreatedAt    time.Time  `json:"created_at"`
	ActivatedAt  *time.Time `json:"activated_at,omitempty"`
	RollbackFrom int        `json:"rollback_from,omitempty"`
}

// ConfigRevisionsResponse lists all config revisions.
type ConfigRevisionsResponse struct {
	Revisions []ConfigRevisionInfo `json:"revisions"`
}

// ── Event Messages ──

// StateChangedEvent is emitted when the supervisor state changes.
type StateChangedEvent struct {
	From StateChangeInfo `json:"from"`
	To   StateChangeInfo `json:"to"`
}

// StateChangeInfo holds state change details.
type StateChangeInfo struct {
	State     string `json:"state"`
	Timestamp int64  `json:"timestamp"`
}

// MetricUpdateEvent holds a snapshot of network metrics.
type MetricUpdateEvent struct {
	OutboundID string  `json:"outbound_id"`
	Latency    float64 `json:"latency_ms"`
	Upload     int64   `json:"upload_bps"`
	Download   int64   `json:"download_bps"`
}

// IPChangedEvent is emitted when the outbound IP changes.
type IPChangedEvent struct {
	OldIP   string `json:"old_ip,omitempty"`
	NewIP   string `json:"new_ip"`
	Country string `json:"country,omitempty"`
	City    string `json:"city,omitempty"`
	ASN     string `json:"asn,omitempty"`
}

// ── TUN Messages ──

// TUNConfigRequest configures TUN mode parameters.
type TUNConfigRequest struct {
	Name    string   `json:"name"`
	MTU     int      `json:"mtu"`
	Address []string `json:"address"`
	DNS     []string `json:"dns"`
	Gateway string   `json:"gateway,omitempty"`
}

// TUNStatusResponse reports the current TUN adapter status.
type TUNStatusResponse struct {
	Installed  bool     `json:"installed"`
	Enabled    bool     `json:"enabled"`
	Name       string   `json:"name,omitempty"`
	Addresses  []string `json:"addresses,omitempty"`
	MTU        int      `json:"mtu"`
	RouteCount int      `json:"route_count"`
}

// TUNStateChangedEvent is emitted when the TUN adapter state changes.
type TUNStateChangedEvent struct {
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Reason   string `json:"reason,omitempty"`
}

// ── Subscription Messages ──

// SubAddRequest adds a new subscription.
type SubAddRequest struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	SkipTLSVerify bool   `json:"skip_tls_verify,omitempty"`
}

// SubInfo represents a subscription entry.
type SubInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Configured    bool   `json:"configured"`
	Enabled       bool   `json:"enabled"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	NodeCount     int    `json:"node_count"`
}

// ── Metrics Messages ──

// MetricsResponse holds current network metrics.
type MetricsResponse struct {
	OutboundID string  `json:"outbound_id"`
	Latency    int64   `json:"latency_ms"`
	Upload     int64   `json:"upload_bytes"`
	Download   int64   `json:"download_bytes"`
	LossRate   float64 `json:"loss_rate"`
}

// ── IP Detection Messages ──

// IPCheckResponse holds IP detection results.
type IPCheckResponse struct {
	OutboundID string `json:"outbound_id"`
	IP         string `json:"ip"`
	Country    string `json:"country,omitempty"`
	City       string `json:"city,omitempty"`
	ASN        string `json:"asn,omitempty"`
	Error      string `json:"error,omitempty"`
}
