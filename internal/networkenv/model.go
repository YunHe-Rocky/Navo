// Package networkenv owns Navo's read-only, platform-neutral network
// environment model. It aggregates authoritative facts; it never mutates the
// operating system or owns recovery state.
package networkenv

import "time"

const (
	SnapshotVersion    = 1
	SnapshotStaleAfter = 10 * time.Second
)

type HealthState string

const (
	HealthUnknown     HealthState = "unknown"
	HealthChecking    HealthState = "checking"
	HealthHealthy     HealthState = "healthy"
	HealthDegraded    HealthState = "degraded"
	HealthUnavailable HealthState = "unavailable"
)

type Ownership string

const (
	OwnerNone     Ownership = "none"
	OwnerNavo     Ownership = "navo"
	OwnerExternal Ownership = "external"
	OwnerUnknown  Ownership = "unknown"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warning"
	SeverityError Severity = "error"
)

const (
	FindingSystemProxyExternal            = "ENV_SYSTEM_PROXY_EXTERNAL"
	FindingSystemProxyStaleNavo           = "ENV_SYSTEM_PROXY_STALE_NAVO"
	FindingSystemProxyEndpointUnreachable = "ENV_SYSTEM_PROXY_ENDPOINT_UNREACHABLE"
	FindingExternalTUNPresent             = "ENV_EXTERNAL_TUN_PRESENT"
	FindingNavoTUNMissing                 = "ENV_NAVO_TUN_MISSING"
	FindingNavoTUNDisabled                = "ENV_NAVO_TUN_DISABLED"
	FindingNavoRouteResidual              = "ENV_NAVO_ROUTE_RESIDUAL"
	FindingNavoDNSInconsistent            = "ENV_NAVO_DNS_INCONSISTENT"
	FindingNavoNRPTInconsistent           = "ENV_NAVO_NRPT_INCONSISTENT"
	FindingNavoFirewallInconsistent       = "ENV_NAVO_FIREWALL_INCONSISTENT"
	FindingNetworkJournalPending          = "ENV_NETWORK_JOURNAL_PENDING"
	FindingPhysicalNetworkUnavailable     = "ENV_PHYSICAL_NETWORK_UNAVAILABLE"
	FindingCaptureDataPlaneFailed         = "ENV_CAPTURE_DATAPLANE_FAILED"
	FindingObservationPartial             = "ENV_OBSERVATION_PARTIAL"
)

type TransitionSnapshot struct {
	Busy        bool   `json:"busy"`
	ID          string `json:"id,omitempty"`
	Operation   string `json:"operation,omitempty"`
	Intent      string `json:"intent,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Phase       string `json:"phase,omitempty"`
	FaultDomain string `json:"fault_domain,omitempty"`
}

type PhysicalSnapshot struct {
	Known            bool     `json:"known"`
	Available        bool     `json:"available"`
	ActiveInterfaces []string `json:"active_interfaces,omitempty"`
	LastError        string   `json:"last_error,omitempty"`
}

type SystemProxySnapshot struct {
	Enabled              bool      `json:"enabled"`
	ProxyServer          string    `json:"proxy_server,omitempty"`
	AutoDetect           bool      `json:"auto_detect"`
	AutoConfigConfigured bool      `json:"auto_config_configured"`
	BypassConfigured     bool      `json:"bypass_configured"`
	Ownership            Ownership `json:"ownership"`
	OwnedByNavo          bool      `json:"owned_by_navo"`
	OwnershipMarker      bool      `json:"ownership_marker"`
	OwnershipLost        bool      `json:"ownership_lost"`
	LocalEndpoint        bool      `json:"local_endpoint"`
	Reachable            bool      `json:"reachable"`
	ReachableKnown       bool      `json:"reachable_known"`
	LastError            string    `json:"last_error,omitempty"`
}

type TUNAdapterSnapshot struct {
	Present        bool      `json:"present"`
	Enabled        bool      `json:"enabled"`
	Name           string    `json:"name,omitempty"`
	InterfaceGUID  string    `json:"interface_guid,omitempty"`
	InterfaceIndex int       `json:"interface_index,omitempty"`
	State          string    `json:"state,omitempty"`
	Ownership      Ownership `json:"ownership"`
	SessionID      string    `json:"session_id,omitempty"`
	Stage          string    `json:"stage,omitempty"`
	FaultID        string    `json:"fault_id,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
}

type ExternalAdapterRef struct {
	Name           string `json:"name"`
	InterfaceIndex int    `json:"interface_index,omitempty"`
	State          string `json:"state,omitempty"`
}

type TUNSnapshot struct {
	Expected        bool                 `json:"expected"`
	Navo            TUNAdapterSnapshot   `json:"navo"`
	ExternalPresent bool                 `json:"external_present"`
	External        []ExternalAdapterRef `json:"external,omitempty"`
}

// ResourceSnapshot summarizes only resources proven through the existing
// Navo journal/ownership authority. It never lists third-party configuration.
type ResourceSnapshot struct {
	Known         bool   `json:"known"`
	Coherent      bool   `json:"coherent"`
	OwnedCount    int    `json:"owned_count"`
	ExistingCount int    `json:"existing_count"`
	MissingCount  int    `json:"missing_count"`
	ConflictCount int    `json:"conflict_count"`
	LastError     string `json:"last_error,omitempty"`
}

type JournalSnapshot struct {
	Present              bool   `json:"present"`
	Dirty                bool   `json:"dirty"`
	Version              int    `json:"version,omitempty"`
	SessionID            string `json:"session_id,omitempty"`
	AdapterName          string `json:"adapter_name,omitempty"`
	OwnedResources       int    `json:"owned_resources"`
	PreexistingResources int    `json:"preexisting_resources"`
	PendingActions       int    `json:"pending_actions"`
	MissingResources     int    `json:"missing_resources"`
	ConflictingResources int    `json:"conflicting_resources"`
	LastError            string `json:"last_error,omitempty"`
}

type CaptureSnapshot struct {
	State          string `json:"state"`
	DesiredMode    string `json:"desired_mode"`
	CommittedMode  string `json:"committed_mode"`
	FaultID        string `json:"fault_id,omitempty"`
	ReadinessState string `json:"readiness_state,omitempty"`
	ReadinessError string `json:"readiness_error,omitempty"`
}

type Finding struct {
	Code         string    `json:"code"`
	Severity     Severity  `json:"severity"`
	Domain       string    `json:"domain"`
	Summary      string    `json:"summary"`
	Detail       string    `json:"detail,omitempty"`
	Ownership    Ownership `json:"ownership"`
	Recoverable  bool      `json:"recoverable"`
	Transitional bool      `json:"transitional"`
}

// MachineSnapshot is produced by the privileged Service's read-only observer.
type MachineSnapshot struct {
	Physical          PhysicalSnapshot `json:"physical"`
	TUN               TUNSnapshot      `json:"tun"`
	DNS               ResourceSnapshot `json:"dns"`
	Routes            ResourceSnapshot `json:"routes"`
	NRPT              ResourceSnapshot `json:"nrpt"`
	Firewall          ResourceSnapshot `json:"firewall"`
	Journal           JournalSnapshot  `json:"journal"`
	ObservationErrors []string         `json:"observation_errors,omitempty"`
}

type Snapshot struct {
	Version     int         `json:"version"`
	CollectedAt time.Time   `json:"collected_at"`
	Health      HealthState `json:"health"`
	Stale       bool        `json:"stale"`
	Partial     bool        `json:"partial"`

	Transition  TransitionSnapshot  `json:"transition"`
	Capture     CaptureSnapshot     `json:"capture"`
	Physical    PhysicalSnapshot    `json:"physical"`
	SystemProxy SystemProxySnapshot `json:"system_proxy"`
	TUN         TUNSnapshot         `json:"tun"`
	DNS         ResourceSnapshot    `json:"dns"`
	Routes      ResourceSnapshot    `json:"routes"`
	NRPT        ResourceSnapshot    `json:"nrpt"`
	Firewall    ResourceSnapshot    `json:"firewall"`
	Journal     JournalSnapshot     `json:"journal"`

	Findings          []Finding `json:"findings"`
	ObservationErrors []string  `json:"observation_errors,omitempty"`
}
