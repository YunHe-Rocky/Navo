package selfheal

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"navo/internal/logstore"
)

type Engine struct {
	cfg      Config
	registry *Registry
	budgets  *budgetStore
	queue    chan ErrorEvent

	mu       sync.Mutex
	started  bool
	stopping bool
	pending  map[string]pendingEvent
	locks    map[string]*sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	sleep    func(context.Context, time.Duration) error
}

type pendingEvent struct {
	last  time.Time
	count uint64
}

func New(cfg Config, registry *Registry) (*Engine, error) {
	if registry == nil {
		return nil, fmt.Errorf("self-heal registry is required")
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 128
	}
	if cfg.VerificationTimeout <= 0 {
		cfg.VerificationTimeout = 15 * time.Second
	}
	if cfg.DefaultMaxAttempts <= 0 {
		cfg.DefaultMaxAttempts = 3
	}
	if cfg.DedupeWindow <= 0 {
		cfg.DedupeWindow = 5 * time.Second
	}
	budgets, err := newBudgetStore(cfg.StateFile)
	if err != nil {
		return nil, err
	}
	return &Engine{
		cfg: cfg, registry: registry, budgets: budgets,
		queue: make(chan ErrorEvent, cfg.QueueSize), pending: make(map[string]pendingEvent),
		locks: make(map[string]*sync.Mutex), sleep: sleepContext,
	}, nil
}

func (e *Engine) Start(parent context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return fmt.Errorf("self-heal engine already started")
	}
	if !e.cfg.Enabled {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	e.cancel = cancel
	e.started = true
	e.stopping = false
	e.wg.Add(1)
	go e.run(ctx)
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.started || e.stopping {
		e.mu.Unlock()
		return
	}
	e.stopping = true
	cancel := e.cancel
	e.mu.Unlock()
	cancel()
	e.wg.Wait()
	e.mu.Lock()
	e.started = false
	e.cancel = nil
	// Events accepted by a previous lifecycle must never execute after restart.
	e.queue = make(chan ErrorEvent, e.cfg.QueueSize)
	e.pending = make(map[string]pendingEvent)
	e.locks = make(map[string]*sync.Mutex)
	e.mu.Unlock()
}

func (e *Engine) Submit(event ErrorEvent) bool {
	if event.Validate() != nil || !e.cfg.Enabled {
		return false
	}
	if _, known := e.registry.Lookup(event.Code); !known {
		return false
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.Count == 0 {
		event.Count = 1
	}
	key, _ := eventKey(event)
	e.mu.Lock()
	if !e.started || e.stopping {
		e.mu.Unlock()
		return false
	}
	if pending, exists := e.pending[key]; exists && event.OccurredAt.Sub(pending.last) <= e.cfg.DedupeWindow {
		pending.last = event.OccurredAt
		pending.count += event.Count
		e.pending[key] = pending
		e.mu.Unlock()
		return true
	}
	e.pending[key] = pendingEvent{last: event.OccurredAt, count: event.Count}
	e.mu.Unlock()
	select {
	case e.queue <- event:
		return true
	default:
		e.mu.Lock()
		delete(e.pending, key)
		e.mu.Unlock()
		_ = logstore.Emit(logstore.LevelWarn, "SelfHeal", "Engine", "SELFHEAL_EVENT_DROPPED", map[string]any{"error_code": event.Code})
		return false
	}
}

func (e *Engine) run(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-e.queue:
			e.mu.Lock()
			stopping := e.stopping
			e.mu.Unlock()
			if stopping || ctx.Err() != nil {
				return
			}
			key, _ := eventKey(event)
			e.mu.Lock()
			if pending, ok := e.pending[key]; ok {
				event.Count = pending.count
			}
			e.mu.Unlock()
			e.process(ctx, event)
			e.mu.Lock()
			delete(e.pending, key)
			e.mu.Unlock()
		}
	}
}

func (e *Engine) process(ctx context.Context, event ErrorEvent) {
	policy, ok := e.registry.Lookup(event.Code)
	if !ok {
		return
	}
	definition := policy.Definition()
	_, resourceHash := eventKey(event)
	_ = logstore.Emit(logstore.LevelWarn, "SelfHeal", string(definition.Category), "SELFHEAL_DETECTED", map[string]any{
		"error_code": event.Code, "resource_hash": resourceHash, "source_service": event.SourceService, "count": event.Count,
	})
	if !definition.AutoRepair || !definition.Retryable || definition.Category == CategorySecurity {
		return
	}
	lock := e.resourceLock(resourceHash)
	lock.Lock()
	defer lock.Unlock()

	verifyCtx, cancel := context.WithTimeout(ctx, e.cfg.VerificationTimeout)
	present, err := policy.FaultPresent(verifyCtx, event)
	cancel()
	if err != nil || !present {
		return
	}
	if e.cfg.ObserveOnly {
		_ = logstore.Emit(logstore.LevelInfo, "SelfHeal", string(definition.Category), "SELFHEAL_REPAIR_OBSERVED", map[string]any{
			"policy": policy.Name(), "error_code": event.Code,
		})
		return
	}
	budget := definition.Budget
	if budget.MaxAttempts == 0 {
		budget.MaxAttempts = e.cfg.DefaultMaxAttempts
	}
	if budget.Window <= 0 {
		budget.Window = 5 * time.Minute
	}
	if budget.Cooldown <= 0 {
		budget.Cooldown = 10 * time.Minute
	}
	attempt, circuit, allowed, err := e.budgets.begin(event, budget)
	if err != nil || !allowed {
		if circuit == "opened" {
			e.emitCircuit("SELFHEAL_CIRCUIT_OPENED", event, resourceHash)
		}
		return
	}
	if circuit == "half_open" {
		e.emitCircuit("SELFHEAL_CIRCUIT_HALF_OPEN", event, resourceHash)
	}
	if err := e.sleep(ctx, backoff(attempt)); err != nil {
		if circuit == "half_open" {
			if resetErr := e.budgets.cancelHalfOpen(event); resetErr != nil {
				_ = logstore.Emit(logstore.LevelError, "SelfHeal", "CircuitBreaker", "SELFHEAL_HALF_OPEN_RELEASE_FAILED", map[string]any{
					"error_code": event.Code, "resource_hash": resourceHash,
				})
			}
		}
		return
	}
	started := time.Now()
	_ = logstore.Emit(logstore.LevelInfo, "SelfHeal", string(definition.Category), "SELFHEAL_REPAIR_STARTED", map[string]any{
		"policy": policy.Name(), "attempt": attempt, "max_attempts": budget.MaxAttempts, "correlation_id": event.CorrelationID,
	})
	action, repairErr := policy.Repair(ctx, event)
	result := VerificationResult{}
	if repairErr == nil {
		verifyCtx, cancel = context.WithTimeout(ctx, e.cfg.VerificationTimeout)
		result, repairErr = policy.Verify(verifyCtx, event, action)
		cancel()
		if repairErr == nil && !result.Recovered {
			repairErr = fmt.Errorf("repair verification did not recover the resource")
		}
	}
	rollback := "not_required"
	if repairErr != nil && action.Mutated {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), e.cfg.VerificationTimeout)
		if err := policy.Rollback(rollbackCtx, event, action); err != nil {
			rollback = "failed"
		} else {
			rollback = "succeeded"
		}
		rollbackCancel()
	}
	success := repairErr == nil
	circuit, persistErr := e.budgets.complete(event, budget, success)
	if persistErr != nil {
		success = false
	}
	message := "SELFHEAL_REPAIR_FAILED"
	level := logstore.LevelError
	if success {
		message = "SELFHEAL_REPAIR_SUCCEEDED"
		level = logstore.LevelInfo
	}
	_ = logstore.Emit(level, "SelfHeal", string(definition.Category), message, map[string]any{
		"error_code": event.Code, "action": action.Name, "verification": result.Evidence,
		"duration_ms": time.Since(started).Milliseconds(), "rollback": rollback,
	})
	if circuit == "opened" {
		e.emitCircuit("SELFHEAL_CIRCUIT_OPENED", event, resourceHash)
	} else if success {
		e.emitCircuit("SELFHEAL_CIRCUIT_CLOSED", event, resourceHash)
	}
}

func (e *Engine) resourceLock(key string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	lock := e.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		e.locks[key] = lock
	}
	return lock
}

func (e *Engine) emitCircuit(message string, event ErrorEvent, resourceHash string) {
	_ = logstore.Emit(logstore.LevelWarn, "SelfHeal", "CircuitBreaker", message, map[string]any{
		"error_code": event.Code, "resource_hash": resourceHash,
	})
}

func backoff(attempt int) time.Duration {
	steps := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	index := min(max(attempt-1, 0), len(steps)-1)
	base := steps[index]
	return base + time.Duration(rand.Int64N(int64(base/5)+1))
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
