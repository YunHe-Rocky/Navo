package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"navo/internal/host"
	"navo/internal/network"
)

// Supervisor manages the lifecycle of a CoreHost with a state machine,
// crash recovery, and config swap support.
type Supervisor struct {
	host       host.CoreHost
	reconciler *network.Reconciler

	mu         sync.RWMutex
	state      State
	stateSince time.Time
	status     SupervisorStatus
	lastError  string

	// Crash recovery
	backoff           []time.Duration
	maxRestarts       int
	restartCount      int
	totalRestarts     int
	restartSuppressed bool
	warmIdle          bool

	// Config management
	activeConfigPath string

	// Event subscribers
	eventSubs []chan StateEvent
	subMu     sync.RWMutex

	// Internal control
	cancel context.CancelFunc
}

// NewSupervisor creates a new Supervisor wrapping a CoreHost and Reconciler.
func NewSupervisor(h host.CoreHost, rec *network.Reconciler) *Supervisor {
	return &Supervisor{
		host:        h,
		reconciler:  rec,
		state:       StateStopped,
		stateSince:  time.Now(),
		backoff:     []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second},
		maxRestarts: 3,
	}
}

// ── Public API ──

// Start begins the supervisor lifecycle.
func (s *Supervisor) Start(ctx context.Context, configPath string) error {
	s.mu.Lock()
	if s.state == StateRunning || s.state == StateStarting {
		s.mu.Unlock()
		return fmt.Errorf("supervisor already running (state=%s)", s.state)
	}
	// A user/coordinator initiated start begins a new recovery budget. Internal
	// crash restarts call startCore directly and retain the consecutive count.
	s.restartCount = 0
	s.warmIdle = false
	s.mu.Unlock()

	// Transition to RECONCILE
	if err := s.transition(EventStart); err != nil {
		return err
	}

	// Reconcile (check recovery state, clean up TUN/route/DNS)
	s.setState(StateReconcile)
	if s.reconciler != nil {
		result, err := s.reconciler.Reconcile(ctx, &network.ReconcileConfig{ListenPort: 0})
		if err != nil {
			s.mu.Lock()
			s.lastError = err.Error()
			s.mu.Unlock()
			s.setState(StateFailed)
			return fmt.Errorf("network reconciliation failed: %w", err)
		} else if result.RecoveryState == host.RecoveryDirty {
			err := fmt.Errorf("network reconciliation remained dirty: found=%d fixed=%d unfixed=%d",
				len(result.IssuesFound), len(result.IssuesFixed), len(result.IssuesUnfixed))
			s.mu.Lock()
			s.lastError = err.Error()
			s.mu.Unlock()
			s.setState(StateFailed)
			return err
		}
	}
	s.transition(EventReconcileDone)
	s.setState(StateReady)

	// Start the core
	return s.startCore(ctx, configPath)
}

// Stop gracefully shuts down the core and transitions to stopped.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	state := s.state
	cancel := s.cancel
	s.warmIdle = false
	s.mu.Unlock()

	if state == StateStopped {
		return fmt.Errorf("cannot stop from state=%s", state)
	}
	if cancel != nil {
		cancel()
	}
	if IsValidTransition(state, EventStop) {
		if err := s.transition(EventStop); err != nil {
			return err
		}
	}
	s.setState(StateStopping)

	// Mark dirty shutdown before stopping (Recovery state uses NORMAL only when stop completes)
	if s.reconciler != nil {
		hostStatus := s.host.Status()
		s.reconciler.MarkDirtyShutdown(hostStatus.PID, 0, "", nil)
	}

	force := state != StateRunning && state != StateDegraded
	stopErr := s.host.Stop(ctx, force, 10*time.Second)
	if stopErr != nil {
		s.mu.Lock()
		s.lastError = stopErr.Error()
		s.mu.Unlock()
		s.setState(StateFailed)
		return stopErr
	}

	// Mark normal only after a verified clean stop; failed cleanup remains dirty
	// so the next startup reconciler gets another chance.
	if s.reconciler != nil && stopErr == nil {
		s.reconciler.MarkNormalExit()
	}

	if IsValidTransition(StateStopping, EventStopped) {
		_ = s.transition(EventStopped)
	}
	s.setState(StateStopped)
	return stopErr
}

// Restart stops and restarts the core.
func (s *Supervisor) Restart(ctx context.Context, configPath string) error {
	if err := s.Stop(ctx); err != nil {
		// A replacement must never start while the old process may still own the
		// listener or TUN handle. Force cleanup is independent and bounded.
		forceCtx, forceCancel := context.WithTimeout(context.Background(), 5*time.Second)
		forceErr := s.host.Stop(forceCtx, true, 5*time.Second)
		forceCancel()
		if forceErr != nil {
			s.setState(StateFailed)
			return errors.Join(err, fmt.Errorf("force stop before restart: %w", forceErr))
		}
		s.setState(StateStopped)
	}
	return s.Start(ctx, configPath)
}

// SwapConfig stops the current core and starts a new one with a different config.
func (s *Supervisor) SwapConfig(ctx context.Context, newConfigPath string) error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return fmt.Errorf("cannot swap config: not running (state=%s)", s.state)
	}
	previousConfigPath := s.activeConfigPath
	cancel := s.cancel
	s.mu.Unlock()

	if err := s.transition(EventConfigSwap); err != nil {
		return err
	}

	s.setState(StateStopping)
	if cancel != nil {
		// Stop the old crash monitor before stopping its process. Otherwise it can
		// race the coordinated replacement and restart the old configuration.
		cancel()
	}

	if err := s.host.Stop(ctx, false, 10*time.Second); err != nil {
		stopErr := err
		s.mu.Lock()
		s.lastError = stopErr.Error()
		s.mu.Unlock()
		forceCtx, forceCancel := context.WithTimeout(context.Background(), 5*time.Second)
		forceErr := s.host.Stop(forceCtx, true, 5*time.Second)
		forceCancel()
		if forceErr != nil {
			s.setState(StateFailed)
			return errors.Join(
				fmt.Errorf("stop old core before config swap: %w", stopErr),
				fmt.Errorf("force stop old core: %w", forceErr),
			)
		}
		s.setState(StateStopped)
		if previousConfigPath != "" {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 15*time.Second)
			rollbackErr := s.Start(rollbackCtx, previousConfigPath)
			rollbackCancel()
			if rollbackErr != nil {
				return errors.Join(
					fmt.Errorf("stop old core before config swap: %w", stopErr),
					fmt.Errorf("restore previous config: %w", rollbackErr),
				)
			}
		}
		return fmt.Errorf("stop old core before config swap: %w; previous config restored", stopErr)
	}

	s.transition(EventStopped)
	s.setState(StateStopped)

	if err := s.Start(ctx, newConfigPath); err != nil {
		if previousConfigPath == "" {
			return err
		}
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer rollbackCancel()
		if rollbackErr := s.Start(rollbackCtx, previousConfigPath); rollbackErr != nil {
			return fmt.Errorf("start replacement: %w; restore previous config: %v", err, rollbackErr)
		}
		return fmt.Errorf("start replacement: %w; previous config restored", err)
	}
	return nil
}

// Status returns the current supervisor status.
func (s *Supervisor) Status() SupervisorStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.status
	status.State = s.state
	status.StateSince = s.stateSince
	status.LastError = s.lastError
	status.TotalRestarts = s.totalRestarts

	if s.state == StateRunning {
		hostStatus := s.host.Status()
		status.PID = hostStatus.PID
		status.Uptime = hostStatus.Uptime
		status.ConfigHash = hostStatus.ConfigHash
		status.RestartCount = s.restartCount
	}

	return status
}

// SetRestartSuppressed prevents the crash monitor from racing an externally
// coordinated capture transition or recovery.
func (s *Supervisor) SetRestartSuppressed(suppressed bool) {
	s.mu.Lock()
	s.restartSuppressed = suppressed
	s.mu.Unlock()
}

// SetWarmIdle marks a running loopback-only core as retained for a bounded
// System Proxy lease. A warm-idle crash is treated as lease loss and must not
// consume recovery budget or restart a core while capture is committed off.
func (s *Supervisor) SetWarmIdle(warm bool) {
	s.mu.Lock()
	s.warmIdle = warm && s.state == StateRunning
	s.mu.Unlock()
}

func (s *Supervisor) WarmIdle() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.warmIdle
}

func (s *Supervisor) consumeWarmIdle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.warmIdle {
		return false
	}
	s.warmIdle = false
	return true
}

func (s *Supervisor) restartIsSuppressed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.restartSuppressed
}

// State returns the current state.
func (s *Supervisor) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Events returns a channel that receives state change events.
// Callers must read the channel promptly to avoid blocking.
func (s *Supervisor) Events() <-chan StateEvent {
	ch := make(chan StateEvent, 50)
	s.subMu.Lock()
	s.eventSubs = append(s.eventSubs, ch)
	s.subMu.Unlock()
	return ch
}

// Unsubscribe removes an event subscription.
func (s *Supervisor) Unsubscribe(ch <-chan StateEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for i, sub := range s.eventSubs {
		if sub == ch {
			s.eventSubs = append(s.eventSubs[:i], s.eventSubs[i+1:]...)
			close(sub)
			return
		}
	}
}

// ── Internal ──

func (s *Supervisor) startCore(ctx context.Context, configPath string) error {
	if err := s.transition(EventStart); err != nil {
		return err
	}

	s.setState(StateStarting)
	s.mu.Lock()
	s.activeConfigPath = configPath
	s.mu.Unlock()

	// The request context bounds startup only. The core must outlive the
	// capture.prepare IPC request that started it and is stopped explicitly by
	// Supervisor.Stop.
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = lifecycleCancel
	s.mu.Unlock()
	stopStartupCancellation := context.AfterFunc(ctx, lifecycleCancel)
	pid, err := s.host.Start(lifecycleCtx, configPath)
	stopStartupCancellation()
	if err != nil {
		lifecycleCancel()
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		s.transition(EventFail)
		s.setState(StateFailed)
		return fmt.Errorf("failed to start core: %w", err)
	}

	s.transition(EventStarted)
	s.setState(StateRunning)

	log.Printf("[supervisor] core started (PID=%d, config=%s)", pid, configPath)

	// Start crash monitor
	go s.monitor(lifecycleCtx, configPath)

	return nil
}

// monitor watches for process exit and handles crash recovery.
func (s *Supervisor) monitor(ctx context.Context, configPath string) {
	// Poll for state changes
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			state := s.state
			s.mu.RUnlock()

			if state != StateRunning && state != StateDegraded && state != StateDirty {
				continue
			}

			hostStatus := s.host.Status()
			if hostStatus.State == host.HostStateRunning {
				continue // still running
			}

			// Only trigger crash recovery when the host actually failed,
			// not during a planned stop/restart (Stopping/Stopped).
			if hostStatus.State == host.HostStateFailed {
				s.handleCrash(ctx, configPath)
				return
			}
		}
	}
}

func (s *Supervisor) handleCrash(ctx context.Context, configPath string) {
	s.mu.Lock()
	previousLifecycleCancel := s.cancel
	s.mu.Unlock()
	if previousLifecycleCancel != nil {
		previousLifecycleCancel()
	}
	if s.consumeWarmIdle() {
		s.setState(StateStopped)
		log.Printf("[supervisor] warm-idle core exited; lease invalidated without restart")
		return
	}
	if s.restartIsSuppressed() {
		s.setState(StateFailed)
		log.Printf("[supervisor] core restart suppressed by capture coordinator")
		return
	}
	s.mu.Lock()
	s.restartCount++
	s.totalRestarts++
	restartAttempt := s.restartCount
	s.lastError = "process exited unexpectedly"
	s.mu.Unlock()

	s.transition(EventCrash)
	s.setState(StateDirty)

	log.Printf("[supervisor] core crashed (attempt %d/%d)", restartAttempt, s.maxRestarts)

	if restartAttempt > s.maxRestarts {
		s.transition(EventFail)
		s.setState(StateFailed)
		log.Printf("[supervisor] max restarts exceeded, entering FAILED state")
		return
	}

	// Apply backoff
	backoffDuration := s.backoff[min(restartAttempt-1, len(s.backoff)-1)]
	log.Printf("[supervisor] backoff %v before restart attempt %d", backoffDuration, restartAttempt)
	timer := time.NewTimer(backoffDuration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	if s.restartIsSuppressed() {
		s.setState(StateFailed)
		log.Printf("[supervisor] core restart suppressed during backoff")
		return
	}

	// Reconcile before restarting
	s.setState(StateReconcile)
	if s.reconciler != nil {
		result, err := s.reconciler.Reconcile(ctx, &network.ReconcileConfig{})
		if err != nil {
			log.Printf("[supervisor] crash reconciliation error: %v", err)
		} else {
			log.Printf("[supervisor] crash reconciliation: fixed=%d issues", len(result.IssuesFixed))
		}
	}
	s.transition(EventReconcileDone)
	s.setState(StateReady)

	// Attempt restart
	if err := s.startCore(ctx, configPath); err != nil {
		log.Printf("[supervisor] restart attempt %d failed: %v", restartAttempt, err)
		// The startCore failure will set state to Failed
	}
}

// transition emits a state event for the given event.
func (s *Supervisor) transition(event Event) error {
	from := s.State()
	to := NextState(from, event)
	if to == "" {
		return fmt.Errorf("invalid transition: %s -> %s", from, event)
	}

	evt := StateEvent{
		From:      from,
		To:        to,
		Event:     event,
		Timestamp: time.Now(),
	}

	s.emit(evt)
	return nil
}

// setState updates the current state.
func (s *Supervisor) setState(state State) {
	s.mu.Lock()
	old := s.state
	s.state = state
	s.stateSince = time.Now()
	s.mu.Unlock()

	if old != state {
		log.Printf("[supervisor] state: %s -> %s", old, state)
	}
}

// emit sends a state event to all subscribers.
func (s *Supervisor) emit(event StateEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for _, ch := range s.eventSubs {
		select {
		case ch <- event:
		default:
			// Subscriber is slow, drop event
		}
	}
}
