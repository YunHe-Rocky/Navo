package capture

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateMapping(t *testing.T) {
	tests := []struct {
		mode     Mode
		starting State
		running  State
	}{
		{ModeOff, StateStopping, StateStopped},
		{ModeSystemProxy, StateStartingSystemProxy, StateRunningSystemProxy},
		{ModeTUN, StateStartingTUN, StateRunningTUN},
	}
	for _, test := range tests {
		if got := StartingState(test.mode); got != test.starting {
			t.Errorf("StartingState(%s) = %s, want %s", test.mode, got, test.starting)
		}
		if got := RunningState(test.mode); got != test.running {
			t.Errorf("RunningState(%s) = %s, want %s", test.mode, got, test.running)
		}
	}
}

func TestTransitionJournalRoundTripAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	store := NewJournalStore(path)
	want := TransitionJournal{
		ID: "transition-1", From: ModeSystemProxy, To: ModeTUN,
		CurrentStep: PhaseStartingCore, StartedAt: time.Now().UTC(),
		SystemProxyBackup: map[string]any{"enabled": true},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.From != want.From || got.To != want.To {
		t.Fatalf("journal mismatch: %#v", got)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal still exists: %v", err)
	}
}
