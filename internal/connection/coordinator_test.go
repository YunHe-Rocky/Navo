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
