// Package network provides network state reconciliation and TUN cleanup.
package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"navo/internal/fsatomic"
	"navo/internal/host"
	"navo/internal/network/tun"
)

// Reconciler coordinates recovery state before proxy startup. Exact route,
// NRPT, and firewall mutations belong exclusively to Manager's V2 journal.
type Reconciler struct {
	stateFilePath string
	tunManager    tun.Manager
	routeManager  tun.RouteManager
	dnsManager    tun.DNSManager
}

// RecoveryStateFile is the persisted recovery state on disk.
type RecoveryStateFile struct {
	State          host.RecoveryState `json:"state"`
	LastCleanExit  time.Time          `json:"last_clean_exit,omitempty"`
	LastKnownPID   int                `json:"last_known_pid,omitempty"`
	LastListenPort int                `json:"last_listen_port,omitempty"`
	LastTUNAdapter string             `json:"last_tun_adapter,omitempty"`
	LastDNSServers []string           `json:"last_dns_servers,omitempty"`
	DirtySince     time.Time          `json:"dirty_since,omitempty"`
	CheckedAt      time.Time          `json:"checked_at,omitempty"`
}

// ReconcileConfig holds parameters for a reconciliation run.
type ReconcileConfig struct {
	ListenPort int
	TUNName    string
}

// defaultStatePath returns the default recovery state file path.
func defaultStatePath() string {
	if p := os.Getenv("NAVO_RECOVERY_STATE_PATH"); p != "" {
		return p
	}
	return filepath.Join(os.Getenv("PROGRAMDATA"), "Navo", "service", "recovery_state.json")
}

// NewReconciler creates a new Reconciler with TUN awareness.
func NewReconciler(tunMgr tun.Manager, routeMgr tun.RouteManager, dnsMgr tun.DNSManager) *Reconciler {
	return &Reconciler{
		stateFilePath: defaultStatePath(),
		tunManager:    tunMgr,
		routeManager:  routeMgr,
		dnsManager:    dnsMgr,
	}
}

// SetStateFilePath overrides the default state file path (for testing).
func (r *Reconciler) SetStateFilePath(path string) {
	r.stateFilePath = path
}

// MarkDirtyShutdown writes a DIRTY_SHUTDOWN state to disk before stopping.
func (r *Reconciler) MarkDirtyShutdown(pid int, port int, tunName string, dnsServers []string) error {
	state := RecoveryStateFile{
		State:          host.RecoveryDirty,
		LastKnownPID:   pid,
		LastListenPort: port,
		LastTUNAdapter: tunName,
		LastDNSServers: dnsServers,
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

// Reconcile performs the full network reconciliation flow.
// Steps: check recovery flag → clean TUN adapter → clean routes → reset DNS → clean stale files → mark READY
func (r *Reconciler) Reconcile(ctx context.Context, cfg *ReconcileConfig) (*host.ReconcileResult, error) {
	result := &host.ReconcileResult{}

	state, err := r.readState()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.RecoveryState = host.RecoveryNormal
			log.Printf("[reconciler] no previous state file, assuming normal")
			return result, nil
		}
		result.RecoveryState = host.RecoveryDirty
		return result, fmt.Errorf("read recovery state: %w", err)
	}

	if state.State == host.RecoveryNormal {
		result.RecoveryState = host.RecoveryNormal
		log.Printf("[reconciler] previous shutdown was normal, no cleanup needed")
		return result, nil
	}

	// DIRTY_SHUTDOWN detected — begin reconciliation
	result.RecoveryState = host.RecoveryDirty
	result.IssuesFound = append(result.IssuesFound,
		fmt.Sprintf("dirty shutdown detected (since %s, PID=%d, port=%d)",
			state.DirtySince.Format(time.RFC3339), state.LastKnownPID, state.LastListenPort))

	log.Printf("[reconciler] dirty shutdown detected, starting reconciliation")

	// Step 1: Clean up TUN adapter from previous session
	if r.tunManager != nil && state.LastTUNAdapter != "" {
		if st := r.tunManager.Status(); st.Created {
			result.IssuesFound = append(result.IssuesFound,
				fmt.Sprintf("stale TUN adapter %q detected", state.LastTUNAdapter))
			res, err := r.tunManager.Cleanup(ctx)
			if err != nil {
				result.IssuesUnfixed = append(result.IssuesUnfixed,
					fmt.Sprintf("TUN adapter cleanup failed: %v", err))
			} else {
				if res.AdapterRemoved {
					result.IssuesFixed = append(result.IssuesFixed,
						fmt.Sprintf("removed stale TUN adapter %q", state.LastTUNAdapter))
				}
			}
		}
	}

	// Routes, NRPT and firewall state were already recovered by Manager before
	// Supervisor startup. Without a V2 journal, guessing ownership is unsafe.

	// Check if the last known port is free.
	checkPort := state.LastListenPort
	if checkPort == 0 && cfg != nil {
		checkPort = cfg.ListenPort
	}
	if checkPort > 0 {
		if isPortInUse(checkPort) {
			issue := fmt.Sprintf("port %d is still in use from previous session", checkPort)
			result.IssuesFound = append(result.IssuesFound, issue)
			result.IssuesUnfixed = append(result.IssuesUnfixed, issue)
			log.Printf("[reconciler] WARNING: %s", issue)
		} else {
			result.IssuesFixed = append(result.IssuesFixed,
				fmt.Sprintf("port %d is free", checkPort))
		}
	}

	if len(result.IssuesUnfixed) > 0 {
		result.RecoveryState = host.RecoveryDirty
		return result, fmt.Errorf("network reconciliation incomplete: %s", result.IssuesUnfixed[0])
	}

	// Mark reconciliation complete
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
	log.Printf("[reconciler] reconciliation complete: found=%d fixed=%d unfixed=%d",
		len(result.IssuesFound), len(result.IssuesFixed), len(result.IssuesUnfixed))

	return result, nil
}

// readState reads the recovery state from disk.
func (r *Reconciler) readState() (*RecoveryStateFile, error) {
	if err := os.MkdirAll(filepath.Dir(r.stateFilePath), 0755); err != nil {
		return nil, err
	}
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
	if err := os.MkdirAll(filepath.Dir(r.stateFilePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal recovery state: %w", err)
	}
	return fsatomic.WriteFile(r.stateFilePath, data, 0600)
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
