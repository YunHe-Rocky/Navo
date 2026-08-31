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
	Intent      Intent
	Domain      Domain
	Priority    int
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

var (
	ErrBusy       = errors.New("connection transaction is busy")
	ErrSuperseded = errors.New("connection transaction was superseded by higher-priority work in the same domain")
)

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
	seq atomic.Uint64

	mu       sync.RWMutex
	snapshot Snapshot
	waiters  []*waiter
}

func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

type beginResult struct {
	transaction *Transaction
	err         error
}

type waiter struct {
	request  Request
	policy   Policy
	sequence uint64
	ready    chan beginResult
}

func (c *Coordinator) Begin(ctx context.Context, request Request) (*Transaction, error) {
	if ctx == nil {
		return nil, fmt.Errorf("connection transaction context is required")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	policy := PolicyFor(request)
	c.mu.Lock()
	if !c.snapshot.Busy && len(c.waiters) == 0 {
		transaction := c.startLocked(request, policy)
		c.mu.Unlock()
		return transaction, nil
	}
	wait := &waiter{
		request: request, policy: policy, sequence: c.seq.Add(1),
		ready: make(chan beginResult, 1),
	}
	superseded := c.supersedeLowerPriorityLocked(wait)
	c.waiters = append(c.waiters, wait)
	c.snapshot.Queued = len(c.waiters)
	c.mu.Unlock()
	for _, previous := range superseded {
		previous.ready <- beginResult{err: fmt.Errorf(
			"%w: %s/%s replaced %s/%s",
			ErrSuperseded, wait.policy.Intent, wait.policy.Domain,
			previous.policy.Intent, previous.policy.Domain,
		)}
	}

	select {
	case <-ctx.Done():
		if c.removeWaiter(wait) {
			return nil, fmt.Errorf("wait for connection transaction: %w", ctx.Err())
		}
		// Admission won the race with cancellation. Release the newly granted
		// transaction immediately so ownership cannot leak.
		result := <-wait.ready
		if result.transaction != nil {
			result.transaction.Finish(ctx.Err())
			return nil, fmt.Errorf("wait for connection transaction: %w", ctx.Err())
		}
		return nil, result.err
	case result := <-wait.ready:
		return result.transaction, result.err
	}
}

func (c *Coordinator) TryBegin(request Request) (*Transaction, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot.Busy || len(c.waiters) > 0 {
		return nil, &BusyError{Current: c.snapshot}
	}
	return c.startLocked(request, PolicyFor(request)), nil
}

func (c *Coordinator) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *Coordinator) startLocked(request Request, policy Policy) *Transaction {
	now := time.Now().UTC()
	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = fmt.Sprintf("connection-%d-%d", now.UnixNano(), c.seq.Add(1))
	}
	queued := len(c.waiters)
	lastID := c.snapshot.LastID
	lastOperation := c.snapshot.LastOperation
	lastPhase := c.snapshot.LastPhase
	lastError := c.snapshot.LastError
	completedAt := c.snapshot.CompletedAt
	c.snapshot = Snapshot{
		Busy: true, ID: id, Operation: request.Operation, Origin: request.Origin,
		Intent: policy.Intent, Domain: policy.Domain, Priority: policy.Rank,
		Phase: PhasePreparing, FaultDomain: strings.TrimSpace(request.FaultDomain),
		StartedAt: now, Queued: queued,
		LastID: lastID, LastOperation: lastOperation, LastPhase: lastPhase,
		LastError: lastError, CompletedAt: completedAt,
	}
	return &Transaction{coordinator: c, id: id}
}

func (c *Coordinator) supersedeLowerPriorityLocked(next *waiter) []*waiter {
	if next == nil {
		return nil
	}
	kept := c.waiters[:0]
	var superseded []*waiter
	for _, current := range c.waiters {
		if current.policy.Domain == next.policy.Domain && current.policy.Rank < next.policy.Rank {
			superseded = append(superseded, current)
			continue
		}
		kept = append(kept, current)
	}
	c.waiters = kept
	return superseded
}

func (c *Coordinator) removeWaiter(target *waiter) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for index, current := range c.waiters {
		if current != target {
			continue
		}
		c.waiters = append(c.waiters[:index], c.waiters[index+1:]...)
		c.snapshot.Queued = len(c.waiters)
		return true
	}
	return false
}

func (c *Coordinator) popNextLocked() *waiter {
	if len(c.waiters) == 0 {
		return nil
	}
	best := 0
	for index := 1; index < len(c.waiters); index++ {
		candidate, current := c.waiters[index], c.waiters[best]
		if candidate.policy.Rank > current.policy.Rank ||
			(candidate.policy.Rank == current.policy.Rank && candidate.sequence < current.sequence) {
			best = index
		}
	}
	next := c.waiters[best]
	c.waiters = append(c.waiters[:best], c.waiters[best+1:]...)
	return next
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
			queued := len(c.waiters)
			c.snapshot = Snapshot{
				Queued: queued, LastID: t.id,
				LastOperation: c.snapshot.Operation, LastPhase: phase,
				LastError: message, CompletedAt: time.Now().UTC(),
			}
		}
		next := c.popNextLocked()
		var granted *Transaction
		if next != nil {
			granted = c.startLocked(next.request, next.policy)
		}
		c.mu.Unlock()
		if next != nil {
			next.ready <- beginResult{transaction: granted}
		}
	})
}

func (t *Transaction) Close() {
	t.Finish(nil)
}
