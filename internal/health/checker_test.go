package health

import (
	"context"
	"testing"
	"time"

	"navo/internal/host"
)

// mockHost implements host.CoreHost for testing.
type mockHost struct {
	healthResult *host.HealthResult
	statusResult host.HostStatus
}

func (m *mockHost) ID() string { return "mock" }

func (m *mockHost) Start(ctx context.Context, configPath string) (int, error) { return 0, nil }
func (m *mockHost) Stop(ctx context.Context, force bool, timeout time.Duration) error {
	return nil
}
func (m *mockHost) Restart(ctx context.Context, configPath string) (int, error) { return 0, nil }
func (m *mockHost) Reload(ctx context.Context, configPath string) error         { return nil }
func (m *mockHost) Status() host.HostStatus                                     { return m.statusResult }
func (m *mockHost) HealthCheck(ctx context.Context) *host.HealthResult {
	if m.healthResult != nil {
		return m.healthResult
	}
	return &host.HealthResult{Healthy: true, ProcessOK: true, PortOK: true, LatencyMs: 5}
}
func (m *mockHost) ValidateConfig(ctx context.Context, configPath string) error { return nil }
func (m *mockHost) GetLogs(lines int) ([]string, error)                         { return nil, nil }
func (m *mockHost) Reconcile(ctx context.Context) (*host.ReconcileResult, error) {
	return &host.ReconcileResult{RecoveryState: host.RecoveryNormal}, nil
}

func TestNewChecker(t *testing.T) {
	mock := &mockHost{}
	c := NewChecker(mock, 10*time.Second, nil)

	if c.host == nil {
		t.Error("host is nil")
	}
	if c.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", c.interval)
	}
	if c.lastResult != nil {
		t.Error("lastResult should be nil initially")
	}
}

func TestChecker_StartStop(t *testing.T) {
	mock := &mockHost{}
	c := NewChecker(mock, 100*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Start(ctx)

	// Wait for at least one check
	time.Sleep(150 * time.Millisecond)

	c.Stop()

	result := c.LastResult()
	if result == nil {
		t.Error("LastResult() is nil after start")
	} else if !result.Healthy {
		t.Errorf("expected healthy check, got: %v", result.Error)
	}
}

func TestChecker_UnhealthyCallback(t *testing.T) {
	mock := &mockHost{
		healthResult: &host.HealthResult{
			Healthy: false,
			Error:   "test error",
		},
	}

	called := make(chan struct{}, 1)
	onUnhealthy := func(r *host.HealthResult) {
		called <- struct{}{}
	}

	c := NewChecker(mock, 50*time.Millisecond, onUnhealthy)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Start(ctx)

	select {
	case <-called:
		// Callback fired as expected
	case <-time.After(2 * time.Second):
		t.Fatal("onUnhealthy callback was not called")
	}

	c.Stop()
}

func TestChecker_DoubleStart(t *testing.T) {
	mock := &mockHost{}
	c := NewChecker(mock, time.Hour, nil)

	ctx := context.Background()
	c.Start(ctx)
	c.Start(ctx) // Second start should be no-op

	c.Stop()

	// Should not panic or deadlock
}

func TestChecker_StopWithoutStart(t *testing.T) {
	mock := &mockHost{}
	c := NewChecker(mock, time.Hour, nil)

	c.Stop() // Should not panic
}

func TestChecker_ContextCancelStopsChecker(t *testing.T) {
	mock := &mockHost{}
	c := NewChecker(mock, 50*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Context cancellation should stop the checker without panic
	// No assertion needed — just verifying no goroutine leak or panic
}

func TestChecker_LastResultPersists(t *testing.T) {
	mock := &mockHost{
		healthResult: &host.HealthResult{
			Healthy:   true,
			ProcessOK: true,
			PortOK:    true,
			LatencyMs: 42,
		},
	}
	c := NewChecker(mock, time.Hour, nil)
	c.check(context.Background())

	result := c.LastResult()
	if result == nil || result.LatencyMs != 42 {
		t.Errorf("LastResult() LatencyMs = %d, want 42", result.LatencyMs)
	}
}
