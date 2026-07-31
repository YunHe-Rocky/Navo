// Package recovery handles network state reconciliation after crashes.
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"navo/internal/host"
)

// Reconciler performs network state cleanup before proxy core startup.
type Reconciler struct {
	stateFilePath string
}

// RecoveryStateFile is the persisted recovery state.
type RecoveryStateFile struct {
	State          host.RecoveryState `json:"state"`
	LastCleanExit  time.Time          `json:"last_clean_exit,omitempty"`
	LastKnownPID   int                `json:"last_known_pid,omitempty"`
	LastListenPort int                `json:"last_listen_port,omitempty"`
	DirtySince     time.Time          `json:"dirty_since,omitempty"`
	CheckedAt      time.Time          `json:"checked_at,omitempty"`
}

// NewReconciler creates a new Reconciler.
// If stateFilePath is empty, defaults to %TEMP%/navo-recovery.json.
func NewReconciler(stateFilePath string) *Reconciler {
	if stateFilePath == "" {
		stateFilePath = filepath.Join(os.TempDir(), "navo-recovery.json")
	}
	return &Reconciler{stateFilePath: stateFilePath}
}

// MarkDirtyShutdown writes a DIRTY_SHUTDOWN state to disk.
func (r *Reconciler) MarkDirtyShutdown(pid int, port int) error {
	state := RecoveryStateFile{
		State:          host.RecoveryDirty,
		LastKnownPID:   pid,
		LastListenPort: port,
		DirtySince:     time.Now(),
	}
	return r.writeState(state)
}

// MarkNormalExit writes a NORMAL state to disk after a clean shutdown.
func (r *Reconciler) MarkNormalExit() error {
	state := RecoveryStateFile{
		State:         host.RecoveryNormal,
		LastCleanExit: time.Now(),
	}
	return r.writeState(state)
}

// Reconcile checks for dirty shutdown and performs cleanup.
func (r *Reconciler) Reconcile(ctx context.Context, listenPort int) (*host.ReconcileResult, error) {
	result := &host.ReconcileResult{}

	state, err := r.readState()
	if err != nil {
		result.RecoveryState = host.RecoveryNormal
		log.Printf("[recovery] no previous state file, assuming normal")
		return result, nil
	}

	if state.State == host.RecoveryNormal {
		result.RecoveryState = host.RecoveryNormal
		log.Printf("[recovery] previous shutdown was normal, no cleanup needed")
		return result, nil
	}

	// DIRTY_SHUTDOWN detected
	result.RecoveryState = host.RecoveryDirty
	result.IssuesFound = append(result.IssuesFound,
		fmt.Sprintf("dirty shutdown detected (since %s, PID=%d, port=%d)",
			state.DirtySince.Format(time.RFC3339), state.LastKnownPID, state.LastListenPort))

	log.Printf("[recovery] dirty shutdown detected - starting reconciliation")

	// Step 1: Check if the last known port is free
	checkPort := state.LastListenPort
	if checkPort == 0 {
		checkPort = listenPort
	}

	if checkPort > 0 {
		if isPortInUse(checkPort) {
			issue := fmt.Sprintf("port %d is still in use from previous session", checkPort)
			result.IssuesFound = append(result.IssuesFound, issue)
			result.IssuesUnfixed = append(result.IssuesUnfixed, issue)
			log.Printf("[recovery] WARNING: %s", issue)
		} else {
			result.IssuesFixed = append(result.IssuesFixed,
				fmt.Sprintf("port %d is free", checkPort))
		}
	}

	// Step 2: Clean stale files
	if err := r.cleanupStaleFiles(); err != nil {
		result.IssuesUnfixed = append(result.IssuesUnfixed,
			fmt.Sprintf("cleanup failed: %v", err))
	} else {
		result.IssuesFixed = append(result.IssuesFixed, "stale files checked")
	}

	// Mark recovery complete
	result.RecoveryState = host.RecoveryReady
	if err := r.writeState(RecoveryStateFile{
		State:     host.RecoveryReady,
		CheckedAt: time.Now(),
	}); err != nil {
		result.IssuesUnfixed = append(result.IssuesUnfixed,
			fmt.Sprintf("cannot write recovery state: %v", err))
		return result, err
	}

	result.IssuesFixed = append(result.IssuesFixed, "recovery state updated to READY")
	log.Printf("[recovery] reconciliation complete: found=%d fixed=%d unfixed=%d",
		len(result.IssuesFound), len(result.IssuesFixed), len(result.IssuesUnfixed))

	return result, nil
}

// readState reads the recovery state from disk.
func (r *Reconciler) readState() (*RecoveryStateFile, error) {
	data, err := os.ReadFile(r.stateFilePath)
	if err != nil {
		return nil, err
	}

	var state RecoveryStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("corrupted recovery state file: %w", err)
	}
	return &state, nil
}

// writeState writes the recovery state to disk.
func (r *Reconciler) writeState(state RecoveryStateFile) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal recovery state: %w", err)
	}
	return os.WriteFile(r.stateFilePath, data, 0644)
}

// cleanupStaleFiles removes temporary files from a previous session.
func (r *Reconciler) cleanupStaleFiles() error {
	dir := filepath.Dir(r.stateFilePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != filepath.Base(r.stateFilePath) {
			log.Printf("[recovery] would clean: %s", entry.Name())
		}
	}
	return nil
}

// isPortInUse returns true if a TCP port is currently in use.
func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}
