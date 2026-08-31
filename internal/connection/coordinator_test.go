package connection

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoordinatorSerializesControlAndKeepsObservationSnapshotAvailable(t *testing.T) {
	coordinator := NewCoordinator()
	first, err := coordinator.Begin(context.Background(), Request{
		ID: "switch-node", Operation: OperationNodeSwitch, Origin: OriginUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.SetPhase(PhaseVerifying); err != nil {
		t.Fatal(err)
	}

	acquired := make(chan *Transaction, 1)
	go func() {
		transaction, beginErr := coordinator.Begin(context.Background(), Request{
			ID: "change-policy", Operation: OperationPolicyChange, Origin: OriginUser,
		})
		if beginErr != nil {
			acquired <- nil
			return
		}
		acquired <- transaction
	}()

	deadline := time.Now().Add(time.Second)
	for coordinator.Snapshot().Queued != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	snapshot := coordinator.Snapshot()
	if !snapshot.Busy || snapshot.ID != "switch-node" ||
		snapshot.Phase != PhaseVerifying || snapshot.Queued != 1 {
		t.Fatalf("unexpected active snapshot: %#v", snapshot)
	}
	select {
	case <-acquired:
		t.Fatal("second control transaction bypassed the active transaction")
	case <-time.After(20 * time.Millisecond):
	}

	first.Close()
	second := <-acquired
	if second == nil {
		t.Fatal("second transaction was not acquired")
	}
	if snapshot = coordinator.Snapshot(); snapshot.ID != "change-policy" ||
		snapshot.Operation != OperationPolicyChange {
		t.Fatalf("unexpected second snapshot: %#v", snapshot)
	}
	second.Close()
	snapshot = coordinator.Snapshot()
	if snapshot.Busy || snapshot.LastID != "change-policy" ||
		snapshot.LastPhase != PhaseCompleted {
		t.Fatalf("unexpected completed snapshot: %#v", snapshot)
	}
}

func TestCoordinatorWaitHonorsContextCancellation(t *testing.T) {
	coordinator := NewCoordinator()
	active, err := coordinator.Begin(context.Background(), Request{
		Operation: OperationCaptureSwitch, Origin: OriginUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = coordinator.Begin(ctx, Request{
		Operation: OperationCoreSwitch, Origin: OriginUser,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Begin() error = %v", err)
	}
	if queued := coordinator.Snapshot().Queued; queued != 0 {
		t.Fatalf("canceled waiter leaked queue count %d", queued)
	}
}

func TestCoordinatorTryBeginReportsCurrentOwner(t *testing.T) {
	coordinator := NewCoordinator()
	active, err := coordinator.Begin(context.Background(), Request{
		ID: "active-recovery", Operation: OperationRecovery, Origin: OriginStartup,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()

	_, err = coordinator.TryBegin(Request{
		Operation: OperationSelfHeal, Origin: OriginSelfHeal,
		FaultDomain: "system_proxy",
	})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("TryBegin() error = %v", err)
	}
	var busy *BusyError
	if !errors.As(err, &busy) || busy.Current.ID != "active-recovery" {
		t.Fatalf("busy error lost owner: %#v", err)
	}
}

func TestTransactionFinishRecordsFailure(t *testing.T) {
	coordinator := NewCoordinator()
	transaction, err := coordinator.Begin(context.Background(), Request{
		ID: "failed-switch", Operation: OperationNodeSwitch, Origin: OriginUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	transaction.Finish(errors.New("HTTPS verification failed"))
	snapshot := coordinator.Snapshot()
	if snapshot.Busy || snapshot.LastPhase != PhaseFailed ||
		snapshot.LastError != "HTTPS verification failed" {
		t.Fatalf("failure was not recorded: %#v", snapshot)
	}
}

func TestPolicyForRequestUsesIntentAndControlDomain(t *testing.T) {
	tests := []struct {
		name       string
		request    Request
		wantIntent Intent
		wantDomain Domain
	}{
		{
			name:       "interactive capture",
			request:    Request{Operation: OperationCaptureSwitch, Origin: OriginUser},
			wantIntent: IntentInteractive, wantDomain: DomainCapture,
		},
		{
			name:       "startup routing",
			request:    Request{Operation: OperationNodeSwitch, Origin: OriginStartup},
			wantIntent: IntentStartup, wantDomain: DomainRouting,
		},
		{
			name:       "self heal core fault",
			request:    Request{Operation: OperationSelfHeal, Origin: OriginSelfHeal, FaultDomain: "core"},
			wantIntent: IntentRecovery, wantDomain: DomainCore,
		},
		{
			name:       "shutdown safety",
			request:    Request{Operation: OperationCaptureSwitch, Origin: OriginShutdown},
			wantIntent: IntentSafety, wantDomain: DomainCapture,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := PolicyFor(test.request)
			if policy.Intent != test.wantIntent || policy.Domain != test.wantDomain {
				t.Fatalf("policy = %#v, want intent=%s domain=%s", policy, test.wantIntent, test.wantDomain)
			}
		})
	}
}

func TestCoordinatorDequeuesInteractiveWorkBeforeQueuedStartupWork(t *testing.T) {
	coordinator := NewCoordinator()
	active, err := coordinator.Begin(context.Background(), Request{
		ID: "active", Operation: OperationCoreUpdate, Origin: OriginUser,
	})
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		transaction *Transaction
		err         error
	}
	startupResult := make(chan result, 1)
	go func() {
		transaction, beginErr := coordinator.Begin(context.Background(), Request{
			ID: "startup-capture", Operation: OperationCaptureSwitch, Origin: OriginStartup,
		})
		startupResult <- result{transaction: transaction, err: beginErr}
	}()
	waitForQueued(t, coordinator, 1)

	userResult := make(chan result, 1)
	go func() {
		transaction, beginErr := coordinator.Begin(context.Background(), Request{
			ID: "user-route", Operation: OperationNodeSwitch, Origin: OriginUser,
		})
		userResult <- result{transaction: transaction, err: beginErr}
	}()
	waitForQueued(t, coordinator, 2)
	active.Close()

	user := <-userResult
	if user.err != nil || user.transaction == nil {
		t.Fatalf("interactive request failed: %v", user.err)
	}
	if got := coordinator.Snapshot(); got.ID != "user-route" || got.Intent != IntentInteractive || got.Domain != DomainRouting {
		t.Fatalf("interactive request was not selected first: %#v", got)
	}
	select {
	case startup := <-startupResult:
		if startup.transaction != nil {
			startup.transaction.Close()
		}
		t.Fatalf("startup request bypassed active interactive work: %v", startup.err)
	case <-time.After(20 * time.Millisecond):
	}
	user.transaction.Close()
	startup := <-startupResult
	if startup.err != nil || startup.transaction == nil {
		t.Fatalf("startup request failed after interactive completion: %v", startup.err)
	}
	startup.transaction.Close()
}

func TestCoordinatorSupersedesOnlyLowerPriorityQueuedWorkInSameDomain(t *testing.T) {
	coordinator := NewCoordinator()
	active, err := coordinator.Begin(context.Background(), Request{
		ID: "active-core", Operation: OperationCoreSwitch, Origin: OriginUser,
	})
	if err != nil {
		t.Fatal(err)
	}

	backgroundErr := make(chan error, 1)
	go func() {
		transaction, beginErr := coordinator.Begin(context.Background(), Request{
			ID: "startup-capture", Operation: OperationCaptureSwitch, Origin: OriginStartup,
		})
		if transaction != nil {
			transaction.Close()
		}
		backgroundErr <- beginErr
	}()
	waitForQueued(t, coordinator, 1)

	userResult := make(chan *Transaction, 1)
	go func() {
		transaction, beginErr := coordinator.Begin(context.Background(), Request{
			ID: "user-capture", Operation: OperationCaptureSwitch, Origin: OriginUser,
		})
		if beginErr != nil {
			userResult <- nil
			return
		}
		userResult <- transaction
	}()

	select {
	case beginErr := <-backgroundErr:
		if !errors.Is(beginErr, ErrSuperseded) {
			t.Fatalf("background error = %v, want ErrSuperseded", beginErr)
		}
	case <-time.After(time.Second):
		t.Fatal("lower priority same-domain request was not superseded")
	}
	if snapshot := coordinator.Snapshot(); snapshot.Queued != 1 || snapshot.ID != "active-core" {
		t.Fatalf("active transaction was preempted or queue count is wrong: %#v", snapshot)
	}
	active.Close()
	user := <-userResult
	if user == nil {
		t.Fatal("interactive capture request did not acquire after active transaction")
	}
	user.Close()
}

func waitForQueued(t *testing.T, coordinator *Coordinator, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if coordinator.Snapshot().Queued == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("queued = %d, want %d", coordinator.Snapshot().Queued, want)
}
