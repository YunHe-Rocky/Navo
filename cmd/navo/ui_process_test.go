package main

import (
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestUIProcessHelper(t *testing.T) {
	if os.Getenv("NAVO_UI_PROCESS_HELPER") != "1" {
		return
	}
	select {}
}

func TestUIProcessManagerRestartsExitedUI(t *testing.T) {
	var starts atomic.Int32
	manager := &uiProcessManager{
		focus:  func() bool { return false },
		events: make(chan error, 2),
		start: func() (*managedProcess, error) {
			starts.Add(1)
			command := exec.Command(os.Args[0], "-test.run=TestUIProcessHelper")
			command.Env = append(os.Environ(), "NAVO_UI_PROCESS_HELPER=1")
			return startManagedProcess(command, t.TempDir(), io.Discard, true)
		},
	}

	if err := manager.Show(); err != nil {
		t.Fatal(err)
	}
	manager.Stop(3 * time.Second)
	waitForUIManagerState(t, manager, false)

	if err := manager.Show(); err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 2 {
		t.Fatalf("start count = %d, want 2", got)
	}
	manager.Stop(3 * time.Second)
	waitForUIManagerState(t, manager, false)
}

func TestFocusStartedUIStopsAfterWindowIsFound(t *testing.T) {
	var attempts atomic.Int32
	process := &managedProcess{done: make(chan struct{})}
	focusStartedUI(process, func() bool {
		return attempts.Add(1) >= 2
	}, time.Second)
	if got := attempts.Load(); got != 2 {
		t.Fatalf("focus attempts = %d, want 2", got)
	}
}

func waitForUIManagerState(t *testing.T, manager *uiProcessManager, running bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		manager.mu.Lock()
		actual := manager.process != nil
		manager.mu.Unlock()
		if actual == running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("UI running state did not become %t", running)
}
