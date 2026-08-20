package selfheal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testPolicy(code ErrorCode, calls *atomic.Int32) PolicyFuncs {
	return PolicyFuncs{
		PolicyName: "test-policy",
		Def: Definition{
			Code: code, Category: CategoryMonitor, Severity: SeverityError,
			Retryable: true, AutoRepair: true,
			Budget: Budget{MaxAttempts: 2, Window: time.Hour, Cooldown: time.Hour},
		},
		CheckFunc: func(context.Context, ErrorEvent) (bool, error) { return true, nil },
		RepairFunc: func(context.Context, ErrorEvent) (RepairAction, error) {
			calls.Add(1)
			return RepairAction{Name: "repair", Mutated: true}, nil
		},
		VerifyFunc: func(context.Context, ErrorEvent, RepairAction) (VerificationResult, error) {
			return VerificationResult{Recovered: true, Evidence: "verified"}, nil
		},
	}
}

func newTestEngine(t *testing.T, cfg Config, policy Policy) *Engine {
	t.Helper()
	registry, err := NewRegistry(policy)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	engine.sleep = func(context.Context, time.Duration) error { return nil }
	if err := engine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Stop)
	return engine
}

func waitFor(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not reached")
}

func TestUnknownCodeNeverRuns(t *testing.T) {
	var calls atomic.Int32
	cfg := DefaultConfig("")
	engine := newTestEngine(t, cfg, testPolicy(CodeTrafficCollectorStale, &calls))
	if engine.Submit(ErrorEvent{Code: CodeCoreCrashed, SourceService: "Supervisor"}) {
		t.Fatal("unknown error code was accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("unknown error triggered repair")
	}
}

func TestObserveOnlyDoesNotRepairOrConsumeBudget(t *testing.T) {
	var calls atomic.Int32
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "selfheal-state.json"))
	cfg.ObserveOnly = true
	engine := newTestEngine(t, cfg, testPolicy(CodeTrafficCollectorStale, &calls))
	event := ErrorEvent{Code: CodeTrafficCollectorStale, SourceService: "Monitor", ResourceID: "collector"}
	if !engine.Submit(event) {
		t.Fatal("event was not accepted")
	}
	waitFor(t, func() bool {
		engine.mu.Lock()
		defer engine.mu.Unlock()
		return len(engine.pending) == 0
	})
	if calls.Load() != 0 {
		t.Fatal("observe-only executed repair")
	}
	if _, err := os.Stat(cfg.StateFile); !os.IsNotExist(err) {
		t.Fatalf("observe-only mutated budget state: %v", err)
	}
}

func TestVerificationFailureRollsBackAndOpensCircuit(t *testing.T) {
	var repairs, rollbacks atomic.Int32
	policy := testPolicy(CodeDNSMismatch, &repairs)
	policy.Def.Budget.MaxAttempts = 1
	policy.VerifyFunc = func(context.Context, ErrorEvent, RepairAction) (VerificationResult, error) {
		return VerificationResult{}, errors.New("DNS still mismatched")
	}
	policy.RollbackFunc = func(context.Context, ErrorEvent, RepairAction) error {
		rollbacks.Add(1)
		return nil
	}
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "selfheal-state.json"))
	engine := newTestEngine(t, cfg, policy)
	event := ErrorEvent{Code: CodeDNSMismatch, SourceService: "Network", ResourceID: "secret.example"}
	if !engine.Submit(event) {
		t.Fatal("event was not accepted")
	}
	waitFor(t, func() bool {
		if rollbacks.Load() != 1 {
			return false
		}
		engine.mu.Lock()
		defer engine.mu.Unlock()
		return len(engine.pending) == 0
	})
	if repairs.Load() != 1 {
		t.Fatalf("repair calls = %d", repairs.Load())
	}
	data, err := os.ReadFile(cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret.example") {
		t.Fatal("raw resource identifier persisted")
	}
	if !strings.Contains(string(data), `"open_until"`) {
		t.Fatal("circuit did not open")
	}
}

func TestStopDuringHalfOpenBackoffReleasesCircuit(t *testing.T) {
	var calls atomic.Int32
	policy := testPolicy(CodeDNSMismatch, &calls)
	policy.Def.Budget.MaxAttempts = 1
	policy.Def.Budget.Cooldown = time.Minute
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "selfheal-state.json"))
	registry, err := NewRegistry(policy)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	engine.budgets.now = func() time.Time { return now }
	event := ErrorEvent{Code: CodeDNSMismatch, SourceService: "Network", ResourceID: "dns"}
	if _, state, allowed, err := engine.budgets.begin(event, policy.Def.Budget); err != nil || !allowed || state != "closed" {
		t.Fatalf("begin initial attempt: state=%q allowed=%v err=%v", state, allowed, err)
	}
	if state, err := engine.budgets.complete(event, policy.Def.Budget, false); err != nil || state != "opened" {
		t.Fatalf("open circuit: state=%q err=%v", state, err)
	}
	now = now.Add(policy.Def.Budget.Cooldown + time.Second)

	sleepStarted := make(chan struct{})
	engine.sleep = func(ctx context.Context, _ time.Duration) error {
		close(sleepStarted)
		<-ctx.Done()
		return ctx.Err()
	}
	if err := engine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !engine.Submit(event) {
		t.Fatal("half-open event was not accepted")
	}
	select {
	case <-sleepStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("half-open attempt did not reach backoff")
	}
	engine.Stop()

	_, state, allowed, err := engine.budgets.begin(event, policy.Def.Budget)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || state != "half_open" {
		t.Fatalf("canceled half-open attempt stayed busy: state=%q allowed=%v", state, allowed)
	}
}

func TestStopDropsQueuedEventsBeforeRestart(t *testing.T) {
	var calls atomic.Int32
	policy := testPolicy(CodeTrafficCollectorStale, &calls)
	blocked := make(chan struct{})
	policy.CheckFunc = func(ctx context.Context, event ErrorEvent) (bool, error) {
		if event.ResourceID != "blocking" {
			return true, nil
		}
		close(blocked)
		<-ctx.Done()
		return false, ctx.Err()
	}
	cfg := DefaultConfig("")
	registry, err := NewRegistry(policy)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	engine.sleep = func(context.Context, time.Duration) error { return nil }
	if err := engine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !engine.Submit(ErrorEvent{Code: CodeTrafficCollectorStale, SourceService: "Monitor", ResourceID: "blocking"}) {
		t.Fatal("blocking event was not accepted")
	}
	select {
	case <-blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking event did not start")
	}
	if !engine.Submit(ErrorEvent{Code: CodeTrafficCollectorStale, SourceService: "Monitor", ResourceID: "stale"}) {
		t.Fatal("queued event was not accepted")
	}
	engine.Stop()
	if calls.Load() != 0 {
		t.Fatalf("queued repair ran during stop: calls=%d", calls.Load())
	}
	if err := engine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Stop)
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("stale queued repair ran after restart: calls=%d", calls.Load())
	}
}

func TestDuplicateStormCoalescesRepair(t *testing.T) {
	var calls atomic.Int32
	policy := testPolicy(CodeTrafficCollectorStale, &calls)
	gate := make(chan struct{})
	policy.CheckFunc = func(ctx context.Context, _ ErrorEvent) (bool, error) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-gate:
			return true, nil
		}
	}
	cfg := DefaultConfig("")
	engine := newTestEngine(t, cfg, policy)
	event := ErrorEvent{Code: CodeTrafficCollectorStale, SourceService: "Monitor", ResourceID: "traffic"}
	for range 100 {
		if !engine.Submit(event) {
			t.Fatal("duplicate event was not coalesced")
		}
	}
	close(gate)
	waitFor(t, func() bool { return calls.Load() == 1 })
}

func TestStopCancelsPendingRepair(t *testing.T) {
	var calls atomic.Int32
	policy := testPolicy(CodeTrafficCollectorStale, &calls)
	policy.CheckFunc = func(ctx context.Context, _ ErrorEvent) (bool, error) {
		<-ctx.Done()
		return false, ctx.Err()
	}
	cfg := DefaultConfig("")
	registry, _ := NewRegistry(policy)
	engine, err := New(cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	engine.Submit(ErrorEvent{Code: CodeTrafficCollectorStale, SourceService: "Monitor"})
	engine.Stop()
	if calls.Load() != 0 {
		t.Fatal("repair ran during shutdown")
	}
}

func TestRepairBudgetIsHardCappedAtTwoAttempts(t *testing.T) {
	var repairs atomic.Int32
	policy := testPolicy(CodeDNSMismatch, &repairs)
	policy.Def.Budget.MaxAttempts = 5
	policy.VerifyFunc = func(context.Context, ErrorEvent, RepairAction) (VerificationResult, error) {
		return VerificationResult{}, errors.New("DNS remains unavailable")
	}
	policy.RollbackFunc = func(context.Context, ErrorEvent, RepairAction) error {
		return nil
	}
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "selfheal-state.json"))
	cfg.DefaultMaxAttempts = 9
	cfg.DedupeWindow = time.Nanosecond
	engine := newTestEngine(t, cfg, policy)

	for index := 0; index < 3; index++ {
		event := ErrorEvent{
			Code: CodeDNSMismatch, SourceService: "Network", ResourceID: "dns",
			OccurredAt: time.Now().Add(time.Duration(index) * time.Second),
		}
		if !engine.Submit(event) {
			t.Fatalf("event %d was not accepted", index+1)
		}
		waitFor(t, func() bool {
			engine.mu.Lock()
			defer engine.mu.Unlock()
			return len(engine.pending) == 0
		})
	}
	if repairs.Load() != MaxRepairRounds {
		t.Fatalf("repair calls = %d, want hard cap %d", repairs.Load(), MaxRepairRounds)
	}
}

func TestDefaultRepairBudgetIsTwoRounds(t *testing.T) {
	cfg := DefaultConfig("")
	if cfg.DefaultMaxAttempts != MaxRepairRounds {
		t.Fatalf("DefaultMaxAttempts = %d, want %d", cfg.DefaultMaxAttempts, MaxRepairRounds)
	}
}
