package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
)

func TestEnableProxyRejectsFailedEndToEndProbe(t *testing.T) {
	instance, err := New(Config{
		ProxyProbeFn: func(context.Context, string) error {
			return errors.New("proxy handshake failed")
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = instance.EnableProxy()
	if err == nil || !strings.Contains(err.Error(), "local HTTP proxy is not ready") {
		t.Fatalf("expected readiness error, got %v", err)
	}
	if instance.ProxyStatus().Enabled {
		t.Fatal("system proxy was enabled after a failed end-to-end probe")
	}
}

func TestTrayConnectRequiresActiveSelectionBeforeCoreStart(t *testing.T) {
	var calls []string
	instance, err := New(Config{
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			calls = append(calls, method)
			return map[string]interface{}{
				"type": "RESPONSE",
				"payload": map[string]interface{}{
					"mode": "rule", "active_id": "",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "tray-connect",
		"method":     "connection.enable",
	})
	if !isErrorResponse(resp) {
		t.Fatalf("expected active-selection error, got %#v", resp)
	}
	payload, _ := resp["payload"].(map[string]interface{})
	if payload["code"] != "ACTIVE_SELECTION_REQUIRED" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
	if len(calls) != 1 || calls[0] != "runtime.status" {
		t.Fatalf("core must not start without active selection: %v", calls)
	}
}

func TestRawCoreLifecycleIsNotExposedToUI(t *testing.T) {
	serviceCalled := false
	instance, err := New(Config{
		SendToServiceFn: func(map[string]interface{}) (map[string]interface{}, error) {
			serviceCalled = true
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"core.start", "core.stop", "core.restart", "service.shutdown"} {
		resp := instance.Dispatch(context.Background(), map[string]interface{}{
			"request_id": method,
			"method":     method,
		})
		if !isErrorResponse(resp) {
			t.Fatalf("%s unexpectedly exposed: %#v", method, resp)
		}
	}
	if serviceCalled {
		t.Fatal("raw lifecycle request crossed the Agent boundary")
	}
}

func TestStopIsConcurrentAndIdempotent(t *testing.T) {
	instance, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	instance.running = true

	var callers sync.WaitGroup
	for i := 0; i < 32; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			instance.Stop()
		}()
	}
	callers.Wait()

	select {
	case <-instance.stopCh:
	default:
		t.Fatal("stop channel was not closed")
	}
}

func TestCaptureTUNFailureKeepsTransactionUncommitted(t *testing.T) {
	var calls []string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			calls = append(calls, method)
			switch method {
			case "capture.prepare":
				mode, _ := msg["mode"].(string)
				if mode == "off" {
					return map[string]interface{}{
						"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
					}, nil
				}
				return map[string]interface{}{
					"type": "ERROR",
					"payload": map[string]interface{}{
						"code": "NET_005", "message": "TUN startup failed",
					},
				}, nil
			default:
				t.Fatalf("unexpected service call %q", method)
				return nil, nil
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "capture-1",
		"method":     "capture.set",
		"mode":       "tun",
	})
	if !isErrorResponse(resp) {
		t.Fatalf("expected TUN error, got %#v", resp)
	}
	if len(calls) != 2 || calls[0] != "capture.prepare" || calls[1] != "capture.prepare" {
		t.Fatalf("unexpected transaction calls: %v", calls)
	}
	status := instance.captureSnapshot()
	if status.State != capture.StateFaulted || status.CommittedMode != capture.ModeOff {
		t.Fatalf("capture did not fail safe: %#v", status)
	}
}

func TestCaptureRollbackFailureRetainsPreviousCommittedMode(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "capture.json")
	instance, err := New(Config{
		CaptureJournalPath: journalPath,
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "capture.prepare":
				return map[string]interface{}{
					"type": "ERROR",
					"payload": map[string]interface{}{
						"code": "NET_ROLLBACK_FAILED", "message": "adapter still active",
					},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE",
					"payload": map[string]interface{}{
						"name": "Navo", "state": "degraded", "interface_index": 42,
					},
				}, nil
			default:
				t.Fatalf("unexpected service call %q", msg["method"])
				return nil, nil
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	journal := capture.TransitionJournal{
		ID: "rollback-failure", From: capture.ModeSystemProxy, To: capture.ModeTUN,
		CurrentStep: capture.PhaseStartingCore, StartedAt: time.Now().UTC(),
	}
	if err := instance.captureFailure(capture.ModeTUN, journal, errors.New("startup failed")); err == nil {
		t.Fatal("expected transition and rollback failure")
	}
	status := instance.captureSnapshot()
	if status.State != capture.StateFaulted || status.CommittedMode != capture.ModeSystemProxy {
		t.Fatalf("rollback failure hid previous committed mode: %#v", status)
	}
	if status.Adapter.InterfaceIndex != 42 {
		t.Fatalf("residual adapter status missing: %#v", status.Adapter)
	}
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("rollback evidence journal must remain: %v", err)
	}
}

func TestCaptureTUNRejectsNonElevatedProcessBeforeServiceMutation(t *testing.T) {
	var calls []string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return false },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			calls = append(calls, method)
			return map[string]interface{}{
				"type":    "RESPONSE",
				"payload": map[string]interface{}{"enabled": false},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "capture-admin",
		"method":     "capture.set",
		"mode":       "tun",
	})
	if !isErrorResponse(resp) {
		t.Fatalf("expected privilege error, got %#v", resp)
	}
	payload, _ := resp["payload"].(map[string]interface{})
	if payload["code"] != "TUN_REQUIRES_ADMIN" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
	if len(calls) != 0 {
		t.Fatalf("non-admin TUN activation mutated Service: %v", calls)
	}
}

func TestCaptureTransitionRejectsConcurrentRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "capture.prepare":
				close(entered)
				<-release
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE",
					"payload": map[string]interface{}{
						"name": "Navo", "state": "missing", "interface_index": 0,
					},
				}, nil
			default:
				return nil, errors.New("unexpected service call")
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	first := make(chan error, 1)
	go func() {
		first <- instance.transitionCaptureMode(context.Background(), capture.ModeOff)
	}()
	<-entered
	if err := instance.transitionCaptureMode(context.Background(), capture.ModeSystemProxy); !errors.Is(err, errCaptureBusy) {
		t.Fatalf("expected concurrent request rejection, got %v", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first transition failed: %v", err)
	}
}

func TestCaptureTransitionHonorsCanceledContextBeforeMutation(t *testing.T) {
	serviceCalled := false
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(map[string]interface{}) (map[string]interface{}, error) {
			serviceCalled = true
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := instance.transitionCaptureMode(ctx, capture.ModeOff); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled transition, got %v", err)
	}
	if serviceCalled {
		t.Fatal("canceled transition reached Service")
	}
}

func TestLegacyTUNEnableRejectsNonElevatedProcessBeforeServiceMutation(t *testing.T) {
	serviceCalled := false
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return false },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			serviceCalled = true
			return map[string]interface{}{"type": "RESPONSE"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "legacy-tun-admin",
		"method":     "tun.enable",
	})
	if !isErrorResponse(resp) {
		t.Fatalf("expected privilege error, got %#v", resp)
	}
	payload, _ := resp["payload"].(map[string]interface{})
	if payload["code"] != "TUN_REQUIRES_ADMIN" {
		t.Fatalf("unexpected error payload: %#v", payload)
	}
	if serviceCalled {
		t.Fatal("legacy non-admin TUN activation reached Service")
	}
}

func TestStartupRecoveryClearsCorruptJournalAfterOwnedCleanup(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "capture.json")
	if err := os.WriteFile(journalPath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Config{
		CaptureJournalPath: journalPath,
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] != "capture.prepare" || msg["mode"] != "off" {
				t.Fatalf("unexpected recovery request: %#v", msg)
			}
			return map[string]interface{}{
				"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.recoverCaptureOnStartup(context.Background()); err != nil {
		t.Fatalf("safe cleanup should recover a corrupt journal: %v", err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt journal remains after successful cleanup: %v", err)
	}
	if snapshot := instance.captureSnapshot(); snapshot.State != capture.StateStopped {
		t.Fatalf("startup recovery did not settle: %#v", snapshot)
	}
}

func TestStartupRecoverySettlesToFaultWhenCleanupFails(t *testing.T) {
	journalPath := filepath.Join(t.TempDir(), "capture.json")
	instance, err := New(Config{
		CaptureJournalPath: journalPath,
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"type": "ERROR",
				"payload": map[string]interface{}{
					"code": "RECOVERY_FAILED", "message": "route cleanup failed",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.captureJournal.Save(capture.TransitionJournal{
		ID: "crashed", To: capture.ModeTUN,
		CurrentStep: capture.PhaseConfiguringRoute,
	}); err != nil {
		t.Fatal(err)
	}
	if err := instance.recoverCaptureOnStartup(context.Background()); err == nil {
		t.Fatal("failed cleanup unexpectedly reported success")
	}
	snapshot := instance.captureSnapshot()
	if snapshot.State != capture.StateFaulted || snapshot.CommittedMode != capture.ModeOff {
		t.Fatalf("failed startup cleanup remained ambiguous: %#v", snapshot)
	}
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("failed cleanup must retain its recovery journal: %v", err)
	}
}

func TestRuntimeCoreFailureFallsBackToOff(t *testing.T) {
	var offCalls int
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] == "capture.prepare" && msg["mode"] == "off" {
				offCalls++
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
				}, nil
			}
			return nil, errors.New("unexpected service request")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningSystemProxy, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeSystemProxy, CommittedMode: capture.ModeSystemProxy,
	})
	if err := instance.recoverUnhealthyCapture(
		capture.ModeSystemProxy, errors.New("core is unavailable"),
	); err == nil {
		t.Fatal("health recovery should retain the triggering fault")
	}
	snapshot := instance.captureSnapshot()
	if offCalls != 1 || snapshot.State != capture.StateFaulted ||
		snapshot.CommittedMode != capture.ModeOff {
		t.Fatalf("runtime failure did not fail closed: calls=%d snapshot=%#v", offCalls, snapshot)
	}
}
