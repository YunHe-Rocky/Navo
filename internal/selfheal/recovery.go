package selfheal

import "time"

// RepairActionName is a coordinator-owned mutation capability. Policies select
// from this closed set; they never execute Windows networking commands directly.
type RepairActionName string

const (
	ActionNone                  RepairActionName = "none"
	ActionReapplyCapture        RepairActionName = "reapply_capture"
	ActionRestartOwnedCore      RepairActionName = "restart_owned_core"
	ActionReconcileOwnedCapture RepairActionName = "reconcile_owned_capture"
	ActionReconcileOwnedNetwork RepairActionName = "reconcile_owned_network"
	ActionRecoverOwnedNetwork   RepairActionName = "recover_owned_network"
	ActionReapplyTrafficPolicy  RepairActionName = "reapply_traffic_policy"
)

type FaultPlan struct {
	Domain        FaultDomain                       `json:"domain"`
	Evidence      []string                          `json:"evidence"`
	RoundActions  [MaxRepairRounds]RepairActionName `json:"round_actions"`
	Controllable  bool                              `json:"controllable"`
	AllowFailover bool                              `json:"allow_failover"`
	Impact        string                            `json:"impact"`
}

var faultMatrix = []FaultPlan{
	{
		Domain:       FaultDomainNode,
		Evidence:     []string{"active_outbound", "source_channel", "proxy_dns", "proxy_tcp", "proxy_https", "proxy_exit_ip", "routed_traffic"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReapplyCapture, ActionRestartOwnedCore},
		Controllable: true, AllowFailover: true, Impact: "active proxy path is unavailable",
	},
	{
		Domain:       FaultDomainCore,
		Evidence:     []string{"owned_core", "supervisor_state", "pid", "config_hash", "proxy_listener"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionRestartOwnedCore, ActionReapplyCapture},
		Controllable: true, Impact: "Navo core cannot carry proxy traffic",
	},
	{
		Domain:       FaultDomainSystemProxy,
		Evidence:     []string{"ownership_marker", "wininet_enabled", "wininet_endpoint", "preconfig_https"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedCapture, ActionRestartOwnedCore},
		Controllable: true, Impact: "Windows applications are not entering Navo",
	},
	{
		Domain:       FaultDomainTUN,
		Evidence:     []string{"session_id", "adapter_guid", "interface_index", "adapter_state", "health_stage", "routed_traffic"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedCapture, ActionRecoverOwnedNetwork},
		Controllable: true, Impact: "TUN capture is unavailable",
	},
	{
		Domain:       FaultDomainRoute,
		Evidence:     []string{"session_id", "endpoint_bypass", "owned_split_routes", "interface_index", "route_probe"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedNetwork, ActionRecoverOwnedNetwork},
		Controllable: true, Impact: "Navo-owned routes do not carry the expected traffic",
	},
	{
		Domain:       FaultDomainDNS,
		Evidence:     []string{"dns_query", "resolver_path", "owned_nrpt", "proxy_dns"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedNetwork, ActionRecoverOwnedNetwork},
		Controllable: true, Impact: "name resolution through the active Navo path is unavailable",
	},
	{
		Domain:       FaultDomainNRPT,
		Evidence:     []string{"session_id", "owned_nrpt_rule", "resolver_path", "dns_query"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedNetwork, ActionRecoverOwnedNetwork},
		Controllable: true, Impact: "Navo-owned DNS routing is inconsistent",
	},
	{
		Domain:       FaultDomainFirewall,
		Evidence:     []string{"session_id", "owned_firewall_rule", "direction", "action", "remote_cidr"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReconcileOwnedNetwork, ActionRecoverOwnedNetwork},
		Controllable: true, Impact: "a Navo-owned firewall rule blocks or leaks traffic",
	},
	{
		Domain:       FaultDomainTrafficRule,
		Evidence:     []string{"runtime_mode", "list_mode", "compiled_revision", "matched_rule", "route_probe"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionReapplyTrafficPolicy, ActionRestartOwnedCore},
		Controllable: true, Impact: "traffic does not follow the selected Navo policy",
	},
	{
		Domain:       FaultDomainPhysicalNetwork,
		Evidence:     []string{"physical_adapter", "link_state", "gateway", "direct_dns", "direct_https"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionNone, ActionNone},
		Controllable: false, Impact: "the physical network is unavailable; Navo will not modify it",
	},
	{
		Domain:       FaultDomainDetection,
		Evidence:     []string{"probe_scope", "probe_error", "independent_control_probe", "checked_at"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionNone, ActionNone},
		Controllable: false, Impact: "connectivity cannot be determined reliably",
	},
	{
		Domain:       FaultDomainUnknown,
		Evidence:     []string{"observations", "excluded_domains", "checked_at"},
		RoundActions: [MaxRepairRounds]RepairActionName{ActionNone, ActionNone},
		Controllable: false, Impact: "connectivity failed for an unknown reason",
	},
}

func FaultMatrix() []FaultPlan {
	result := make([]FaultPlan, len(faultMatrix))
	for index, plan := range faultMatrix {
		result[index] = plan
		result[index].Evidence = append([]string(nil), plan.Evidence...)
	}
	return result
}

func PlanFor(domain FaultDomain) FaultPlan {
	for _, plan := range faultMatrix {
		if plan.Domain == domain {
			plan.Evidence = append([]string(nil), plan.Evidence...)
			return plan
		}
	}
	return PlanFor(FaultDomainUnknown)
}

func (p FaultPlan) Action(round int) RepairActionName {
	if round < 1 || round > MaxRepairRounds {
		return ActionNone
	}
	return p.RoundActions[round-1]
}

type RecoveryState string

const (
	RecoveryIdle      RecoveryState = "idle"
	RecoveryDetected  RecoveryState = "detected"
	RecoveryRepairing RecoveryState = "repairing"
	RecoveryVerifying RecoveryState = "verifying"
	RecoveryFailover  RecoveryState = "failover"
	RecoveryRecovered RecoveryState = "recovered"
	RecoveryFailed    RecoveryState = "failed"
)

type FaultEvidence struct {
	Code          ErrorCode      `json:"code"`
	Domain        FaultDomain    `json:"domain"`
	Severity      Severity       `json:"severity"`
	Summary       string         `json:"summary"`
	Symptom       string         `json:"symptom"`
	Impact        string         `json:"impact"`
	SourceService string         `json:"source_service"`
	CoreID        string         `json:"core_id,omitempty"`
	OutboundID    string         `json:"outbound_id,omitempty"`
	CaptureMode   string         `json:"capture_mode,omitempty"`
	ObservedAt    time.Time      `json:"observed_at"`
	Details       map[string]any `json:"details,omitempty"`
}

type RoundResult struct {
	Round       int              `json:"round"`
	Action      RepairActionName `json:"action"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt time.Time        `json:"completed_at"`
	Recovered   bool             `json:"recovered"`
	Evidence    string           `json:"evidence,omitempty"`
	Error       string           `json:"error,omitempty"`
	Rollback    string           `json:"rollback,omitempty"`
}

type CandidateResult struct {
	OutboundID  string    `json:"outbound_id"`
	SourceType  string    `json:"source_type"`
	LatencyMS   int64     `json:"latency_ms,omitempty"`
	Reachable   bool      `json:"reachable"`
	Selected    bool      `json:"selected"`
	Verified    bool      `json:"verified"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

type RecoveryReport struct {
	ID          string            `json:"id"`
	State       RecoveryState     `json:"state"`
	Evidence    FaultEvidence     `json:"evidence"`
	Rounds      []RoundResult     `json:"rounds"`
	Candidates  []CandidateResult `json:"candidates,omitempty"`
	Recovered   bool              `json:"recovered"`
	Exhausted   bool              `json:"exhausted"`
	Failover    bool              `json:"failover"`
	FinalError  string            `json:"final_error,omitempty"`
	FinalImpact string            `json:"final_impact,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
