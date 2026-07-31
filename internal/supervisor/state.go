// Package supervisor provides a management layer above CoreHost that handles
// lifecycle, state transitions, crash recovery, and config swapping.
package supervisor

import "time"

// State represents the supervisor state machine state.
type State string

const (
	StateStopped   State = "stopped"
	StateReconcile State = "reconciling"
	StateReady     State = "ready"
	StateStarting  State = "starting"
	StateRunning   State = "running"
	StateStopping  State = "stopping"
	StateFailed    State = "failed"
	StateDegraded  State = "degraded"
	StateDirty     State = "dirty"
)

// Event represents a state machine event.
type Event string

const (
	EventStart         Event = "start"
	EventStarted       Event = "started"
	EventStop          Event = "stop"
	EventStopped       Event = "stopped"
	EventCrash         Event = "crash"
	EventRecovered     Event = "recovered"
	EventFail          Event = "fail"
	EventDegrade       Event = "degrade"
	EventRecover       Event = "recover"
	EventConfigSwap    Event = "config_swap"
	EventDirtyDetected Event = "dirty_detected"
	EventReconcileDone Event = "reconcile_done"
)

// Transition represents a valid state transition.
type Transition struct {
	From State
	Event Event
	To   State
}

// ValidTransitions defines all allowed state transitions.
var ValidTransitions = []Transition{
	// Normal lifecycle
	{StateStopped, EventStart, StateReconcile},
	{StateReconcile, EventReconcileDone, StateReady},
	{StateReady, EventStart, StateStarting},
	{StateStarting, EventStarted, StateRunning},
	{StateRunning, EventStop, StateStopping},
	{StateStopping, EventStopped, StateStopped},
	{StateRunning, EventConfigSwap, StateStopping}, // swap: stop current, then start new

	// Crash recovery
	{StateRunning, EventCrash, StateDirty},
	{StateDirty, EventReconcileDone, StateReady},
	{StateDirty, EventStart, StateReconcile},

	// Degradation
	{StateRunning, EventDegrade, StateDegraded},
	{StateDegraded, EventRecover, StateRunning},
	{StateDegraded, EventCrash, StateDirty},

	// Failure
	{StateStarting, EventFail, StateFailed},
	{StateFailed, EventStart, StateReconcile},
	{StateFailed, EventStop, StateStopped},

	// Force stop from any non-terminal state
	{StateStarting, EventStop, StateStopping},
	{StateDegraded, EventStop, StateStopping},
	{StateFailed, EventStop, StateStopped},
}

// transitionMap maps (from, event) to (to, valid).
var transitionMap = buildTransitionMap()

func buildTransitionMap() map[State]map[Event]State {
	m := make(map[State]map[Event]State)
	for _, t := range ValidTransitions {
		if m[t.From] == nil {
			m[t.From] = make(map[Event]State)
		}
		m[t.From][t.Event] = t.To
	}
	return m
}

// IsValidTransition returns whether a transition is valid.
func IsValidTransition(from State, event Event) bool {
	events, ok := transitionMap[from]
	if !ok {
		return false
	}
	_, ok = events[event]
	return ok
}

// NextState returns the next state for a given transition, or empty string if invalid.
func NextState(from State, event Event) State {
	events, ok := transitionMap[from]
	if !ok {
		return ""
	}
	return events[event]
}

// ── Event Types ──

// StateEvent represents a state transition event with metadata.
type StateEvent struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Event     Event     `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SupervisorStatus provides a snapshot of supervisor state.
type SupervisorStatus struct {
	State         State     `json:"state"`
	PID           int       `json:"pid"`
	Uptime        time.Duration `json:"uptime"`
	ConfigHash    string    `json:"config_hash"`
	RestartCount  int       `json:"restart_count"`
	LastError     string    `json:"last_error,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	StateSince    time.Time `json:"state_since"`    // when current state was entered
	TotalRestarts int       `json:"total_restarts"` // lifetime restarts
}
