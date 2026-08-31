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
	"navo/internal/connection"

	"navo/internal/domain/capture"

	"navo/internal/logstore"
	"navo/internal/selfheal"
	"navo/internal/startup"
)

type fakeStartupController struct {
	settings       startup.Settings
	configureCalls int
	enabled        bool
	mode           string
}

func (f *fakeStartupController) Status(context.Context) (startup.Settings, error) {
	return f.settings, nil
}

func (f *fakeStartupController) Configure(_ context.Context, enabled bool, mode string) (startup.Settings, error) {
	f.configureCalls++
	f.enabled = enabled
	f.mode = mode
	f.settings.Enabled = enabled
	f.settings.Mode = mode
	f.settings.Registered = enabled
	return f.settings, nil
}

func TestStartupSetRequiresSelectedRouteForProxyPolicy(t *testing.T) {
	controller := &fakeStartupController{settings: startup.Settings{Supported: true, Mode: startup.ModeSystemProxy}}
	instance, err := New(Config{
		StartupController: controller,
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"request_id": msg["request_id"], "type": "RESPONSE",
				"payload": map[string]interface{}{"mode": "bypass_mainland", "selected_id": ""},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.handleStartupSet(context.Background(), "startup-no-route", map[string]interface{}{
		"enabled": true, "mode": startup.ModeSystemProxy,
	})
	if !isErrorResponse(response) || responseCode(response) != "OUTBOUND_REQUIRED" {
		t.Fatalf("response = %#v", response)
	}
	if controller.configureCalls != 0 {
		t.Fatalf("startup task configured without a route: %d calls", controller.configureCalls)
	}
}

func TestStartupSetPersistsExplicitCaptureMode(t *testing.T) {
	controller := &fakeStartupController{settings: startup.Settings{Supported: true, Mode: startup.ModeSystemProxy}}
	instance, err := New(Config{
		StartupController: controller,
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"request_id": msg["request_id"], "type": "RESPONSE",
				"payload": map[string]interface{}{"mode": "global", "selected_id": "route-1"},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.handleStartupSet(context.Background(), "startup-tun", map[string]interface{}{
		"enabled": true, "mode": startup.ModeTUN,
	})
	if isErrorResponse(response) {
		t.Fatalf("response = %#v", response)
	}
	if controller.configureCalls != 1 || !controller.enabled || controller.mode != startup.ModeTUN {
		t.Fatalf("startup configuration = calls:%d enabled:%t mode:%q", controller.configureCalls, controller.enabled, controller.mode)
	}
}

func TestStartupRestoreDoesNothingWhenDisabled(t *testing.T) {
	controller := &fakeStartupController{settings: startup.Settings{
		Supported: true, Enabled: false, Mode: startup.ModeSystemProxy,
	}}
	instance, err := New(Config{StartupController: controller})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.restoreStartupConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if controller.configureCalls != 0 {
		t.Fatalf("disabled startup mutated configuration: %d calls", controller.configureCalls)
	}
}

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

func TestOutboundSwitchFailureRestoresVerifiedActiveAndTUNCapture(t *testing.T) {
	activeID := "old-node"
	policyMode := "global"
	listMode := "blacklist"
	blacklist := []interface{}{"blocked.example"}
	whitelist := []interface{}{"direct.example"}
	var selections []string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"active_id": activeID, "selected_id": activeID,
						"mode": policyMode, "list_mode": listMode,
						"blacklist": blacklist, "whitelist": whitelist,
					},
				}, nil
			case "outbound.select":
				activeID = msg["id"].(string)
				selections = append(selections, activeID)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active_id": activeID},
				}, nil
			case "capture.prepare":
				mode := msg["mode"].(string)
				if mode == capture.ModeTUN.String() && activeID == "new-node" {
					return map[string]interface{}{
						"type": "ERROR", "payload": map[string]interface{}{
							"code": "APPLICATION_READINESS_FAILED", "message": "new node HTTPS probe failed",
						},
					}, nil
				}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": mode,
						"verification": map[string]interface{}{
							"verified": true,
							"sites": map[string]interface{}{
								"google": map[string]interface{}{
									"dns": true, "tcp": true, "https": true, "status_code": 204,
								},
							},
						},
					},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected method %v", msg["method"])
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
		"request_id": "switch-fails", "method": "outbound.select", "id": "new-node",
	})
	if !isErrorResponse(response) || responseCode(response) != "OUTBOUND_SWITCH_VERIFY_FAILED" {
		t.Fatalf("response = %#v", response)
	}
	if activeID != "old-node" || !reflect.DeepEqual(selections, []string{"new-node", "old-node"}) {
		t.Fatalf("active=%q selections=%v", activeID, selections)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.State != capture.StateRunningTUN {
		t.Fatalf("previous TUN capture was not restored: %#v", snapshot)
	}
	if policyMode != "global" || listMode != "blacklist" || !reflect.DeepEqual(blacklist, []interface{}{"blocked.example"}) || !reflect.DeepEqual(whitelist, []interface{}{"direct.example"}) {
		t.Fatalf("routing policy changed during node rollback")
	}
}

func TestAgentDoesNotTreatCandidateAsActive(t *testing.T) {
	instance, err := New(Config{
		SendToServiceContextFn: func(context.Context, map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"type": "RESPONSE", "payload": map[string]interface{}{
					"active_id": "", "selected_id": "candidate-node", "candidate_id": "candidate-node",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := instance.activeOutboundID(context.Background())
	if err != nil || activeID != "" {
		t.Fatalf("active outbound = %q, err=%v", activeID, err)
	}
}

func TestTrayConnectUsesTUNPrimaryCapture(t *testing.T) {
	var preparedMode string
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"selected_id": "node-a"},
				}, nil
			case "capture.prepare":
				preparedMode, _ = msg["mode"].(string)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": preparedMode,
						"verification": map[string]interface{}{
							"verified": true,
							"sites": map[string]interface{}{
								"google": map[string]interface{}{
									"dns": true, "tcp": true, "https": true, "status_code": 204,
								},
							},
						},
					},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "tray-connect-tun", "method": "connection.enable",
	})
	if isErrorResponse(resp) {
		t.Fatalf("response = %#v", resp)
	}
	if preparedMode != capture.ModeTUN.String() {
		t.Fatalf("prepared capture mode = %q, want TUN", preparedMode)
	}
	if got := instance.captureSnapshot().CommittedMode; got != capture.ModeTUN {
		t.Fatalf("committed capture mode = %q, want TUN", got)
	}
}

func TestTrayConnectPropagatesCanceledRequestContext(t *testing.T) {
	serviceCalls := 0
	instance, err := New(Config{
		SendToServiceContextFn: func(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
			serviceCalls++
			return nil, ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp := instance.dispatchUI(ctx, map[string]interface{}{
		"request_id": "tray-connect-canceled", "method": "connection.enable",
	})
	if !isErrorResponse(resp) {
		t.Fatalf("expected canceled response, got %#v", resp)
	}
	if serviceCalls != 1 {
		t.Fatalf("service calls = %d, want only request-scoped runtime observation", serviceCalls)
	}
	if got := instance.captureSnapshot().CommittedMode; got != capture.ModeOff {
		t.Fatalf("canceled request mutated capture mode to %q", got)
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
	var tunCalls int
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
				tunCalls++
				return map[string]interface{}{
					"type": "ERROR",
					"payload": map[string]interface{}{
						"code": "NET_005", "message": "TUN startup failed",
					},
				}, nil
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active_id": ""},
				}, nil
			case "core.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"core_id": "sing-box"},
				}, nil
			case "network.recover":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{},
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
	if tunCalls != 1+selfheal.MaxRepairRounds {
		t.Fatalf("TUN activation attempts = %d, want %d; calls=%v", tunCalls, 1+selfheal.MaxRepairRounds, calls)
	}
	status := instance.captureSnapshot()
	if status.State != capture.StateFaulted || status.CommittedMode != capture.ModeOff {
		t.Fatalf("capture did not fail safe: %#v", status)
	}
	report := instance.recoverySnapshot()
	if !report.Exhausted || report.Recovered || len(report.Rounds) != selfheal.MaxRepairRounds {
		t.Fatalf("activation recovery report = %#v", report)
	}
}

func TestCaptureMissingOutboundDoesNotEnterAutomaticRepair(t *testing.T) {
	var activationCalls int
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] != "capture.prepare" {
				t.Fatalf("unexpected service call: %#v", msg)
			}
			if msg["mode"] == "off" {
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
				}, nil
			}
			activationCalls++
			return map[string]interface{}{
				"type": "ERROR", "payload": map[string]interface{}{
					"code": "OUTBOUND_REQUIRED", "message": "select an available proxy route",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "capture-no-outbound", "method": "capture.set", "mode": "system_proxy",
	})
	if !isErrorResponse(response) || responseCode(response) != "OUTBOUND_REQUIRED" {
		t.Fatalf("response = %#v", response)
	}
	if activationCalls != 1 {
		t.Fatalf("missing outbound activation attempts = %d, want 1", activationCalls)
	}
	if report := instance.recoverySnapshot(); report.State != selfheal.RecoveryIdle || len(report.Rounds) != 0 {
		t.Fatalf("configuration error entered automatic repair: %#v", report)
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

func TestConnectionAdmissionResponsePreservesSupersededAndBusyCodes(t *testing.T) {
	superseded := connectionAdmissionResponse("superseded", "CAPTURE_BUSY", connection.ErrSuperseded)
	if responseCode(superseded) != "REQUEST_SUPERSEDED" {
		t.Fatalf("superseded response = %#v", superseded)
	}
	busy := connectionAdmissionResponse("busy", "CAPTURE_BUSY", connection.ErrBusy)
	if responseCode(busy) != "CAPTURE_BUSY" {
		t.Fatalf("busy response = %#v", busy)
	}
}

func TestExplicitSystemProxyDisableRequestsWarmRetention(t *testing.T) {
	var retainWarm interface{}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "capture.prepare":
				retainWarm = msg["retain_warm"]
				return map[string]interface{}{"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"}}, nil
			case "tun.status":
				return map[string]interface{}{"type": "RESPONSE", "payload": map[string]interface{}{"name": "Navo", "state": "missing"}}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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
	if err := instance.transitionCaptureMode(context.Background(), capture.ModeOff); err != nil {
		t.Fatal(err)
	}
	if retainWarm != true {
		t.Fatalf("ordinary system-proxy disable retain_warm = %#v, want true", retainWarm)
	}
}

func TestCaptureFailureRollbackForcesColdServiceCleanup(t *testing.T) {
	var offRequest map[string]interface{}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] != "capture.prepare" || msg["mode"] != "off" {
				return nil, fmt.Errorf("unexpected service message %#v", msg)
			}
			offRequest = msg
			return map[string]interface{}{"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	journal := capture.TransitionJournal{
		ID: "warm-rollback", From: capture.ModeSystemProxy, To: capture.ModeTUN,
		CurrentStep: capture.PhaseStartingCore, StartedAt: time.Now().UTC(),
	}
	_ = instance.captureFailure(capture.ModeTUN, journal, errors.New("activation failed"))
	if offRequest == nil {
		t.Fatal("capture failure did not request Service cleanup")
	}
	if retain, supplied := offRequest["retain_warm"]; supplied || retain == true {
		t.Fatalf("failure rollback retained warm core: %#v", offRequest)
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

func TestAgentDoesNotMirrorServiceTUNFaultIntoSystemProxyFailure(t *testing.T) {
	instance, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateFaulted, Phase: capture.PhaseFaulted,
		DesiredMode: capture.ModeSystemProxy, CommittedMode: capture.ModeOff,
		FaultID: "system-proxy-fault", LastError: "proxy route unavailable",
	})

	instance.refreshCaptureFault(map[string]interface{}{
		"fault_id": "stale-tun-fault", "last_error": "TUN data plane failed",
	})

	snapshot := instance.captureSnapshot()
	if snapshot.DesiredMode != capture.ModeSystemProxy || snapshot.FaultID != "system-proxy-fault" || snapshot.CanRetryTUN {
		t.Fatalf("System Proxy fault was rewritten as TUN: %#v", snapshot)
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
	systemProxyCalls := 0
	for _, mode := range modes {
		if mode == "system_proxy" {
			systemProxyCalls++
		}
	}
	if len(modes) < 3 || modes[0] != "off" || modes[1] != "system_proxy" ||
		modes[len(modes)-1] != "off" || systemProxyCalls != 1+selfheal.MaxRepairRounds {
		t.Fatalf("capture calls=%v, want recovery-first then %d bounded attempts", modes, 1+selfheal.MaxRepairRounds)
	}
	report := instance.recoverySnapshot()
	if !report.Exhausted || report.Recovered || len(report.Rounds) != selfheal.MaxRepairRounds {
		t.Fatalf("activation recovery report = %#v", report)
	}
}

func TestRuntimeCoreFailureRunsTwoRoundsThenFallsBackToOff(t *testing.T) {
	var offCalls, activationCalls int
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceFn: func(msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] == "capture.prepare" {
				switch msg["mode"] {
				case "off":
					offCalls++
					return map[string]interface{}{
						"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
					}, nil
				case "system_proxy":
					activationCalls++
					return map[string]interface{}{
						"type": "ERROR", "error": map[string]interface{}{
							"code": "CORE_UNAVAILABLE", "message": "core restart failed",
						},
					}, nil
				}
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
	report := instance.recoverySnapshot()
	if activationCalls != selfheal.MaxRepairRounds || offCalls < selfheal.MaxRepairRounds ||
		snapshot.State != capture.StateFaulted || snapshot.CommittedMode != capture.ModeOff {
		t.Fatalf("runtime failure did not fail closed: off=%d activation=%d snapshot=%#v", offCalls, activationCalls, snapshot)
	}
	if !report.Exhausted || report.Recovered || len(report.Rounds) != selfheal.MaxRepairRounds ||
		report.Evidence.Domain != selfheal.FaultDomainCore {
		t.Fatalf("recovery report = %#v", report)
	}
}

func TestSameChannelFailoverCommitsOnlyAfterFullReadiness(t *testing.T) {
	selectedID := "node-a"
	verification := map[string]interface{}{
		"verified":    true,
		"verified_at": time.Now().UTC(),
		"sites": map[string]interface{}{
			"chatgpt-web":    map[string]interface{}{"dns": true, "tcp": true, "https": true},
			"openai-auth":    map[string]interface{}{"dns": true, "tcp": true, "https": true},
			"openai-api":     map[string]interface{}{"dns": true, "tcp": true, "https": true},
			"openai-assets":  map[string]interface{}{"dns": true, "tcp": true, "https": true},
			"chatgpt-stream": map[string]interface{}{"dns": true, "tcp": true, "https": true},
		},
	}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		IsElevatedFn:       func() bool { return true },
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "outbound.failover_candidates":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"source_type": "airport_subscription",
						"candidates": []interface{}{map[string]interface{}{
							"outbound_id": "node-b", "source_type": "airport_subscription",
							"latency_ms": 18, "reachable": true,
						}},
						"rejected": []interface{}{},
					},
				}, nil
			case "outbound.select":
				selectedID, _ = msg["id"].(string)
				return map[string]interface{}{"type": "RESPONSE", "payload": map[string]interface{}{"active_id": selectedID}}, nil
			case "capture.prepare":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": msg["mode"], "verification": verification,
					},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"verification": verification},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateStopped, Phase: capture.PhaseStopped,
		DesiredMode: capture.ModeOff, CommittedMode: capture.ModeOff,
	})
	transaction, err := instance.coordinator.TryBegin(connection.Request{
		Operation: connection.OperationSelfHeal, Origin: connection.OriginSelfHeal,
		FaultDomain: string(selfheal.FaultDomainNode),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Finish(nil)
	report := selfheal.RecoveryReport{ID: "failover-test", StartedAt: time.Now().UTC()}
	if err := instance.attemptSameChannelFailover(context.Background(), transaction, capture.ModeTUN, "node-a", &report); err != nil {
		t.Fatal(err)
	}
	if selectedID != "node-b" || !report.Recovered || report.State != selfheal.RecoveryRecovered || len(report.Candidates) != 1 || !report.Candidates[0].Verified {
		t.Fatalf("failover did not commit verified candidate: selected=%q report=%#v", selectedID, report)
	}
	if snapshot := instance.captureSnapshot(); snapshot.CommittedMode != capture.ModeTUN || snapshot.Readiness.State != "ready" {
		t.Fatalf("capture snapshot = %#v", snapshot)
	}
}
func TestPolicyMutationWaitsForActiveCaptureTransaction(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	policyCalled := make(chan struct{}, 1)
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "capture.prepare":
				close(entered)
				<-release
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"state": "missing"},
				}, nil
			case "runtime.mode.set":
				policyCalled <- struct{}{}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "global"},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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

	captureDone := make(chan error, 1)
	go func() {
		captureDone <- instance.transitionCaptureMode(context.Background(), capture.ModeOff)
	}()
	<-entered
	snapshot := instance.coordinator.Snapshot()
	if !snapshot.Busy || snapshot.Operation != connection.OperationCaptureSwitch ||
		snapshot.Phase != connection.PhaseApplying {
		t.Fatalf("capture transaction snapshot = %#v", snapshot)
	}

	policyDone := make(chan map[string]interface{}, 1)
	go func() {
		policyDone <- instance.dispatchUI(context.Background(), map[string]interface{}{
			"request_id": "policy-after-capture",
			"method":     "runtime.mode.set",
			"mode":       "global",
		})
	}()
	select {
	case <-policyCalled:
		t.Fatal("policy mutation bypassed the active capture transaction")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-captureDone; err != nil {
		t.Fatalf("capture transition failed: %v", err)
	}
	response := <-policyDone
	if isErrorResponse(response) {
		t.Fatalf("policy mutation failed: %#v", response)
	}
	select {
	case <-policyCalled:
	default:
		t.Fatal("policy mutation was not forwarded after capture completed")
	}
}

func TestActivePolicyNoOpSkipsCurrentUserProbe(t *testing.T) {
	probeCalls := 0
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		CaptureRouteProbeFn: func(context.Context, capture.Mode, string) error {
			probeCalls++
			return nil
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "global", "list_mode": "off", "blacklist": []string{}, "whitelist": []string{},
					},
				}, nil
			case "runtime.list_mode.set":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "off", "verified": true, "changed": false,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "list-mode-noop", "method": "runtime.list_mode.set", "mode": "off",
	})
	if isErrorResponse(response) || probeCalls != 0 {
		t.Fatalf("no-op response=%#v probe_calls=%d", response, probeCalls)
	}
}
func TestSystemProxyDirectPolicyUsesRouteAwareCurrentUserProbe(t *testing.T) {
	verification := map[string]interface{}{
		"verified": true,
		"sites": map[string]interface{}{
			"baidu": map[string]interface{}{
				"dns": true, "tcp": true, "https": true, "status_code": 200,
			},
		},
	}
	var probedMode capture.Mode
	var probedRuntimeMode string
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		CaptureProbeFn: func(context.Context, capture.Mode) error {
			t.Fatal("legacy capture probe was used despite route-aware configuration")
			return nil
		},
		CaptureRouteProbeFn: func(_ context.Context, mode capture.Mode, runtimeMode string) error {
			probedMode, probedRuntimeMode = mode, runtimeMode
			return nil
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "global", "list_mode": "off",
						"blacklist": []string{}, "whitelist": []string{},
					},
				}, nil
			case "runtime.mode.set":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": msg["mode"]},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "direct", "list_mode": "off", "verification": verification,
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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

	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "system-proxy-direct-policy",
		"method":     "runtime.mode.set",
		"mode":       "direct",
	})
	if isErrorResponse(response) {
		t.Fatalf("direct policy mutation failed: %#v", response)
	}
	if probedMode != capture.ModeSystemProxy || probedRuntimeMode != "direct" {
		t.Fatalf("route-aware probe = (%s, %q)", probedMode, probedRuntimeMode)
	}
	readiness := instance.captureSnapshot().Readiness
	if readiness.State != "ready" || readiness.Scope != "direct" || !readiness.DefaultProxy {
		t.Fatalf("direct readiness = %#v", readiness)
	}
}

func TestActivePolicyMutationVerifiesCurrentUserCapture(t *testing.T) {
	probeCalls := 0
	verification := map[string]interface{}{
		"verified": true,
		"sites": map[string]interface{}{
			"google": map[string]interface{}{
				"dns": true, "tcp": true, "https": true, "status_code": 204,
			},
		},
	}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		CaptureProbeFn: func(_ context.Context, mode capture.Mode) error {
			probeCalls++
			if mode != capture.ModeTUN {
				t.Fatalf("probe mode = %s, want TUN", mode)
			}
			return nil
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "bypass_mainland", "list_mode": "off",
						"blacklist": []string{"google.com"}, "whitelist": []string{"example.cn"},
					},
				}, nil
			case "runtime.list_mode.set":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": msg["mode"]},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"verification": verification},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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
		"request_id": "policy-host-verify",
		"method":     "runtime.list_mode.set",
		"mode":       "blacklist",
	})
	if isErrorResponse(response) {
		t.Fatalf("policy mutation failed: %#v", response)
	}
	if probeCalls != 1 {
		t.Fatalf("current-user probe calls = %d, want 1", probeCalls)
	}
	if readiness := instance.captureSnapshot().Readiness; readiness.State != "ready" {
		t.Fatalf("capture readiness was not refreshed: %#v", readiness)
	}
}

func TestActivePolicyMutationRestoresPreviousPolicyAfterHostFailure(t *testing.T) {
	probeCalls := 0
	var listModes []string
	verification := map[string]interface{}{
		"verified": true,
		"sites": map[string]interface{}{
			"google": map[string]interface{}{
				"dns": true, "tcp": true, "https": true, "status_code": 204,
			},
		},
	}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		CaptureProbeFn: func(_ context.Context, mode capture.Mode) error {
			probeCalls++
			if mode != capture.ModeTUN {
				t.Fatalf("probe mode = %s, want TUN", mode)
			}
			if probeCalls == 1 {
				return errors.New("current-user TUN path failed after policy swap")
			}
			return nil
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "bypass_mainland", "list_mode": "off",
						"blacklist": []interface{}{"google.com"}, "whitelist": []interface{}{"example.cn"},
					},
				}, nil
			case "runtime.list_mode.set":
				listModes = append(listModes, msg["mode"].(string))
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": msg["mode"]},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"verification": verification},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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
		"request_id": "policy-host-rollback",
		"method":     "runtime.list_mode.set",
		"mode":       "whitelist",
	})
	if !isErrorResponse(response) || responseCode(response) != "POLICY_READINESS_FAILED" {
		t.Fatalf("response = %#v", response)
	}
	if !reflect.DeepEqual(listModes, []string{"whitelist", "off"}) {
		t.Fatalf("list mode calls = %#v, want mutation then exact rollback", listModes)
	}
	if probeCalls != 2 {
		t.Fatalf("current-user probe calls = %d, want failed mutation plus restored policy", probeCalls)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.Readiness.State != "ready" {
		t.Fatalf("restored capture snapshot = %#v", snapshot)
	}
}

func TestActivePolicyMutationFailsClosedWhenRollbackCannotBeProven(t *testing.T) {
	listModeCalls := 0
	verification := map[string]interface{}{
		"verified": true,
		"sites": map[string]interface{}{
			"google": map[string]interface{}{
				"dns": true, "tcp": true, "https": true, "status_code": 204,
			},
		},
	}
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		CaptureProbeFn: func(_ context.Context, mode capture.Mode) error {
			return fmt.Errorf("current-user %s path is unavailable", mode)
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": "bypass_mainland", "list_mode": "off",
						"blacklist": []string{"google.com"}, "whitelist": []string{"example.cn"},
					},
				}, nil
			case "runtime.list_mode.set":
				listModeCalls++
				if listModeCalls == 2 {
					return map[string]interface{}{
						"type": "ERROR", "payload": map[string]interface{}{"code": "ROLLBACK_FAILED", "message": "rollback rejected"},
					}, nil
				}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": msg["mode"]},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"verification": verification},
				}, nil
			case "capture.prepare":
				if msg["mode"] != "off" {
					t.Fatalf("fail-closed mode = %v, want off", msg["mode"])
				}
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"mode": "off"},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"name": "Navo", "state": "missing"},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
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
		"request_id": "policy-host-fail-closed",
		"method":     "runtime.list_mode.set",
		"mode":       "blacklist",
	})
	if !isErrorResponse(response) || responseCode(response) != "POLICY_READINESS_ROLLBACK_FAILED" {
		t.Fatalf("response = %#v", response)
	}
	if snapshot := instance.captureSnapshot(); snapshot.CommittedMode != capture.ModeOff || snapshot.State != capture.StateStopped {
		t.Fatalf("capture did not fail closed: %#v", snapshot)
	}
}

func TestRestoreRuntimePolicyUsesExactMutationBoundary(t *testing.T) {
	previous := runtimePolicySnapshot{
		Mode:      "bypass_mainland",
		ListMode:  "whitelist",
		Blacklist: []string{"proxy.example", "203.0.113.0/24"},
		Whitelist: []string{"direct.example", "198.51.100.0/24"},
	}
	tests := []struct {
		method string
		check  func(*testing.T, map[string]interface{})
	}{
		{
			method: "runtime.mode.set",
			check: func(t *testing.T, request map[string]interface{}) {
				if request["mode"] != previous.Mode {
					t.Fatalf("restored mode = %#v", request["mode"])
				}
			},
		},
		{
			method: "runtime.list_mode.set",
			check: func(t *testing.T, request map[string]interface{}) {
				if request["mode"] != previous.ListMode {
					t.Fatalf("restored list mode = %#v", request["mode"])
				}
			},
		},
		{
			method: "runtime.rules.set",
			check: func(t *testing.T, request map[string]interface{}) {
				if !reflect.DeepEqual(request["blacklist"], previous.Blacklist) ||
					!reflect.DeepEqual(request["whitelist"], previous.Whitelist) {
					t.Fatalf("restored rules = %#v", request)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			var restored map[string]interface{}
			instance, err := New(Config{
				CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
				SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
					restored = msg
					return map[string]interface{}{
						"type": "RESPONSE", "payload": map[string]interface{}{},
					}, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.restoreRuntimePolicy(
				context.Background(), map[string]interface{}{"method": test.method}, previous,
			); err != nil {
				t.Fatal(err)
			}
			if restored["method"] != test.method || restored["request_id"] == "" {
				t.Fatalf("rollback request boundary = %#v", restored)
			}
			test.check(t, restored)
		})
	}
}

func TestCaptureReadinessFromServicePayload(t *testing.T) {
	checkedAt := time.Date(2026, time.August, 17, 1, 2, 3, 0, time.UTC)
	completeSites := map[string]interface{}{
		"openai-api": map[string]interface{}{
			"dns": true, "tcp": true, "https": true, "status_code": 401,
		},
	}
	tests := []struct {
		name      string
		mode      capture.Mode
		payload   map[string]interface{}
		wantState string
	}{
		{
			name: "system proxy verified flag", mode: capture.ModeSystemProxy,
			payload: map[string]interface{}{"verification": map[string]interface{}{
				"verified": true, "verified_at": checkedAt, "sites": completeSites,
			}},
			wantState: "ready",
		},
		{
			name: "TUN infers readiness from complete site evidence", mode: capture.ModeTUN,
			payload: map[string]interface{}{"verification": map[string]interface{}{
				"verified_at": checkedAt, "sites": completeSites,
			}},
			wantState: "ready",
		},
		{
			name: "missing evidence", mode: capture.ModeSystemProxy,
			payload: map[string]interface{}{}, wantState: "unverified",
		},
		{
			name: "incomplete evidence", mode: capture.ModeTUN,
			payload: map[string]interface{}{"verification": map[string]interface{}{
				"verified_at": checkedAt,
				"sites": map[string]interface{}{"chatgpt-web": map[string]interface{}{
					"dns": true, "tcp": false, "https": false,
				}},
			}},
			wantState: "failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := captureReadinessFromServicePayload(test.payload, test.mode)
			if got.State != test.wantState {
				t.Fatalf("readiness = %#v", got)
			}
			if test.wantState == "ready" && (got.CheckedAt != checkedAt || len(got.Sites) != 1) {
				t.Fatalf("ready evidence = %#v", got)
			}
		})
	}
	if got := captureReadinessFromServicePayload(map[string]interface{}{}, capture.ModeOff); got.State != "" {
		t.Fatalf("off readiness = %#v", got)
	}
}

func TestCaptureRouteProbePrefersModeAwareRules(t *testing.T) {
	legacyCalled := false
	var gotMode capture.Mode
	var gotRuntimeMode string
	instance := &Agent{
		captureRouteProbe: func(_ context.Context, mode capture.Mode, runtimeMode string) error {
			gotMode, gotRuntimeMode = mode, runtimeMode
			return nil
		},
		captureProbe: func(context.Context, capture.Mode) error {
			legacyCalled = true
			return nil
		},
	}
	if err := instance.probeCaptureRoute(context.Background(), capture.ModeSystemProxy, "global"); err != nil {
		t.Fatal(err)
	}
	if legacyCalled || gotMode != capture.ModeSystemProxy || gotRuntimeMode != "global" {
		t.Fatalf("route probe = mode=%s runtime=%q legacy=%t", gotMode, gotRuntimeMode, legacyCalled)
	}

	legacyMode := capture.ModeOff
	instance.captureRouteProbe = nil
	instance.captureProbe = func(_ context.Context, mode capture.Mode) error {
		legacyMode = mode
		return nil
	}
	if err := instance.probeCaptureRoute(context.Background(), capture.ModeTUN, "bypass_mainland"); err != nil {
		t.Fatal(err)
	}
	if legacyMode != capture.ModeTUN {
		t.Fatalf("legacy fallback mode = %s, want TUN", legacyMode)
	}
}

func TestTUNReadinessFailsWhenCurrentUserHostPathFails(t *testing.T) {
	var probedModes []capture.Mode
	instance, err := New(Config{
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		CaptureProbeFn: func(_ context.Context, mode capture.Mode) error {
			probedModes = append(probedModes, mode)
			return errors.New("current-user TUN path timed out")
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			if msg["method"] != "runtime.verify" {
				t.Fatalf("unexpected service method %q", msg["method"])
			}
			return map[string]interface{}{
				"type": "RESPONSE",
				"payload": map[string]interface{}{
					"verification": map[string]interface{}{
						"verified": true,
						"sites": map[string]interface{}{
							"google": map[string]interface{}{
								"dns": true, "tcp": true, "https": true, "status_code": 204,
							},
						},
					},
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

	readiness, verifyErr := instance.verifyCaptureReadiness(context.Background())
	if verifyErr == nil || !strings.Contains(verifyErr.Error(), "current-user TUN path timed out") {
		t.Fatalf("current-user host failure was not propagated: %v", verifyErr)
	}
	if len(probedModes) != 1 || probedModes[0] != capture.ModeTUN {
		t.Fatalf("current-user probes = %v, want [tun]", probedModes)
	}
	if readiness.State != "failed" || instance.captureSnapshot().Readiness.State != "failed" {
		t.Fatalf("failed host path was not persisted: %#v", readiness)
	}
}

func TestFailedTUNActivationSelfHealsAfterUserTransactionReleases(t *testing.T) {
	probeCalls := 0
	verification := map[string]interface{}{
		"verified": true,
		"sites": map[string]interface{}{
			"google": map[string]interface{}{
				"dns": true, "tcp": true, "https": true, "status_code": 204,
			},
		},
	}
	instance, err := New(Config{
		IsElevatedFn:       func() bool { return true },
		CaptureJournalPath: filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:       systemproxy.NewManagerWithDirectory(t.TempDir()),
		CaptureProbeFn: func(_ context.Context, mode capture.Mode) error {
			if mode != capture.ModeTUN {
				t.Fatalf("probe mode = %s, want TUN", mode)
			}
			probeCalls++
			if probeCalls == 1 {
				return errors.New("current-user direct/TUN HTTPS timeout")
			}
			return nil
		},
		SendToServiceContextFn: func(_ context.Context, msg map[string]interface{}) (map[string]interface{}, error) {
			switch msg["method"] {
			case "capture.prepare":
				mode, _ := msg["mode"].(string)
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"mode": mode, "verification": verification,
					},
				}, nil
			case "tun.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{
						"name": "Navo", "state": "enabled", "identifier": "owned-tun", "interface_index": 12,
					},
				}, nil
			case "runtime.verify":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"verification": verification},
				}, nil
			case "runtime.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"active_id": "node-a"},
				}, nil
			case "core.status":
				return map[string]interface{}{
					"type": "RESPONSE", "payload": map[string]interface{}{"core_id": "sing-box"},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected service method %v", msg["method"])
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.transitionCaptureMode(context.Background(), capture.ModeTUN); err != nil {
		t.Fatal(err)
	}
	if probeCalls != 3 {
		t.Fatalf("current-user TUN probe calls = %d, want activation failure + repaired activation + verification", probeCalls)
	}
	report := instance.recoverySnapshot()
	if report.State != selfheal.RecoveryRecovered || !report.Recovered || len(report.Rounds) != 1 {
		t.Fatalf("activation recovery report = %#v", report)
	}
	if report.Evidence.Domain != selfheal.FaultDomainNode || report.Rounds[0].Action != selfheal.ActionReapplyCapture {
		t.Fatalf("activation recovery attribution = %#v", report)
	}
	snapshot := instance.captureSnapshot()
	if snapshot.CommittedMode != capture.ModeTUN || snapshot.Readiness.State != "ready" {
		t.Fatalf("repaired capture snapshot = %#v", snapshot)
	}
}

func TestClassifyCaptureFaultSeparatesHostTimeoutFromAdapterFailure(t *testing.T) {
	domain, code, _ := classifyCaptureFault(
		capture.ModeTUN, errors.New(
			"capture data-plane check: current-user direct/TUN ChatGPT route openai-api failed: "+
				"InternetOpenUrlW(https://api.openai.com/v1/models): winapi error #12002",
		),
	)
	if domain != selfheal.FaultDomainNode || code != selfheal.CodeNodeUnavailable {
		t.Fatalf("host-path failure classified as domain=%s code=%s", domain, code)
	}
	domain, code, _ = classifyCaptureFault(capture.ModeTUN, errors.New("TUN adapter is unavailable"))
	if domain != selfheal.FaultDomainTUN || code != selfheal.CodeTUNAdapterMissing {
		t.Fatalf("adapter failure classified as domain=%s code=%s", domain, code)
	}
}

type coreUpdateTestService struct {
	mu      sync.Mutex
	methods []string
}

func (s *coreUpdateTestService) send(
	_ context.Context,
	msg map[string]interface{},
) (map[string]interface{}, error) {
	method, _ := msg["method"].(string)
	s.mu.Lock()
	s.methods = append(s.methods, method)
	s.mu.Unlock()
	switch method {
	case "core.status":
		return map[string]interface{}{
			"type": "RESPONSE", "payload": map[string]interface{}{
				"core_id": "sing-box", "state": "running",
			},
		}, nil
	case "core.update.stop", "core.update.start":
		return map[string]interface{}{
			"type": "RESPONSE", "payload": map[string]interface{}{"ok": true},
		}, nil
	case "runtime.verify":
		return map[string]interface{}{
			"type": "RESPONSE", "payload": map[string]interface{}{
				"verification": map[string]interface{}{
					"verified": true,
					"sites": map[string]interface{}{
						"google": map[string]interface{}{
							"dns": true, "tcp": true, "https": true, "status_code": 204,
						},
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unexpected service method %s", method)
	}
}

func (s *coreUpdateTestService) count(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, item := range s.methods {
		if item == method {
			count++
		}
	}
	return count
}

func newCoreUpdateTestAgent(
	t *testing.T,
	timeout time.Duration,
) (*Agent, *coreUpdateTestService) {
	t.Helper()
	service := &coreUpdateTestService{}
	instance, err := New(Config{
		CoreUpdateSessionTimeout: timeout,
		CaptureJournalPath:       filepath.Join(t.TempDir(), "capture.json"),
		ProxyManager:             systemproxy.NewManagerWithDirectory(t.TempDir()),
		SendToServiceContextFn:   service.send,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateStopped, Phase: capture.PhaseStopped,
		DesiredMode: capture.ModeOff, CommittedMode: capture.ModeOff,
	})
	return instance, service
}

func beginCoreUpdateTestSession(t *testing.T, instance *Agent, requestID string) string {
	t.Helper()
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": requestID, "method": "core.update.begin", "core_id": "sing-box",
	})
	if isErrorResponse(response) {
		t.Fatalf("begin response = %#v", response)
	}
	payload, _ := response["payload"].(map[string]interface{})
	sessionID, _ := payload["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("begin payload = %#v", payload)
	}
	return sessionID
}

func TestCoreUpdateSessionSerializesWholeMutationWindow(t *testing.T) {
	instance, service := newCoreUpdateTestAgent(t, time.Minute)
	sessionID := beginCoreUpdateTestSession(t, instance, "update-window")
	if snapshot := instance.coordinator.Snapshot(); !snapshot.Busy || snapshot.Operation != connection.OperationCoreUpdate {
		t.Fatalf("coordinator snapshot = %#v", snapshot)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	blocked := instance.dispatchUI(ctx, map[string]interface{}{
		"request_id": "interleaved-policy", "method": "runtime.mode.set", "mode": "global",
	})
	if !isErrorResponse(blocked) || responseCode(blocked) != "CONNECTION_BUSY" {
		t.Fatalf("interleaved mutation response = %#v", blocked)
	}

	committed := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "update-commit", "method": "core.update.commit",
		"session_id": sessionID, "core_id": "sing-box",
	})
	if isErrorResponse(committed) {
		t.Fatalf("commit response = %#v", committed)
	}
	snapshot := instance.coordinator.Snapshot()
	if snapshot.Busy || snapshot.LastOperation != connection.OperationCoreUpdate || snapshot.LastPhase != connection.PhaseCompleted {
		t.Fatalf("coordinator after commit = %#v", snapshot)
	}
	if service.count("core.update.stop") != 1 || service.count("core.update.start") != 1 || service.count("runtime.verify") != 1 {
		t.Fatalf("core update service methods = %v", service.methods)
	}
}

func TestCoreUpdateRollbackRestartsCoreAndReleasesCoordinator(t *testing.T) {
	instance, service := newCoreUpdateTestAgent(t, time.Minute)
	sessionID := beginCoreUpdateTestSession(t, instance, "update-rollback")
	response := instance.dispatchUI(context.Background(), map[string]interface{}{
		"request_id": "rollback", "method": "core.update.rollback",
		"session_id": sessionID, "core_id": "sing-box", "reason": "file replacement failed",
	})
	if isErrorResponse(response) {
		t.Fatalf("rollback response = %#v", response)
	}
	snapshot := instance.coordinator.Snapshot()
	if snapshot.Busy || snapshot.LastPhase != connection.PhaseFailed || !strings.Contains(snapshot.LastError, "file replacement failed") {
		t.Fatalf("coordinator after rollback = %#v", snapshot)
	}
	if service.count("core.update.start") != 1 {
		t.Fatalf("old core was not restarted: %v", service.methods)
	}
}

func TestCoreUpdateRawStepsAreNotExposedToUI(t *testing.T) {
	instance, _ := newCoreUpdateTestAgent(t, time.Minute)
	for _, method := range []string{"core.update.stop", "core.update.start"} {
		response := instance.dispatchUI(context.Background(), map[string]interface{}{
			"request_id": method, "method": method, "core_id": "sing-box",
		})
		if !isErrorResponse(response) || responseCode(response) != "CORE_UPDATE_SESSION_REQUIRED" {
			t.Fatalf("%s response = %#v", method, response)
		}
	}
}

func TestCoreUpdateSessionTimeoutRecoversAndReleasesCoordinator(t *testing.T) {
	instance, service := newCoreUpdateTestAgent(t, 30*time.Millisecond)
	_ = beginCoreUpdateTestSession(t, instance, "update-timeout")
	deadline := time.Now().Add(2 * time.Second)
	for instance.coordinator.Snapshot().Busy && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	snapshot := instance.coordinator.Snapshot()
	if snapshot.Busy || snapshot.LastPhase != connection.PhaseFailed || !strings.Contains(snapshot.LastError, "expired") {
		t.Fatalf("coordinator after timeout = %#v", snapshot)
	}
	if service.count("core.update.start") != 1 {
		t.Fatalf("timeout did not restart the core: %v", service.methods)
	}
}
