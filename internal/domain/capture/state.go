package capture

import "time"

// State is the authoritative capture lifecycle exposed to every controller.
type State string

const (
	StateStopped             State = "stopped"
	StateStartingSystemProxy State = "starting_system_proxy"
	StateRunningSystemProxy  State = "running_system_proxy"
	StateStartingTUN         State = "starting_tun"
	StateRunningTUN          State = "running_tun"
	StateStopping            State = "stopping"
	StateRecovering          State = "recovering"
	StateFaulted             State = "faulted"
)

// Phase describes user-visible progress within a serialized transition.
type Phase string

const (
	PhaseStopped          Phase = "stopped"
	PhaseStoppingOld      Phase = "stopping_old_mode"
	PhaseRecovering       Phase = "recovering_adapter"
	PhaseStartingCore     Phase = "starting_core"
	PhaseConfiguringRoute Phase = "configuring_routes"
	PhaseChecking         Phase = "checking_connection"
	PhaseRunning          Phase = "running"
	PhaseFaulted          Phase = "faulted"
	PhaseRollingBack      Phase = "rolling_back"
)

// AdapterState normalizes Windows adapter observations.
type AdapterState string

const (
	AdapterMissing     AdapterState = "missing"
	AdapterDisabled    AdapterState = "disabled"
	AdapterEnabled     AdapterState = "enabled"
	AdapterStarting    AdapterState = "starting"
	AdapterStopping    AdapterState = "stopping"
	AdapterDriverError AdapterState = "driver_error"
	AdapterUnknown     AdapterState = "unknown"
)

// AdapterStatus uses stable Windows identity in addition to the display name.
type AdapterStatus struct {
	State          AdapterState `json:"state"`
	Name           string       `json:"name"`
	InterfaceGUID  string       `json:"interface_guid,omitempty"`
	InterfaceIndex int          `json:"interface_index,omitempty"`
	Error          string       `json:"error,omitempty"`
}

// Snapshot is the single read model consumed by UI, tray, and recovery.
type Snapshot struct {
	State         State         `json:"state"`
	Phase         Phase         `json:"phase"`
	DesiredMode   Mode          `json:"desired_mode"`
	CommittedMode Mode          `json:"committed_mode"`
	TransitionID  string        `json:"transition_id,omitempty"`
	FaultID       string        `json:"fault_id,omitempty"`
	Adapter       AdapterStatus `json:"adapter"`
	LastError     string        `json:"last_error,omitempty"`
	CanRetryTUN   bool          `json:"can_retry_tun"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func InitialSnapshot() Snapshot {
	return Snapshot{
		State: StateStopped, Phase: PhaseStopped,
		DesiredMode: ModeOff, CommittedMode: ModeOff,
		Adapter:   AdapterStatus{State: AdapterMissing},
		UpdatedAt: time.Now().UTC(),
	}
}

func StartingState(mode Mode) State {
	if mode == ModeTUN {
		return StateStartingTUN
	}
	if mode == ModeSystemProxy {
		return StateStartingSystemProxy
	}
	return StateStopping
}

func RunningState(mode Mode) State {
	if mode == ModeTUN {
		return StateRunningTUN
	}
	if mode == ModeSystemProxy {
		return StateRunningSystemProxy
	}
	return StateStopped
}
