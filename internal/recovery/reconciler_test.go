package recovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"navo/internal/host"
)

func TestNewReconciler_DefaultPath(t *testing.T) {
	r := NewReconciler("")

	if r.stateFilePath == "" {
		t.Error("stateFilePath is empty")
	}
	// Should default to TEMP/navo-recovery.json
	expected := filepath.Join(os.TempDir(), "navo-recovery.json")
	if r.stateFilePath != expected {
		t.Errorf("stateFilePath = %s, want %s", r.stateFilePath, expected)
	}
}

func TestNewReconciler_ExplicitPath(t *testing.T) {
	r := NewReconciler("C:\\custom\\path.json")

	if r.stateFilePath != "C:\\custom\\path.json" {
		t.Errorf("stateFilePath = %s", r.stateFilePath)
	}
}

func TestMarkDirtyShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)

	err := r.MarkDirtyShutdown(12345, 12080)
	if err != nil {
		t.Fatalf("MarkDirtyShutdown() error: %v", err)
	}

	// Verify file was written
	state, err := r.readState()
	if err != nil {
		t.Fatalf("readState() error: %v", err)
	}
	if state.State != host.RecoveryDirty {
		t.Errorf("State = %s, want %s", state.State, host.RecoveryDirty)
	}
	if state.LastKnownPID != 12345 {
		t.Errorf("LastKnownPID = %d, want 12345", state.LastKnownPID)
	}
	if state.LastListenPort != 12080 {
		t.Errorf("LastListenPort = %d, want 12080", state.LastListenPort)
	}
}

func TestMarkNormalExit(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)

	// First mark dirty
	r.MarkDirtyShutdown(12345, 12080)

	// Then mark normal
	err := r.MarkNormalExit()
	if err != nil {
		t.Fatalf("MarkNormalExit() error: %v", err)
	}

	state, err := r.readState()
	if err != nil {
		t.Fatalf("readState() error: %v", err)
	}
	if state.State != host.RecoveryNormal {
		t.Errorf("State = %s, want %s", state.State, host.RecoveryNormal)
	}
}

func TestReconcile_NoStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nonexistent", "recovery.json")
	r := NewReconciler(statePath)

	result, err := r.Reconcile(context.Background(), 12080)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryNormal {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryNormal)
	}
}

func TestReconcile_NormalState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)

	r.MarkNormalExit()

	result, err := r.Reconcile(context.Background(), 12080)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryNormal {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryNormal)
	}
}

func TestReconcile_DirtyShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)
	ctx := context.Background()

	r.MarkDirtyShutdown(12345, 12080)

	result, err := r.Reconcile(ctx, 12080)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryReady)
	}
	if len(result.IssuesFound) == 0 {
		t.Error("IssuesFound is empty, expected dirty shutdown issue")
	}
	if len(result.IssuesFixed) == 0 {
		t.Error("IssuesFixed is empty")
	}
}

func TestReconcile_DirtyStateWithFreePort(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)
	ctx := context.Background()

	// Use a high port that's likely free
	r.MarkDirtyShutdown(12345, 49995)

	result, err := r.Reconcile(ctx, 49995)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryReady)
	}
}

func TestReadState_NonexistentFile(t *testing.T) {
	r := NewReconciler("nonexistent_path.json")
	_, err := r.readState()
	if err == nil {
		t.Error("readState() expected error for nonexistent file")
	}
}

func TestReadState_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	os.WriteFile(statePath, []byte("not json"), 0644)

	r := NewReconciler(statePath)
	_, err := r.readState()
	if err == nil {
		t.Error("readState() expected error for corrupted file")
	}
}

func TestIsPortInUse_FreePort(t *testing.T) {
	if isPortInUse(49997) {
		t.Log("port 49997 is in use, trying another")
		if isPortInUse(49996) {
			t.Skip("no free ports available for testing")
		}
	}
}

func TestReconciler_FullCycle(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)
	ctx := context.Background()

	// Step 1: Mark dirty (simulate crash)
	if err := r.MarkDirtyShutdown(99999, 12080); err != nil {
		t.Fatalf("MarkDirtyShutdown: %v", err)
	}

	// Step 2: Reconcile
	result, err := r.Reconcile(ctx, 12080)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryReady)
	}

	// Step 3: After reconcile, state file should be READY
	state, _ := r.readState()
	if state.State != host.RecoveryReady {
		t.Errorf("State after reconcile = %s, want %s", state.State, host.RecoveryReady)
	}

	// Step 4: Mark normal exit
	if err := r.MarkNormalExit(); err != nil {
		t.Fatalf("MarkNormalExit: %v", err)
	}

	// Step 5: Next reconcile should see NORMAL
	result2, err := r.Reconcile(ctx, 12080)
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if result2.RecoveryState != host.RecoveryNormal {
		t.Errorf("RecoveryState = %s, want %s", result2.RecoveryState, host.RecoveryNormal)
	}
}

func TestReconciler_Fixture_Normal(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "recovery_states", "normal.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	os.WriteFile(statePath, data, 0644)

	r := NewReconciler(statePath)
	result, err := r.Reconcile(context.Background(), 12080)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryNormal {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryNormal)
	}
}

func TestReconciler_Fixture_DirtyShutdown(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "recovery_states", "dirty_shutdown.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	os.WriteFile(statePath, data, 0644)

	r := NewReconciler(statePath)
	result, err := r.Reconcile(context.Background(), 12080)
	if err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("RecoveryState = %s, want %s", result.RecoveryState, host.RecoveryReady)
	}
}

func TestCleanupStaleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "recovery.json")
	r := NewReconciler(statePath)

	// Create some extra files
	os.WriteFile(filepath.Join(tmpDir, "temp1.log"), []byte("log"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "temp2.cache"), []byte("cache"), 0644)

	err := r.cleanupStaleFiles()
	if err != nil {
		t.Errorf("cleanupStaleFiles() error: %v", err)
	}
}
