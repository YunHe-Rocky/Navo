package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceStartupTimeoutExceedsNetworkRollbackBudget(t *testing.T) {
	if serviceStartupTimeout <= 30*time.Second {
		t.Fatalf("service startup timeout %s does not cover network rollback", serviceStartupTimeout)
	}
}

func TestWaitForServiceStartupReady(t *testing.T) {
	ready := make(chan struct{})
	exited := make(chan error, 1)
	close(ready)
	if err := waitForServiceStartup(ready, exited, time.Second); err != nil {
		t.Fatalf("ready service returned an error: %v", err)
	}
}

func TestWaitForServiceStartupPreservesExitCause(t *testing.T) {
	ready := make(chan struct{})
	exited := make(chan error, 1)
	exited <- errors.New("recovery failed")
	err := waitForServiceStartup(ready, exited, time.Second)
	if err == nil || !strings.Contains(err.Error(), "recovery failed") {
		t.Fatalf("service exit cause was lost: %v", err)
	}
}

func TestWaitForServiceStartupIsBounded(t *testing.T) {
	ready := make(chan struct{})
	exited := make(chan error)
	started := time.Now()
	err := waitForServiceStartup(ready, exited, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected timeout result: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("startup timeout was not bounded: %s", elapsed)
	}
}
