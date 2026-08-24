package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/monitor"
	"navo/internal/network"
)

type fakeOutboundProber struct {
	result *monitor.ProbeResult
}

func (f fakeOutboundProber) ProbeTCP(context.Context, string, string, int) *monitor.ProbeResult {
	return f.result
}

type fakeTUNNetworkManager struct {
	deactivateErr error
}

func (*fakeTUNNetworkManager) Preflight(context.Context) error { return nil }
func (*fakeTUNNetworkManager) Activate(context.Context) (network.AdapterSnapshot, error) {
	return network.AdapterSnapshot{}, nil
}
func (*fakeTUNNetworkManager) Rebind(context.Context) (network.AdapterSnapshot, error) {
	return network.AdapterSnapshot{}, nil
}
func (f *fakeTUNNetworkManager) Deactivate(context.Context) error { return f.deactivateErr }
func (*fakeTUNNetworkManager) Recover(context.Context) error      { return nil }

func TestDeactivateTUNNetworkRetainsOwnershipWhenRollbackFails(t *testing.T) {
	manager := &fakeTUNNetworkManager{deactivateErr: errors.New("injected rollback failure")}
	service := &Service{networkManager: manager}
	if err := service.deactivateTUNNetwork(context.Background()); err == nil {
		t.Fatal("rollback unexpectedly succeeded")
	}
	if service.networkManager != manager {
		t.Fatal("failed rollback discarded Manager ownership")
	}
}

func TestDeactivateTUNNetworkClearsOwnershipOnlyAfterSuccess(t *testing.T) {
	service := &Service{networkManager: &fakeTUNNetworkManager{}}
	if err := service.deactivateTUNNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.networkManager != nil {
		t.Fatal("successful rollback retained Manager ownership")
	}
}

func TestUnselectedRuntimeUsesDirectTUNValidation(t *testing.T) {
	if !isUnselectedDirectRuntime("", nil) {
		t.Fatal("fresh runtime without an outbound must use direct TUN validation")
	}
	selected := &compiler.Outbound{ID: "node"}
	if isUnselectedDirectRuntime("node", selected) {
		t.Fatal("selected outbound was incorrectly classified as direct-only")
	}
	if isUnselectedDirectRuntime("stale", nil) {
		t.Fatal("stale persisted selection must fail closed")
	}
}

func TestExplicitDirectRuntimeUsesDirectTUNValidationWithSelectedOutbound(t *testing.T) {
	selected := &compiler.Outbound{ID: "node"}
	if !isDirectRuntime(runtimeModeDirect, "node", selected) {
		t.Fatal("explicit direct mode must validate the direct data plane even when an outbound remains selected")
	}
	if isDirectRuntime(runtimeModeGlobal, "node", selected) {
		t.Fatal("global mode with a selected outbound was incorrectly classified as direct")
	}
}

func TestSystemProxyPreflightRejectsUnreachableSelectedOutbound(t *testing.T) {
	err := verifyOutboundReachability(
		context.Background(),
		"dead-node",
		[]compiler.Outbound{{ID: "dead-node", Server: "203.0.113.9", Port: 8001}},
		fakeOutboundProber{result: &monitor.ProbeResult{Error: "i/o timeout"}},
	)
	var transitionErr *captureTransitionError
	if !errors.As(err, &transitionErr) {
		t.Fatalf("error type = %T, want captureTransitionError", err)
	}
	if transitionErr.code != "OUTBOUND_UNREACHABLE" {
		t.Fatalf("error code = %q", transitionErr.code)
	}
	if !strings.Contains(err.Error(), "dead-node") || !strings.Contains(err.Error(), "i/o timeout") {
		t.Fatalf("error is not actionable: %v", err)
	}
}

func TestSystemProxyPreflightAcceptsReachableSelectedOutbound(t *testing.T) {
	err := verifyOutboundReachability(
		context.Background(),
		"healthy-node",
		[]compiler.Outbound{{ID: "healthy-node", Server: "198.51.100.7", Port: 443}},
		fakeOutboundProber{result: &monitor.ProbeResult{Healthy: true}},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCaptureServiceIPCRequestTimeoutCoversHardVerification(t *testing.T) {
	if got := serviceIPCRequestTimeout(map[string]interface{}{"method": "capture.prepare"}); got != captureServiceIPCRequestTimeout {
		t.Fatalf("capture timeout = %s", got)
	}
	if got := serviceIPCRequestTimeout(map[string]interface{}{"method": "core.status"}); got != defaultServiceIPCRequestTimeout {
		t.Fatalf("default timeout = %s", got)
	}
}

func TestRuntimeVerifyWaitsForCaptureTransition(t *testing.T) {
	service := &Service{}
	service.captureMu.Lock()
	defer service.captureMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	responseCh := make(chan map[string]interface{}, 1)
	go func() {
		responseCh <- service.handleRuntimeVerify(ctx, "verify-during-transition")
	}()

	select {
	case response := <-responseCh:
		t.Fatalf("runtime.verify observed a partial capture transition: %#v", response)
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case response := <-responseCh:
		payload, _ := response["payload"].(map[string]interface{})
		if code, _ := payload["code"].(string); code != "CAPTURE_BUSY" {
			t.Fatalf("runtime.verify error code = %q, want CAPTURE_BUSY: %#v", code, response)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime.verify did not respect its context while waiting for capture transition")
	}
}

func TestResetTUNRuntimeAfterRollbackDoesNotRequireConfigCompilation(t *testing.T) {
	service := &Service{runtime: runtimeState{
		TUNEnabled: true, TUNName: "Navo", TUNMTU: 1500,
		TUNOutboundInterface: "Ethernet",
	}, tunPlan: network.TUNActivationPlan{
		SelectedOutboundID: "node-1", OriginalServerHost: "proxy.example", PinnedServerIP: "203.0.113.7",
	}}

	service.resetTUNRuntimeAfterRollback()

	if service.runtime.TUNEnabled {
		t.Fatal("rollback retained TUN-enabled runtime state")
	}
	if service.runtime.TUNOutboundInterface != "" {
		t.Fatalf("rollback retained outbound interface %q", service.runtime.TUNOutboundInterface)
	}
	if service.tunPlan.SelectedOutboundID != "" {
		t.Fatalf("rollback retained TUN activation plan %#v", service.tunPlan)
	}
}

func TestTUNHealthTrackerRequiresConsecutiveFailuresInSameSession(t *testing.T) {
	tracker := tunHealthTracker{}
	if tracker.observe("session-a", true) || tracker.observe("session-a", true) {
		t.Fatal("TUN health faulted before the confirmation threshold")
	}
	if !tracker.observe("session-a", true) {
		t.Fatal("TUN health did not fault after consecutive confirmations")
	}
	tracker.observe("session-a", false)
	if tracker.observe("session-a", true) {
		t.Fatal("healthy observation did not reset the failure streak")
	}
	if tracker.observe("session-b", true) {
		t.Fatal("a new activation session inherited an old failure streak")
	}
}

func TestTUNHealthObservationIgnoresAmbiguityAndChecksIdentity(t *testing.T) {
	expected := tunHealthExpectation{sessionID: "session-a", guid: "{AABB}", index: 27}
	for _, state := range []capture.AdapterState{
		capture.AdapterStarting, capture.AdapterStopping, capture.AdapterUnknown,
	} {
		if actionableTUNObservation(expected, capture.AdapterStatus{State: state, Error: "transient"}) {
			t.Fatalf("ambiguous state %s was treated as destructive evidence", state)
		}
	}
	healthy := capture.AdapterStatus{
		State: capture.AdapterEnabled, InterfaceGUID: "aabb", InterfaceIndex: 27,
	}
	if !tunObservationHealthy(expected, healthy) {
		t.Fatal("matching native adapter identity was not healthy")
	}
	healthy.InterfaceIndex = 28
	if !actionableTUNObservation(expected, healthy) {
		t.Fatal("enabled adapter with a different identity was accepted")
	}
	if !actionableTUNObservation(expected, capture.AdapterStatus{State: capture.AdapterMissing}) {
		t.Fatal("confirmed missing adapter was not actionable")
	}
}
