package selfheal

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ErrorCode string
type Category string
type Severity string
type FaultDomain string

const MaxRepairRounds = 2

const (
	CategoryCore         Category = "core"
	CategoryNetwork      Category = "network"
	CategorySubscription Category = "subscription"
	CategoryMonitor      Category = "monitor"
	CategoryLogging      Category = "logging"
	CategorySecurity     Category = "security"

	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
	SeverityFatal Severity = "FATAL"
)

const (
	FaultDomainNode            FaultDomain = "node"
	FaultDomainCore            FaultDomain = "core"
	FaultDomainSystemProxy     FaultDomain = "system_proxy"
	FaultDomainTUN             FaultDomain = "tun"
	FaultDomainRoute           FaultDomain = "route"
	FaultDomainDNS             FaultDomain = "dns"
	FaultDomainNRPT            FaultDomain = "nrpt"
	FaultDomainFirewall        FaultDomain = "firewall"
	FaultDomainTrafficRule     FaultDomain = "traffic_rule"
	FaultDomainPhysicalNetwork FaultDomain = "physical_network"
	FaultDomainDetection       FaultDomain = "detection"
	FaultDomainUnknown         FaultDomain = "unknown"
)

const (
	CodeCoreStartTimeout        ErrorCode = "NAVO_CORE_START_TIMEOUT"
	CodeCoreCrashed             ErrorCode = "NAVO_CORE_CRASHED"
	CodeCoreSwitchFailed        ErrorCode = "NAVO_CORE_SWITCH_FAILED"
	CodeNodeUnavailable         ErrorCode = "NAVO_NODE_UNAVAILABLE"
	CodeCaptureDataPlaneFailed  ErrorCode = "NAVO_CAPTURE_DATAPLANE_FAILED"
	CodeNRPTMismatch            ErrorCode = "NAVO_NRPT_MISMATCH"
	CodeFirewallMismatch        ErrorCode = "NAVO_FIREWALL_MISMATCH"
	CodeTrafficRuleMismatch     ErrorCode = "NAVO_TRAFFIC_RULE_MISMATCH"
	CodePhysicalNetworkDown     ErrorCode = "NAVO_PHYSICAL_NETWORK_UNAVAILABLE"
	CodeDetectionFailed         ErrorCode = "NAVO_DETECTION_FAILED"
	CodeConnectivityUnknown     ErrorCode = "NAVO_CONNECTIVITY_UNKNOWN"
	CodeTUNAdapterMissing       ErrorCode = "NAVO_TUN_ADAPTER_MISSING"
	CodeTUNAdapterDisabled      ErrorCode = "NAVO_TUN_ADAPTER_DISABLED"
	CodeRouteBypassMissing      ErrorCode = "NAVO_ROUTE_ENDPOINT_BYPASS_MISSING"
	CodeDNSMismatch             ErrorCode = "NAVO_DNS_MISMATCH"
	CodeSystemProxyMismatch     ErrorCode = "NAVO_SYSTEM_PROXY_MISMATCH"
	CodeSubscriptionTimeout     ErrorCode = "NAVO_SUBSCRIPTION_FETCH_TIMEOUT"
	CodeSubscriptionParse       ErrorCode = "NAVO_SUBSCRIPTION_PARSE_FAILED"
	CodeTrafficCollectorStale   ErrorCode = "NAVO_TRAFFIC_COLLECTOR_STALE"
	CodeLinkMonitorDirectNoData ErrorCode = "NAVO_LINK_MONITOR_DIRECT_NO_DATA"
	CodeLinkMonitorProxyNoData  ErrorCode = "NAVO_LINK_MONITOR_PROXY_NO_DATA"
	CodeLogWriteFailed          ErrorCode = "NAVO_LOG_WRITE_FAILED"
	CodePrivacyResetFailed      ErrorCode = "NAVO_INIT_PRIVACY_RESET_FAILED"
)

type ErrorEvent struct {
	Code          ErrorCode
	OccurredAt    time.Time
	SourceService string
	ResourceID    string
	CoreID        string
	OutboundID    string
	CorrelationID string
	TransitionID  string
	Count         uint64
}

func (e ErrorEvent) Validate() error {
	if !strings.HasPrefix(string(e.Code), "NAVO_") {
		return fmt.Errorf("stable NAVO error code is required")
	}
	if strings.TrimSpace(e.SourceService) == "" {
		return fmt.Errorf("source service is required")
	}
	return nil
}

type Budget struct {
	MaxAttempts int
	Window      time.Duration
	Cooldown    time.Duration
}

type Definition struct {
	Code            ErrorCode
	Category        Category
	FaultDomain     FaultDomain
	Severity        Severity
	Retryable       bool
	AutoRepair      bool
	RequiresAdmin   bool
	TransitionOwned bool
	Budget          Budget
}

type RepairAction struct {
	Name        string
	Mutated     bool
	RollbackRef string
}

type VerificationResult struct {
	Recovered bool
	Evidence  string
}

// Policy owns one exact stable error code. Implementations must use existing
// domain coordinators rather than mutating processes, routes, DNS or registry directly.
type Policy interface {
	Name() string
	Definition() Definition
	FaultPresent(context.Context, ErrorEvent) (bool, error)
	Repair(context.Context, ErrorEvent) (RepairAction, error)
	Verify(context.Context, ErrorEvent, RepairAction) (VerificationResult, error)
	Rollback(context.Context, ErrorEvent, RepairAction) error
}

type Config struct {
	Enabled             bool
	ObserveOnly         bool
	QueueSize           int
	VerificationTimeout time.Duration
	DefaultMaxAttempts  int
	StateFile           string
	DedupeWindow        time.Duration
}

func DefaultConfig(stateFile string) Config {
	return Config{
		Enabled: true, QueueSize: 128, VerificationTimeout: 15 * time.Second,
		DefaultMaxAttempts: MaxRepairRounds, StateFile: stateFile, DedupeWindow: 5 * time.Second,
	}
}
