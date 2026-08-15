package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
	"navo/internal/logstore"
)

func TestAgentErrorLogsActionableCodeAndReason(t *testing.T) {
	if err := logstore.Configure(""); err != nil {
		t.Fatal(err)
	}
	response := agentError("ui-test-1", "TUN_ADAPTER_NOT_READY", errors.New("adapter readiness timed out"))
	if response["type"] != "ERROR" {
		t.Fatalf("response = %#v", response)
	}
	entries := logstore.Default().Query(logstore.Query{}).Entries
	if len(entries) != 1 {
		t.Fatalf("entries = %#v", entries)
	}
	entry := entries[0]
	if entry.Message != "request failed: TUN_ADAPTER_NOT_READY" || entry.Fields["reason"] != "adapter readiness timed out" {
		t.Fatalf("entry = %#v", entry)
	}
}

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

func TestProxyReadinessReportsEveryFailedEndpoint(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "upstream unavailable", http.StatusBadGateway)
	}))
	defer proxyServer.Close()

	err := probeHTTPProxy(
		context.Background(),
		strings.TrimPrefix(proxyServer.URL, "http://"),
	)
	if err == nil {
		t.Fatal("HTTP 502 proxy unexpectedly passed readiness")
	}
	var readinessErr *proxyReadinessError
	if !errors.As(err, &readinessErr) {
		t.Fatalf("error type = %T, want proxyReadinessError: %v", err, err)
	}
	for _, endpoint := range []string{
		"connectivitycheck.gstatic.com",
		"cp.cloudflare.com",
		"www.msftconnecttest.com",
	} {
		if !strings.Contains(err.Error(), endpoint) {
			t.Fatalf("readiness error omitted %s: %v", endpoint, err)
		}
	}
}

func TestCaptureIPCRequestTimeoutCoversHardVerification(t *testing.T) {
	if got := agentIPCRequestTimeout(map[string]interface{}{"method": "capture.set"}); got != captureIPCRequestTimeout {
		t.Fatalf("capture timeout = %s", got)
	}
	if got := agentIPCRequestTimeout(map[string]interface{}{"method": "runtime.rules.set"}); got != captureIPCRequestTimeout {
		t.Fatalf("routing-rule timeout = %s", got)
	}
	if got := agentIPCRequestTimeout(map[string]interface{}{"method": "runtime.list_mode.set"}); got != captureIPCRequestTimeout {
		t.Fatalf("runtime.list_mode.set timeout = %v, want %v", got, captureIPCRequestTimeout)
	}
	if got := agentIPCRequestTimeout(map[string]interface{}{"method": "core.select"}); got != coreSwitchIPCRequestTimeout {
		t.Fatalf("core.select timeout = %v, want %v", got, coreSwitchIPCRequestTimeout)
	}
	if got := agentIPCRequestTimeout(map[string]interface{}{"method": "core.status"}); got != defaultIPCRequestTimeout {
		t.Fatalf("default timeout = %s", got)
	}
}

func TestCaptureRecoveryCancelsProductionDispatch(t *testing.T) {
	dispatchExited := make(chan struct{})
	instance, err := New(Config{
		SendToServiceContextFn: func(ctx context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			defer close(dispatchExited)
			if msg["method"] != "capture.prepare" || msg["mode"] != "off" {
				t.Fatalf("unexpected recovery request: %#v", msg)
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := instance.prepareServiceCaptureRecovery(ctx); err == nil {
		t.Fatal("canceled recovery unexpectedly succeeded")
	}
	select {
	case <-dispatchExited:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Service dispatch outlived canceled recovery")
	}
}

func TestSendToServiceContextCancelsWhileTransportIsBusy(t *testing.T) {
	instance, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	instance.serviceMu.Lock()
	defer instance.serviceMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = instance.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": "busy-transport", "method": "core.status",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("busy transport error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("busy transport ignored caller cancellation for %v", elapsed)
	}
}

func TestSendToServiceContextOwnsServiceRequestIdentity(t *testing.T) {
	var serviceRequestIDs []string
	instance, err := New(Config{
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			requestID, _ := msg["request_id"].(string)
			serviceRequestIDs = append(serviceRequestIDs, requestID)
			return map[string]interface{}{
				"request_id": requestID,
				"type":       "RESPONSE",
				"payload":    map[string]interface{}{"method": msg["method"]},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	const parentRequestID = "ui-1786631794855-13"
	for _, method := range []string{"core.status", "core.select"} {
		request := map[string]interface{}{
			"request_id": parentRequestID,
			"method":     method,
		}
		response, sendErr := instance.SendToServiceContext(context.Background(), request)
		if sendErr != nil {
			t.Fatalf("send %s: %v", method, sendErr)
		}
		if response["request_id"] != parentRequestID {
			t.Fatalf("response request_id = %q, want parent %q", response["request_id"], parentRequestID)
		}
		if request["request_id"] != parentRequestID {
			t.Fatalf("caller request was mutated: %#v", request)
		}
	}
	if len(serviceRequestIDs) != 2 {
		t.Fatalf("service request IDs = %v", serviceRequestIDs)
	}
	if serviceRequestIDs[0] == parentRequestID || serviceRequestIDs[1] == parentRequestID {
		t.Fatalf("Agent leaked UI identity to Service: %v", serviceRequestIDs)
	}
	if serviceRequestIDs[0] == serviceRequestIDs[1] {
		t.Fatalf("different Service requests reused identity %q", serviceRequestIDs[0])
	}
}

func TestCoreSwitchRevalidatesCaptureAndRestoresPreviousCoreOnFailure(t *testing.T) {
	var calls []string
	var tunStarts int
	currentCore := "sing-box"
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			if method == "core.select" {
				currentCore = msg["core_id"].(string)
				calls = append(calls, method+":"+currentCore)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active": currentCore},
				}, nil
			}
			calls = append(calls, method)
			switch method {
			case "core.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"core_id": currentCore},
				}, nil
			case "capture.prepare":
				mode, _ := msg["mode"].(string)
				if mode == "tun" {
					tunStarts++
					if tunStarts == 1 {
						return map[string]interface{}{
							"type": "ERROR", "payload": map[string]interface{}{
								"code": "TUN_DNS_VERIFY_FAILED", "message": "new core DNS failed",
							},
						}, nil
					}
				}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": mode},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected method %s", method)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningTUN, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeTUN, CommittedMode: capture.ModeTUN,
	})

	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "switch-core", "method": "core.select", "core_id": "mihomo",
	})
	if !isErrorResponse(response) || responseCode(response) != "CORE_SWITCH_VERIFY_FAILED" {
		t.Fatalf("response = %#v", response)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.State != capture.StateRunningTUN {
		t.Fatalf("previous capture was not restored: %#v", snapshot)
	}
	want := []string{
		"core.status", "capture.prepare", "tun.status", "core.select:mihomo", "capture.prepare",
		"capture.prepare", "core.status", "core.select:sing-box", "core.status", "capture.prepare", "capture.prepare", "tun.status",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestCoreSwitchLostResponseReconcilesActualCoreBeforeRestoringCapture(t *testing.T) {
	currentCore := "sing-box"
	var calls []string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			switch method {
			case "core.status":
				calls = append(calls, method+":"+currentCore)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"core_id": currentCore},
				}, nil
			case "core.select":
				target := msg["core_id"].(string)
				calls = append(calls, method+":"+target)
				currentCore = target
				if target == "mihomo" {
					return nil, errors.New("core.select response was lost")
				}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active": target},
				}, nil
			case "capture.prepare":
				mode := msg["mode"].(string)
				calls = append(calls, method+":"+mode)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": mode},
				}, nil
			case "tun.status":
				calls = append(calls, method)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected method %s", method)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningTUN, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeTUN, CommittedMode: capture.ModeTUN,
	})

	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "switch-core-lost-response", "method": "core.select", "core_id": "mihomo",
	})
	if !isErrorResponse(response) {
		t.Fatalf("response = %#v", response)
	}
	if currentCore != "sing-box" {
		t.Fatalf("ambiguous switch left core %q active", currentCore)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.State != capture.StateRunningTUN {
		t.Fatalf("previous capture was not restored: %#v", snapshot)
	}
	want := []string{
		"core.status:sing-box", "capture.prepare:off", "tun.status", "core.select:mihomo",
		"core.status:mihomo", "core.select:sing-box", "core.status:sing-box",
		"capture.prepare:tun", "tun.status",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestOutboundSwitchStopsAndRestoresTUNCapture(t *testing.T) {
	activeID := "old-node"
	var calls []string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			switch method {
			case "runtime.status":
				calls = append(calls, method+":"+activeID)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active_id": activeID},
				}, nil
			case "outbound.select":
				activeID = msg["id"].(string)
				calls = append(calls, method+":"+activeID)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active_id": activeID},
				}, nil
			case "capture.prepare":
				mode := msg["mode"].(string)
				calls = append(calls, method+":"+mode)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": mode},
				}, nil
			case "tun.status":
				calls = append(calls, method)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected method %s", method)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningTUN, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeTUN, CommittedMode: capture.ModeTUN,
	})

	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "switch-outbound", "method": "outbound.select", "id": "new-node",
	})
	if isErrorResponse(response) {
		t.Fatalf("response = %#v", response)
	}
	if activeID != "new-node" {
		t.Fatalf("active outbound = %q", activeID)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.State != capture.StateRunningTUN {
		t.Fatalf("TUN capture was not restored: %#v", snapshot)
	}
	want := []string{
		"runtime.status:old-node", "capture.prepare:off", "tun.status",
		"outbound.select:new-node", "capture.prepare:tun", "tun.status",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
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
					"mode": "bypass_mainland", "active_id": "",
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
	payload, _ := resp["payload"].(map[string]interface{})
	if payload["code"] != "NET_005" {
		t.Fatalf("Service TUN error code was lost: %#v", payload)
	}
	if len(calls) != 2 || calls[0] != "capture.prepare" || calls[1] != "capture.prepare" {
		t.Fatalf("unexpected transaction calls: %v", calls)
	}
	status := instance.captureSnapshot()
	if status.State != capture.StateFaulted || status.CommittedMode != capture.ModeOff {
		t.Fatalf("capture did not fail safe: %#v", status)
	}
}

func TestCaptureTransitionSameHealthyModeIsIdempotent(t *testing.T) {
	serviceCalled := false
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(map[string]interface{}) (map[string]interface{}, error) {
			serviceCalled = true
			return nil, errors.New("unexpected service call")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningTUN, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeTUN, CommittedMode: capture.ModeTUN,
	})

	if err := instance.transitionCaptureMode(context.Background(), capture.ModeTUN); err != nil {
		t.Fatal(err)
	}
	if serviceCalled {
		t.Fatal("idempotent TUN request restarted the Service transaction")
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

func TestCaptureTransitionSerializesConcurrentRequest(t *testing.T) {
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
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningSystemProxy, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeSystemProxy, CommittedMode: capture.ModeSystemProxy,
	})

	first := make(chan error, 1)
	go func() {
		first <- instance.transitionCaptureMode(context.Background(), capture.ModeOff)
	}()
	<-entered
	second := make(chan error, 1)
	go func() {
		second <- instance.transitionCaptureMode(context.Background(), capture.ModeOff)
	}()
	select {
	case err := <-second:
		t.Fatalf("concurrent request returned before the active transition: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first transition failed: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("serialized transition failed: %v", err)
	}
}

func TestTUNHealthIgnoresTransientSupervisorState(t *testing.T) {
	var methods []string
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			methods = append(methods, method)
			if method != "tun.status" {
				t.Fatalf("TUN health queried transient core state through %q", method)
			}
			return map[string]interface{}{
				"type":    "RESPONSE",
				"payload": map[string]interface{}{"state": "enabled"},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.captureHealthError(capture.ModeTUN); err != nil {
		t.Fatalf("verified TUN was treated as unhealthy: %v", err)
	}
	if len(methods) != 1 || methods[0] != "tun.status" {
		t.Fatalf("unexpected health calls: %v", methods)
	}
}

func TestAgentMirrorsServiceTUNFaultWithoutStartingSecondRollback(t *testing.T) {
	var methods []string
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			method, _ := msg["method"].(string)
			methods = append(methods, method)
			return map[string]interface{}{
				"type": "RESPONSE",
				"payload": map[string]interface{}{
					"fault_id": "service-fault-1", "last_error": "confirmed missing adapter",
					"name": "Navo", "state": "missing",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRunningTUN, Phase: capture.PhaseRunning,
		DesiredMode: capture.ModeTUN, CommittedMode: capture.ModeTUN,
	})

	instance.mirrorServiceTUNFault()

	if len(methods) != 1 || methods[0] != "tun.status" {
		t.Fatalf("Agent initiated a competing TUN recovery: %v", methods)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.State != capture.StateFaulted || snapshot.CommittedMode != capture.ModeOff ||
		snapshot.FaultID != "service-fault-1" || !snapshot.CanRetryTUN {
		t.Fatalf("Service fault was not mirrored faithfully: %#v", snapshot)
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

func TestFaultedCaptureRecoversBeforeNewTransition(t *testing.T) {
	var modes []string
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		ProxyProbeFn: func(context.Context, string) error {
			return errors.New("probe intentionally blocked")
		},
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] == "capture.prepare" {
				modes = append(modes, msg["mode"].(string))
				return map[string]interface{}{
					"type":    "RESPONSE",
					"payload": map[string]interface{}{"mode": msg["mode"]},
				}, nil
			}
			return map[string]interface{}{"type": "RESPONSE", "payload": map[string]interface{}{}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateFaulted, Phase: capture.PhaseFaulted,
		DesiredMode: capture.ModeSystemProxy, CommittedMode: capture.ModeOff,
	})
	err = instance.transitionCaptureMode(context.Background(), capture.ModeSystemProxy)
	if err == nil || !strings.Contains(err.Error(), "probe intentionally blocked") {
		t.Fatalf("unexpected transition result: %v", err)
	}
	want := []string{"off", "system_proxy", "off"}
	if !reflect.DeepEqual(modes, want) {
		t.Fatalf("capture calls=%v, want recovery-first sequence %v", modes, want)
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
