package supervisor

import (
	"context"
	"testing"
	"time"

	"navo/internal/host"
)

// mockCoreHost is a minimal CoreHost for testing the supervisor state machine.
type mockCoreHost struct {
	startErr     error
	stopErr      error
	status       host.HostStatus
	healthResult *host.HealthResult
	lastForce    bool
	startCtx     context.Context
}

func (m *mockCoreHost) ID() string { return "mock" }

func (m *mockCoreHost) Start(ctx context.Context, configPath string) (int, error) {
	m.startCtx = ctx
	if m.startErr != nil {
		return 0, m.startErr
	}
	m.status.State = host.HostStateRunning
	m.status.PID = 12345
	return 12345, nil
}

func (m *mockCoreHost) Stop(ctx context.Context, force bool, timeout time.Duration) error {
	m.lastForce = force
	m.status.State = host.HostStateStopped
	m.status.PID = 0
	return m.stopErr
}

func (m *mockCoreHost) Restart(ctx context.Context, configPath string) (int, error) {
	m.Stop(ctx, false, 0)
	return m.Start(ctx, configPath)
}

func (m *mockCoreHost) Reload(ctx context.Context, configPath string) error {
	return nil
}

func (m *mockCoreHost) Status() host.HostStatus {
	return m.status
}

func (m *mockCoreHost) HealthCheck(ctx context.Context) *host.HealthResult {
	if m.healthResult != nil {
		return m.healthResult
	}
	return &host.HealthResult{Healthy: true, ProcessOK: true, PortOK: true}
}

func (m *mockCoreHost) ValidateConfig(ctx context.Context, configPath string) error {
	return nil
}

func (m *mockCoreHost) GetLogs(lines int) ([]string, error) {
	return nil, nil
}

func (m *mockCoreHost) Reconcile(ctx context.Context) (*host.ReconcileResult, error) {
	return &host.ReconcileResult{RecoveryState: host.RecoveryNormal}, nil
}

func TestNewSupervisor(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	if s.State() != StateStopped {
		t.Errorf("initial state = %s, want %s", s.State(), StateStopped)
	}
	if len(s.backoff) != 3 {
		t.Errorf("backoff length = %d, want 3", len(s.backoff))
	}
	if s.maxRestarts != 3 {
		t.Errorf("maxRestarts = %d, want 3", s.maxRestarts)
	}
}

func TestSupervisor_Start(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Start(ctx, "config.json")
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if s.State() != StateRunning {
		t.Errorf("state after start = %s, want %s", s.State(), StateRunning)
	}

	status := s.Status()
	if status.PID != 12345 {
		t.Errorf("PID = %d, want 12345", status.PID)
	}
}

func TestSupervisor_Stop(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx := context.Background()
	s.Start(ctx, "config.json")

	err := s.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if s.State() != StateStopped {
		t.Errorf("state after stop = %s, want %s", s.State(), StateStopped)
	}
}

func TestSupervisor_StopWhenNotRunning(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	err := s.Stop(context.Background())
	if err == nil {
		t.Error("Stop() expected error when not running")
	}
}

func TestSupervisorForceStopsIntermediateRecoveryState(t *testing.T) {
	mock := &mockCoreHost{status: host.HostStatus{State: host.HostStateFailed}}
	s := NewSupervisor(mock, nil)
	s.setState(StateDirty)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() from dirty recovery state: %v", err)
	}
	if s.State() != StateStopped || !mock.lastForce {
		t.Fatalf("intermediate state was not force-stopped: state=%s force=%v", s.State(), mock.lastForce)
	}
}

func TestCrashRestartKeepsConsecutiveBudget(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)
	s.restartCount = 2
	s.setState(StateReady)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.startCore(ctx, "config.json"); err != nil {
		t.Fatal(err)
	}
	if s.restartCount != 2 {
		t.Fatalf("internal crash restart reset budget to %d", s.restartCount)
	}
}

func TestCoreLifecycleOutlivesStartupRequestContext(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)
	startupCtx, cancelStartup := context.WithCancel(context.Background())
	if err := s.Start(startupCtx, "config.json"); err != nil {
		t.Fatal(err)
	}
	cancelStartup()
	select {
	case <-mock.startCtx.Done():
		t.Fatal("core lifecycle was canceled with the completed startup request")
	case <-time.After(20 * time.Millisecond):
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-mock.startCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("core lifecycle was not canceled by Supervisor.Stop")
	}
}

func TestCrashMonitorOutlivesStartupRequestContext(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)
	s.backoff = []time.Duration{time.Millisecond}
	startupCtx, cancelStartup := context.WithCancel(context.Background())
	if err := s.Start(startupCtx, "config.json"); err != nil {
		t.Fatal(err)
	}
	cancelStartup()
	mock.status.State = host.HostStateFailed
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.Status().TotalRestarts > 0 {
			_ = s.Stop(context.Background())
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("supervisor stopped monitoring when startup request context ended")
}

func TestSupervisor_DoubleStart(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Start(ctx, "config.json")
	err := s.Start(ctx, "config.json")
	if err == nil {
		t.Error("Start() expected error when already running")
	}
}

func TestSupervisor_Events(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	events := s.Events()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		s.Start(ctx, "config.json")
	}()

	// Collect events
	var received []StateEvent
	timeout := time.After(3 * time.Second)

loop:
	for {
		select {
		case evt := <-events:
			received = append(received, evt)
			if evt.To == StateRunning {
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	if len(received) == 0 {
		t.Error("no events received")
	}

	s.Unsubscribe(events)
}

func TestSupervisor_Restart(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx := context.Background()
	s.Start(ctx, "config.json")
	err := s.Restart(ctx, "config.json")
	if err != nil {
		t.Fatalf("Restart() error: %v", err)
	}

	if s.State() != StateRunning {
		t.Errorf("state after restart = %s, want %s", s.State(), StateRunning)
	}
}

func TestSupervisor_StatusReflectsState(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	status := s.Status()
	if status.State != StateStopped {
		t.Errorf("status.State = %s, want %s", status.State, StateStopped)
	}

	ctx := context.Background()
	s.Start(ctx, "config.json")

	status = s.Status()
	if status.State != StateRunning {
		t.Errorf("status.State = %s, want %s", status.State, StateRunning)
	}
}

func TestSwapConfig(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx := context.Background()
	s.Start(ctx, "config.json")

	err := s.SwapConfig(ctx, "new_config.json")
	if err != nil {
		t.Fatalf("SwapConfig() error: %v", err)
	}

	if s.State() != StateRunning {
		t.Errorf("state after swap = %s, want %s", s.State(), StateRunning)
	}
}

func TestSwapConfig_NotRunning(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	err := s.SwapConfig(context.Background(), "config.json")
	if err == nil {
		t.Error("SwapConfig() expected error when not running")
	}
}

func TestStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		from  State
		event Event
		to    State
	}{
		{StateStopped, EventStart, StateReconcile},
		{StateReconcile, EventReconcileDone, StateReady},
		{StateReady, EventStart, StateStarting},
		{StateStarting, EventStarted, StateRunning},
		{StateRunning, EventStop, StateStopping},
		{StateStopping, EventStopped, StateStopped},
		{StateRunning, EventCrash, StateDirty},
		{StateDirty, EventReconcileDone, StateReady},
		{StateRunning, EventDegrade, StateDegraded},
		{StateDegraded, EventRecover, StateRunning},
		{StateStarting, EventFail, StateFailed},
		{StateFailed, EventStart, StateReconcile},
		{StateFailed, EventStop, StateStopped},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"+"+string(tt.event), func(t *testing.T) {
			to := NextState(tt.from, tt.event)
			if to != tt.to {
				t.Errorf("NextState(%s, %s) = %s, want %s", tt.from, tt.event, to, tt.to)
			}
			if !IsValidTransition(tt.from, tt.event) {
				t.Error("IsValidTransition returned false for valid transition")
			}
		})
	}
}

func TestStateMachine_InvalidTransitions(t *testing.T) {
	invalid := []struct {
		from  State
		event Event
	}{
		{StateStopped, EventStarted},       // can't go directly to Started
		{StateRunning, EventReconcileDone}, // can't reconcile while running
		{StateStopped, EventRecovered},     // can't recover from stopped
	}

	for _, tt := range invalid {
		t.Run(string(tt.from)+"+"+string(tt.event), func(t *testing.T) {
			if IsValidTransition(tt.from, tt.event) {
				t.Errorf("IsValidTransition(%s, %s) should be false", tt.from, tt.event)
			}
		})
	}
}

func TestSupervisor_TotalRestarts(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	ctx := context.Background()
	s.Start(ctx, "config.json")

	// Simulate crash and recovery
	s.mu.Lock()
	s.totalRestarts = 5
	s.mu.Unlock()

	status := s.Status()
	if status.TotalRestarts != 5 {
		t.Errorf("TotalRestarts = %d, want 5", status.TotalRestarts)
	}
}

func TestSupervisor_FailedStart(t *testing.T) {
	mock := &mockCoreHost{}
	s := NewSupervisor(mock, nil)

	_ = context.Background()
	s.setState(StateStarting)
	s.lastError = "simulated failure"

	// Verify error is reflected in status
	status := s.Status()
	if status.LastError == "" {
		t.Error("LastError should not be empty after failure")
	}
}

func TestStateConstants(t *testing.T) {
	states := []State{
		StateStopped, StateReconcile, StateReady,
		StateStarting, StateRunning, StateStopping,
		StateFailed, StateDegraded, StateDirty,
	}
	for _, s := range states {
		if string(s) == "" {
			t.Errorf("state %v has empty string value", s)
		}
	}
}

func TestEventConstants(t *testing.T) {
	events := []Event{
		EventStart, EventStarted, EventStop, EventStopped,
		EventCrash, EventRecovered, EventFail,
		EventDegrade, EventRecover, EventConfigSwap,
		EventDirtyDetected, EventReconcileDone,
	}
	for _, e := range events {
		if string(e) == "" {
			t.Errorf("event %v has empty string value", e)
		}
	}
}

func TestStateEvent(t *testing.T) {
	evt := StateEvent{
		From:      StateStopped,
		To:        StateReconcile,
		Event:     EventStart,
		Timestamp: time.Now(),
		Metadata:  map[string]string{"config": "test.json"},
	}

	if evt.From != StateStopped || evt.To != StateReconcile {
		t.Error("StateEvent fields incorrect")
	}
}
