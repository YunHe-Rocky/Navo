// Package host defines the CoreHost interface for managing the network proxy core.
// It is the central abstraction of the Navo network management system.
package host

import (
	"context"
	"time"
)

// CoreHost manages the lifecycle of the network proxy core (sing-box).
// All network core operations go through this interface, allowing
// future replacement of the underlying proxy implementation.
type CoreHost interface {
	// ID returns the stable core identifier used by persistence and IPC.
	ID() string

	// Start launches the proxy core process.
	// configPath must be an absolute path to a valid sing-box JSON config.
	// Returns the PID of the started process.
	Start(ctx context.Context, configPath string) (pid int, err error)

	// Stop terminates the proxy core process.
	// If force is true, the process is killed immediately.
	// If force is false, the process receives an interrupt signal and
	// waits up to timeout before being forcefully killed.
	Stop(ctx context.Context, force bool, timeout time.Duration) error

	// Restart stops and restarts the proxy core with the given config.
	Restart(ctx context.Context, configPath string) (pid int, err error)

	// Reload attempts a hot-reload of the configuration without restarting.
	// Returns an error if the current implementation does not support hot reload.
	Reload(ctx context.Context, configPath string) error

	// Status returns the current state of the proxy core.
	Status() HostStatus

	// HealthCheck performs a single health check of the proxy core.
	HealthCheck(ctx context.Context) *HealthResult

	// ValidateConfig checks whether a configuration file is valid
	// by invoking the proxy core's config validation command.
	ValidateConfig(ctx context.Context, configPath string) error

	// GetLogs returns the last N lines of proxy core output.
	GetLogs(lines int) ([]string, error)

	// Reconcile performs network state cleanup before starting.
	// It detects and cleans residual routes, DNS settings, and zombie processes.
	Reconcile(ctx context.Context) (*ReconcileResult, error)
}

// HostState enumerates the possible states of the proxy core.
type HostState string

const (
	HostStateStopped   HostState = "stopped"
	HostStateStarting  HostState = "starting"
	HostStateRunning   HostState = "running"
	HostStateReloading HostState = "reloading"
	HostStateStopping  HostState = "stopping"
	HostStateFailed    HostState = "failed"
)

// HostStatus represents the current status of the proxy core.
type HostStatus struct {
	CoreID       string        `json:"core_id"`
	State        HostState     `json:"state"`
	PID          int           `json:"pid"`
	Uptime       time.Duration `json:"uptime"`
	ConfigHash   string        `json:"config_hash"`
	RestartCount int           `json:"restart_count"`
	LastError    string        `json:"last_error,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
}

// HealthResult contains the result of a single health check.
type HealthResult struct {
	Healthy   bool      `json:"healthy"`
	ProcessOK bool      `json:"process_ok"`
	PortOK    bool      `json:"port_ok"`
	LatencyMs int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"`
}

// ReconcileResult contains the result of a network reconciliation operation.
type ReconcileResult struct {
	IssuesFound   []string      `json:"issues_found"`
	IssuesFixed   []string      `json:"issues_fixed"`
	IssuesUnfixed []string      `json:"issues_unfixed"`
	RecoveryState RecoveryState `json:"recovery_state"`
}

// RecoveryState indicates the state of the last shutdown.
type RecoveryState string

const (
	RecoveryNormal  RecoveryState = "NORMAL"
	RecoveryDirty   RecoveryState = "DIRTY_SHUTDOWN"
	RecoveryRecover RecoveryState = "RECOVERING"
	RecoveryReady   RecoveryState = "READY"
)
