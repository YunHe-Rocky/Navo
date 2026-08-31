export type Page =
  | "overview"
  | "connection"
  | "sources"
  | "cores"
  | "traffic"
  | "ip"
  | "settings";
export type CaptureMode = "off" | "system_proxy" | "tun";
export type RuntimeMode = "bypass_mainland" | "global" | "direct";
export type RoutingListMode = "off" | "blacklist" | "whitelist";
export type CaptureLifecycleState =
  | "stopped"
  | "starting_system_proxy"
  | "running_system_proxy"
  | "starting_tun"
  | "running_tun"
  | "stopping"
  | "recovering"
  | "faulted";
export type SourceType = "airport_subscription" | "upstream_proxy";
export type CoreState = "stopped" | "starting" | "running" | "stopping" | "failed";
export type ConnectionState = "disconnected" | "connecting" | "connected" | "reconnecting" | "failed";
export type NetworkHealthState = "unknown" | "checking" | "healthy" | "degraded" | "unavailable";
export type IconState = "default" | "airport" | "proxy" | "error";

export interface StartupSettings {
  supported: boolean;
  enabled: boolean;
  mode: Extract<CaptureMode, "system_proxy" | "tun">;
  registered: boolean;
  last_error: string;
  checked_at: string;
}

export type RecoveryState = "idle" | "detected" | "repairing" | "verifying" | "failover" | "recovered" | "failed";

export interface RecoveryEvidence {
  code: string;
  domain: string;
  severity: string;
  summary: string;
  symptom: string;
  impact: string;
  source_service: string;
  core_id?: string;
  outbound_id?: string;
  capture_mode?: string;
  observed_at: string;
  details?: Record<string, unknown>;
}

export interface RecoveryRound {
  round: number;
  action: string;
  started_at: string;
  completed_at: string;
  recovered: boolean;
  evidence?: string;
  error?: string;
  rollback?: string;
}

export interface RecoveryCandidate {
  outbound_id: string;
  source_type: string;
  latency_ms?: number;
  reachable: boolean;
  selected: boolean;
  verified: boolean;
  error?: string;
  completed_at: string;
}

export interface RecoveryReport {
  id: string;
  state: RecoveryState;
  evidence: RecoveryEvidence;
  rounds: RecoveryRound[];
  candidates?: RecoveryCandidate[];
  recovered: boolean;
  exhausted: boolean;
  failover: boolean;
  final_error?: string;
  final_impact?: string;
  started_at: string;
  updated_at: string;
}
export interface CoreStatus {
  core_id: string;
  state: string;
  pid: number;
  uptime_seconds: number;
  config_hash: string;
  restart_count: number;
  last_error: string;
}

export interface CoreOption {
  id: string;
  name: string;
  version: string;
  installed: boolean;
  active: boolean;
  capture_modes?: CaptureMode[];
  system_proxy_supported?: boolean;
  tun_supported?: boolean;
  controller_supported?: boolean;
  metrics_supported?: boolean;
  detection_error?: string;
}

export interface MetricsStatus {
  reachable: boolean;
  available: boolean;
  unavailable_reason: string;
  core_name: string;
  latency_ms: number;
  upload_bytes: number;
  download_bytes: number;
  connections: number;
	local_available: boolean;
	local_unavailable_reason: string;
	local_upload_bps: number;
	local_download_bps: number;
	proxy_upload_bps: number;
	proxy_download_bps: number;
	local_upload_total: number;
	local_download_total: number;
	proxy_upload_total: number;
	proxy_download_total: number;
	traffic_source_state: "ready" | "partial" | "unavailable" | "reset";
	traffic_sampled_at: string;
}

export type NetworkOwnership = "none" | "navo" | "external" | "unknown";
export type NetworkFindingSeverity = "info" | "warning" | "error";

export interface NetworkEnvironmentFinding {
  code: string;
  severity: NetworkFindingSeverity;
  domain: string;
  summary: string;
  detail?: string;
  ownership: NetworkOwnership;
  recoverable: boolean;
  transitional: boolean;
}

export interface NetworkResourceSnapshot {
  known: boolean;
  coherent: boolean;
  owned_count: number;
  existing_count: number;
  missing_count: number;
  conflict_count: number;
  last_error?: string;
}

export interface NetworkEnvironmentSnapshot {
  version: number;
  collected_at: string;
  health: NetworkHealthState;
  stale: boolean;
  partial: boolean;
  transition: {
    busy: boolean;
    id?: string;
    operation?: string;
    intent?: string;
    domain?: string;
    priority?: number;
    phase?: string;
    fault_domain?: string;
  };
  capture: {
    state: string;
    desired_mode: CaptureMode;
    committed_mode: CaptureMode;
    fault_id?: string;
    readiness_state?: string;
    readiness_error?: string;
  };
  physical: {
    known: boolean;
    available: boolean;
    active_interfaces?: string[];
    last_error?: string;
  };
  system_proxy: {
    enabled: boolean;
    proxy_server?: string;
    ownership: NetworkOwnership;
    owned_by_navo: boolean;
    ownership_marker: boolean;
    ownership_lost: boolean;
    local_endpoint: boolean;
    reachable: boolean;
    reachable_known: boolean;
    last_error?: string;
  };
  tun: {
    expected: boolean;
    navo: {
      present: boolean;
      enabled: boolean;
      name?: string;
      state?: string;
      ownership: NetworkOwnership;
      last_error?: string;
    };
    external_present: boolean;
    external?: Array<{ name: string; interface_index?: number; state?: string }>;
  };
  dns: NetworkResourceSnapshot;
  routes: NetworkResourceSnapshot;
  nrpt: NetworkResourceSnapshot;
  firewall: NetworkResourceSnapshot;
  journal: {
    present: boolean;
    dirty: boolean;
    owned_resources: number;
    preexisting_resources: number;
    pending_actions: number;
    missing_resources: number;
    conflicting_resources: number;
    last_error?: string;
  };
  findings: NetworkEnvironmentFinding[];
  observation_errors?: string[];
}

export interface Dashboard {
  environment?: NetworkEnvironmentSnapshot;
  core: CoreStatus;
  cores: CoreOption[];
  proxy: { enabled: boolean; server: string; port: number };
  runtime: {
    mode: RuntimeMode;
    list_mode: RoutingListMode;
    selected_id: string;
    active_id: string;
    candidate_id: string;
    tun_enabled: boolean;
    blacklist: string[];
    whitelist: string[];
  };
  tun: {
    installed: boolean;
    created: boolean;
    enabled: boolean;
    name: string;
    mtu: number;
    state: string;
    identifier: string;
    interface_index: number;
    fault_id: string;
    last_error: string;
  };
  capture: {
    state: CaptureLifecycleState;
    phase: string;
    desired_mode: CaptureMode;
    committed_mode: CaptureMode;
    transition_id: string;
    fault_id: string;
    last_error: string;
    can_retry_tun: boolean;
    updated_at: string;
    readiness: {
      state: "unverified" | "checking" | "ready" | "failed" | string;
      scope: string;
      sites: Record<string, {
        dns: boolean;
        tcp: boolean;
        https: boolean;
        status_code?: number;
      }>;
      default_proxy: boolean;
      checked_at: string;
      error?: string;
    };
    recovery?: RecoveryReport;
    transaction?: {
      busy: boolean;
      id: string;
      operation: string;
      origin: string;
      intent?: string;
      domain?: string;
      priority?: number;
      phase: string;
      fault_domain: string;
      started_at: string;
      queued: number;
      last_id: string;
      last_operation: string;
      last_phase: string;
      last_error: string;
      completed_at: string;
    };
  };
  metrics: MetricsStatus;
  ip: {
    connection_kind: "direct" | "navo" | "external_system_proxy" | string;
    proxy_ip: string;
    proxy_country: string;
    direct_ip: string;
    proxy_error: string;
    direct_error: string;
    proxy_provider: string;
    direct_provider: string;
    proxy_checked_at: string;
    direct_checked_at: string;
    probe_pending?: boolean;
  };
}

export interface IPDetectionResult {
  outbound_id?: string;
  available?: boolean;
  state?: "available" | "inactive" | "unavailable";
  ip: string;
  country: string;
  city: string;
  asn: string;
  isp: string;
  network: string;
  provider: string;
  mobile: boolean;
  proxy: boolean;
	hosting: boolean;
	checked_at: string;
	error: string;
}

export interface IPDetection {
  connection_kind?: "direct" | "navo" | "external_system_proxy" | string;
  source: IPDetectionResult;
  proxy: IPDetectionResult;
}

export interface TrafficPoint {
  timestamp: number;
	localUploadBps: number;
	localDownloadBps: number;
	proxyUploadBps: number;
	proxyDownloadBps: number;
	localUploadTotal: number;
	localDownloadTotal: number;
	proxyUploadTotal: number;
	proxyDownloadTotal: number;
  routeID: string;
	simulated?: boolean;
}

export type TrafficSeries =
	| "localUploadBps"
	| "localDownloadBps"
	| "proxyUploadBps"
	| "proxyDownloadBps";

export interface TrafficChartPreferences {
	visibleSeries: TrafficSeries[];
	windowSeconds: number;
}

export interface RouteInfo {
  id: string;
  name: string;
  type: string;
  server: string;
  port: number;
  provider_id: string;
  source_type: SourceType;
  country: string;
  active: boolean;
  candidate: boolean;
  selected: boolean;
}

export interface Routes {
  outbounds: RouteInfo[];
  selected_id: string;
  active_id: string;
  candidate_id: string;
  mode: string;
}

export interface SubscriptionInfo {
  id: string;
  name: string;
  configured: boolean;
  enabled: boolean;
  node_count: number;
  last_error: string;
  skip_tls_verify: boolean;
}

export interface Subscriptions {
  subscriptions: SubscriptionInfo[];
}

export interface UpstreamRequest {
  name: string;
  proto: "http" | "https" | "socks5";
  server: string;
  port: number;
  username: string;
  password: string;
  udp_policy: "disabled" | "prefer" | "require";
}

export interface SubscriptionRequest {
  name: string;
  url: string;
  skip_tls_verify: boolean;
}

export interface TestResult {
  id: string;
  reachable: boolean;
  latency_ms: number;
  error: string;
}

export interface HostStatus {
  os: string;
  arch: string;
  app_version: string;
  go_version: string;
  logical_cpu: number;
  memory_total_bytes: number;
  memory_available_bytes: number;
  memory_usage_percent: number;
  system_uptime_seconds: number;
  process_uptime_seconds: number;
}

export interface ProxyBenchmark {
  proxy_endpoint: string;
  test_server: string;
  latency_ms: number;
  jitter_ms: number;
  download_mbps: number;
  upload_mbps: number;
  download_bytes: number;
  upload_bytes: number;
  duration_ms: number;
  checked_at: string;
}

export interface LatencyResult {
	outbound_id: string;
	state: "testing" | "completed" | "partial" | "failed";
	tcp_connect_ms: number;
	proxy_handshake_ms: number;
	dns_ms: number;
	tls_ms: number;
	ttfb_ms: number;
	total_ms: number;
	exit_ip?: string;
	checked_at: string;
	error_code?: string;
	error_message?: string;
	dns_observable: boolean;
}

export interface CoreUpdateStatus {
  id: string;
  name: string;
  current_version: string;
  latest_version: string;
  update_available: boolean;
  integrity_ok: boolean;
  release_url: string;
  error: string;
	state: "checking" | "update_available" | "up_to_date" | "failed";
	install_supported: boolean;
	install_blocked_reason?: string;
	asset_name?: string;
}

export interface CoreUpdateReport {
  items: CoreUpdateStatus[];
  checked_at: string;
}

export interface LogEntry {
	id: number;
	timestamp: string;
	level: "DEBUG" | "INFO" | "WARN" | "ERROR";
	category: string;
	service: string;
	component: string;
	message: string;
	fields?: Record<string, unknown>;
}

export interface LogQuery {
	levels: string[];
	categories: string[];
	services: string[];
	from: string;
	to: string;
	after_id: number;
	limit: number;
}

export interface LogQueryResult {
	entries: LogEntry[];
	next_cursor: number;
	has_more: boolean;
}

export interface LogMetadata {
	levels: string[];
	categories: string[];
	services: string[];
}
