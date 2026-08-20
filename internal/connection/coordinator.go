// Package connection owns the cross-domain connection transaction boundary.
//
// Observation remains concurrent. Every operation that can change a core,
// node, capture mode, routing policy, or owned network resource must hold one
// Transaction from Coordinator.
package connection

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Operation string

const (
	OperationCaptureSwitch  Operation = "capture_switch"
	OperationNodeSwitch     Operation = "node_switch"
	OperationCoreSwitch     Operation = "core_switch"
	OperationSourceMutation Operation = "source_mutation"
	OperationPolicyChange   Operation = "policy_change"
	OperationCoreUpdate     Operation = "core_update"
	OperationRecovery       Operation = "recovery"
	OperationSelfHeal       Operation = "self_heal"
)

func (o Operation) Valid() bool {
	switch o {
	case OperationCaptureSwitch, OperationNodeSwitch, OperationCoreSwitch,
		OperationSourceMutation, OperationPolicyChange, OperationCoreUpdate,
		OperationRecovery, OperationSelfHeal:
		return true
	default:
		return false
	}
}

type Origin string

const (
	OriginUser      Origin = "user"
	OriginTray      Origin = "tray"
	OriginScheduler Origin = "scheduler"
	OriginSelfHeal  Origin = "self_heal"
	OriginStartup   Origin = "startup"
	OriginShutdown  Origin = "shutdown"
)

func (o Origin) Valid() bool {
	switch o {
	case OriginUser, OriginTray, OriginScheduler, OriginSelfHeal, OriginStartup, OriginShutdown:
		return true
	default:
		return false
	}
}

type Phase string

const (
	PhaseQueued      Phase = "queued"
	PhasePreparing   Phase = "preparing"
	PhaseApplying    Phase = "applying"
	PhaseVerifying   Phase = "verifying"
	PhaseCommitting  Phase = "committing"
	PhaseRollingBack Phase = "rolling_back"
	PhaseCompleted   Phase = "completed"
	PhaseFailed      Phase = "failed"
)

func (p Phase) Valid() bool {
	switch p {
	case PhaseQueued, PhasePreparing, PhaseApplying, PhaseVerifying,
		PhaseCommitting, PhaseRollingBack, PhaseCompleted, PhaseFailed:
		return true
	default:
		return false
	}
}

type Request struct {
	ID          string
	Operation   Operation
	Origin      Origin
	FaultDomain string
}

func (r Request) Validate() error {
	if !r.Operation.Valid() {
		return fmt.Errorf("invalid connection operation %q", r.Operation)
	}
	if !r.Origin.Valid() {
		return fmt.Errorf("invalid connection origin %q", r.Origin)
	}
	return nil
}

type Snapshot struct {
	Busy        bool
	ID          string
	Operation   Operation
	Origin      Origin
	Phase       Phase
	FaultDomain string
	StartedAt   time.Time
	Queued      int

	LastID        string
	LastOperation Operation
	LastPhase     Phase
	LastError     string
	CompletedAt   time.Time
}

var ErrBusy = errors.New("connection transaction is busy")

type BusyError struct {
	Current Snapshot
}

func (e *BusyError) Error() string {
	if e == nil || !e.Current.Busy {
		return ErrBusy.Error()
	}
	return fmt.Sprintf(
		"%s: %s (%s) is in phase %s",
		ErrBusy, e.Current.Operation, e.Current.ID, e.Current.Phase,
	)
}

func (e *BusyError) Unwrap() error { return ErrBusy }

type Coordinator struct {
	token chan struct{}
	seq   atomic.Uint64

	mu       sync.RWMutex
	snapshot Snapshot
}

func NewCoordinator() *Coordinator {
	c := &Coordinator{token: make(chan struct{}, 1)}
	c.token <- struct{}{}
	return c
}

func (c *Coordinator) Begin(ctx context.Context, request Request) (*Transaction, error) {
	if ctx == nil {
		return nil, fmt.Errorf("connection transaction context is required")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	c.adjustQueued(1)
	select {
	case <-ctx.Done():
		c.adjustQueued(-1)
		return nil, fmt.Errorf("wait for connection transaction: %w", ctx.Err())
	case <-c.token:
		c.adjustQueued(-1)
		return c.start(request), nil
	}
}

func (c *Coordinator) TryBegin(request Request) (*Transaction, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	select {
	case <-c.token:
		return c.start(request), nil
	default:
		snapshot := c.Snapshot()
		return nil, &BusyError{Current: snapshot}
	}
}

func (c *Coordinator) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *Coordinator) start(request Request) *Transaction {
	now := time.Now().UTC()
	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = fmt.Sprintf("connection-%d-%d", now.UnixNano(), c.seq.Add(1))
	}
	c.mu.Lock()
	queued := c.snapshot.Queued
	lastID := c.snapshot.LastID
	lastOperation := c.snapshot.LastOperation
	lastPhase := c.snapshot.LastPhase
	lastError := c.snapshot.LastError
	completedAt := c.snapshot.CompletedAt
	c.snapshot = Snapshot{
		Busy: true, ID: id, Operation: request.Operation, Origin: request.Origin,
		Phase: PhasePreparing, FaultDomain: strings.TrimSpace(request.FaultDomain),
		StartedAt: now, Queued: queued,
		LastID: lastID, LastOperation: lastOperation, LastPhase: lastPhase,
		LastError: lastError, CompletedAt: completedAt,
	}
	c.mu.Unlock()
	return &Transaction{coordinator: c, id: id}
}

func (c *Coordinator) adjustQueued(delta int) {
	c.mu.Lock()
	c.snapshot.Queued += delta
	if c.snapshot.Queued < 0 {
		c.snapshot.Queued = 0
	}
	c.mu.Unlock()
}

type Transaction struct {
	coordinator *Coordinator
	id          string
	once        sync.Once
}

func (t *Transaction) ID() string {
	if t == nil {
		return ""
	}
	return t.id
}

func (t *Transaction) SetPhase(phase Phase) error {
	if t == nil || t.coordinator == nil {
		return fmt.Errorf("connection transaction is not active")
	}
	if !phase.Valid() {
		return fmt.Errorf("invalid connection phase %q", phase)
	}
	t.coordinator.mu.Lock()
	defer t.coordinator.mu.Unlock()
	if !t.coordinator.snapshot.Busy || t.coordinator.snapshot.ID != t.id {
		return fmt.Errorf("connection transaction %s no longer owns the coordinator", t.id)
	}
	t.coordinator.snapshot.Phase = phase
	return nil
}

func (t *Transaction) Finish(result error) {
	if t == nil || t.coordinator == nil {
		return
	}
	t.once.Do(func() {
		c := t.coordinator
		c.mu.Lock()
		if c.snapshot.Busy && c.snapshot.ID == t.id {
			phase := PhaseCompleted
			message := ""
			if result != nil {
				phase = PhaseFailed
				message = result.Error()
			}
			queued := c.snapshot.Queued
			c.snapshot = Snapshot{
				Queued: queued, LastID: t.id,
				LastOperation: c.snapshot.Operation, LastPhase: phase,
				LastError: message, CompletedAt: time.Now().UTC(),
			}
		}
		c.mu.Unlock()
		c.token <- struct{}{}
	})
}

func (t *Transaction) Close() {
	t.Finish(nil)
}
