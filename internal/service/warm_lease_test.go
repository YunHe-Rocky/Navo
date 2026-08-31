package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"navo/internal/host"
	"navo/internal/supervisor"
)

type warmLeaseTestHost struct {
	mu        sync.Mutex
	status    host.HostStatus
	stopCalls int
}

func (*warmLeaseTestHost) ID() string { return "sing-box" }
func (h *warmLeaseTestHost) Start(context.Context, string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = host.HostStatus{State: host.HostStateRunning, PID: 4321, ConfigHash: "host-hash"}
	return h.status.PID, nil
}
func (h *warmLeaseTestHost) Stop(context.Context, bool, time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopCalls++
	h.status = host.HostStatus{State: host.HostStateStopped}
	return nil
}
func (h *warmLeaseTestHost) Restart(ctx context.Context, path string) (int, error) {
	if err := h.Stop(ctx, false, 0); err != nil {
		return 0, err
	}
	return h.Start(ctx, path)
}
func (*warmLeaseTestHost) Reload(context.Context, string) error { return nil }
func (h *warmLeaseTestHost) Status() host.HostStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status
}
func (*warmLeaseTestHost) HealthCheck(context.Context) *host.HealthResult {
	return &host.HealthResult{Healthy: true, ProcessOK: true, PortOK: true}
}
func (*warmLeaseTestHost) ValidateConfig(context.Context, string) error { return nil }
func (*warmLeaseTestHost) GetLogs(int) ([]string, error)                { return nil, nil }
func (*warmLeaseTestHost) Reconcile(context.Context) (*host.ReconcileResult, error) {
	return &host.ReconcileResult{RecoveryState: host.RecoveryNormal}, nil
}

func newWarmLeaseTestService(t *testing.T, ttl time.Duration) (*Service, *warmLeaseTestHost) {
	t.Helper()
	h := &warmLeaseTestHost{}
	sup := supervisor.NewSupervisor(h, nil)
	if err := sup.Start(context.Background(), "runtime.json"); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		host: h, sup: sup, warmTTL: ttl, nowFn: time.Now,
		cfg: Config{ConfigPath: "runtime.json"},
		runtime: runtimeState{
			CoreID: "sing-box", SelectedOutbound: "node-1", ActiveOutbound: "node-1",
			RevisionID: "runtime-1", ConfigHash: "runtime-hash", RevisionStatus: "active",
		},
	}
	t.Cleanup(func() {
		svc.invalidateSystemProxyWarmLocked()
		if sup.State() != supervisor.StateStopped {
			_ = sup.Stop(context.Background())
		}
	})
	return svc, h
}

func TestServiceSystemProxyWarmLeaseRetainsAndResumesExactCore(t *testing.T) {
	svc, host := newWarmLeaseTestService(t, time.Minute)
	verification := RuntimeRoutingVerification{Verified: true, VerifiedAt: time.Now().UTC()}
	svc.rememberSystemProxyWarmLocked(verification)
	if _, _, ok := svc.retainSystemProxyWarmLocked(context.Background()); !ok {
		t.Fatal("healthy committed System Proxy core was not retained")
	}
	if !svc.sup.WarmIdle() {
		t.Fatal("retained core was not marked warm-idle")
	}
	got, remaining, ok := svc.resumeSystemProxyWarmLocked(context.Background())
	if !ok || !got.Verified || remaining <= 0 {
		t.Fatalf("warm resume = verified=%t remaining=%s ok=%t", got.Verified, remaining, ok)
	}
	if svc.sup.WarmIdle() || host.stopCalls != 0 {
		t.Fatalf("warm resume restarted/stopped core: warm=%t stop_calls=%d", svc.sup.WarmIdle(), host.stopCalls)
	}
}

func TestServiceSystemProxyWarmLeaseRejectsRuntimeDrift(t *testing.T) {
	svc, _ := newWarmLeaseTestService(t, time.Minute)
	svc.rememberSystemProxyWarmLocked(RuntimeRoutingVerification{Verified: true})
	if _, _, ok := svc.retainSystemProxyWarmLocked(context.Background()); !ok {
		t.Fatal("failed to establish warm lease")
	}
	svc.runtimeMu.Lock()
	svc.runtime.ConfigHash = "changed-runtime-hash"
	svc.runtimeMu.Unlock()
	if _, _, ok := svc.resumeSystemProxyWarmLocked(context.Background()); ok {
		t.Fatal("warm lease survived runtime config drift")
	}
}

func TestServiceSystemProxyWarmLeaseStopsCoreAtExpiry(t *testing.T) {
	svc, host := newWarmLeaseTestService(t, 20*time.Millisecond)
	svc.rememberSystemProxyWarmLocked(RuntimeRoutingVerification{Verified: true})
	if _, _, ok := svc.retainSystemProxyWarmLocked(context.Background()); !ok {
		t.Fatal("failed to establish expiring warm lease")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if svc.sup.State() == supervisor.StateStopped {
			if host.stopCalls != 1 {
				t.Fatalf("expired warm core stop_calls=%d, want 1", host.stopCalls)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("warm core did not stop at expiry: state=%s stop_calls=%d", svc.sup.State(), host.stopCalls)
}
