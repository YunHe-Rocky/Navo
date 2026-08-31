package service

import (
	"context"
	"log"
	"time"

	"navo/internal/supervisor"
)

const defaultSystemProxyWarmTTL = 90 * time.Second

type systemProxyWarmIdentity struct {
	coreID           string
	selectedOutbound string
	revisionID       string
	configHash       string
	configPath       string
	hostConfigHash   string
	pid              int
}

type systemProxyWarmLease struct {
	identity     systemProxyWarmIdentity
	verification RuntimeRoutingVerification
	remembered   bool
	idle         bool
	expiresAt    time.Time
}

func (l *systemProxyWarmLease) remember(identity systemProxyWarmIdentity, verification RuntimeRoutingVerification) {
	l.identity = identity
	l.verification = verification
	l.remembered = true
	l.idle = false
	l.expiresAt = time.Time{}
}

func (l *systemProxyWarmLease) retain(now time.Time, ttl time.Duration) bool {
	if !l.remembered || ttl <= 0 {
		return false
	}
	l.idle = true
	l.expiresAt = now.Add(ttl)
	return true
}

func (l *systemProxyWarmLease) reusable(
	identity systemProxyWarmIdentity,
	status supervisor.SupervisorStatus,
	now time.Time,
) bool {
	if !l.remembered || !l.idle || l.expiresAt.IsZero() || !now.Before(l.expiresAt) {
		return false
	}
	if status.State != supervisor.StateRunning || status.PID <= 0 || status.PID != l.identity.pid {
		return false
	}
	return identity == l.identity
}

func (l *systemProxyWarmLease) activeMatches(
	identity systemProxyWarmIdentity,
	status supervisor.SupervisorStatus,
) bool {
	if !l.remembered || l.idle || status.State != supervisor.StateRunning || status.PID <= 0 {
		return false
	}
	return status.PID == l.identity.pid && identity == l.identity
}

func (l *systemProxyWarmLease) activate() {
	l.idle = false
	l.expiresAt = time.Time{}
}

func (l *systemProxyWarmLease) clear() {
	*l = systemProxyWarmLease{}
}

func (l systemProxyWarmLease) remaining(now time.Time) time.Duration {
	if !l.idle || l.expiresAt.IsZero() || !now.Before(l.expiresAt) {
		return 0
	}
	return l.expiresAt.Sub(now)
}

func (s *Service) warmNow() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func (s *Service) currentSystemProxyWarmIdentity() (systemProxyWarmIdentity, bool) {
	if s.sup == nil {
		return systemProxyWarmIdentity{}, false
	}
	s.runtimeMu.Lock()
	runtime := s.runtime
	configPath := s.cfg.ConfigPath
	s.runtimeMu.Unlock()
	status := s.sup.Status()
	if status.State != supervisor.StateRunning || status.PID <= 0 || runtime.TUNEnabled ||
		runtime.RevisionStatus != "active" || runtime.RevisionID == "" ||
		runtime.ConfigHash == "" || configPath == "" ||
		activeOutboundID(runtime) != runtime.SelectedOutbound {
		return systemProxyWarmIdentity{}, false
	}
	return systemProxyWarmIdentity{
		coreID: runtime.CoreID, selectedOutbound: runtime.SelectedOutbound,
		revisionID: runtime.RevisionID, configHash: runtime.ConfigHash,
		configPath: configPath, hostConfigHash: status.ConfigHash, pid: status.PID,
	}, true
}

func (s *Service) systemProxyCoreHealthy(ctx context.Context) bool {
	if s.host == nil {
		return false
	}
	health := s.host.HealthCheck(ctx)
	return health != nil && health.Healthy && health.ProcessOK && health.PortOK
}

func (s *Service) rememberSystemProxyWarmLocked(verification RuntimeRoutingVerification) {
	identity, ok := s.currentSystemProxyWarmIdentity()
	if !ok {
		s.invalidateSystemProxyWarmLocked()
		return
	}
	s.warmMu.Lock()
	s.stopWarmTimerLocked()
	s.warmGeneration++
	s.systemProxyWarm.remember(identity, verification)
	s.warmMu.Unlock()
}

func (s *Service) retainSystemProxyWarmLocked(
	ctx context.Context,
) (RuntimeRoutingVerification, time.Time, bool) {
	if s.warmTTL <= 0 {
		return RuntimeRoutingVerification{}, time.Time{}, false
	}
	identity, ok := s.currentSystemProxyWarmIdentity()
	if !ok || !s.systemProxyCoreHealthy(ctx) {
		return RuntimeRoutingVerification{}, time.Time{}, false
	}
	status := s.sup.Status()
	now := s.warmNow()
	s.warmMu.Lock()
	if !s.systemProxyWarm.activeMatches(identity, status) ||
		!s.systemProxyWarm.retain(now, s.warmTTL) {
		s.warmMu.Unlock()
		return RuntimeRoutingVerification{}, time.Time{}, false
	}
	s.warmGeneration++
	generation := s.warmGeneration
	verification := s.systemProxyWarm.verification
	expiresAt := s.systemProxyWarm.expiresAt
	s.stopWarmTimerLocked()
	s.warmTimer = time.AfterFunc(s.warmTTL, func() {
		s.expireSystemProxyWarm(generation)
	})
	s.warmMu.Unlock()
	s.sup.SetWarmIdle(true)
	return verification, expiresAt, true
}

func (s *Service) resumeSystemProxyWarmLocked(
	ctx context.Context,
) (RuntimeRoutingVerification, time.Duration, bool) {
	identity, ok := s.currentSystemProxyWarmIdentity()
	if !ok || !s.systemProxyCoreHealthy(ctx) {
		return RuntimeRoutingVerification{}, 0, false
	}
	status := s.sup.Status()
	now := s.warmNow()
	s.warmMu.Lock()
	if !s.systemProxyWarm.reusable(identity, status, now) {
		s.warmMu.Unlock()
		return RuntimeRoutingVerification{}, 0, false
	}
	remaining := s.systemProxyWarm.remaining(now)
	verification := s.systemProxyWarm.verification
	s.systemProxyWarm.activate()
	s.warmGeneration++
	s.stopWarmTimerLocked()
	s.warmMu.Unlock()
	s.sup.SetWarmIdle(false)
	return verification, remaining, true
}

func (s *Service) invalidateSystemProxyWarmLocked() {
	s.warmMu.Lock()
	s.warmGeneration++
	s.stopWarmTimerLocked()
	s.systemProxyWarm.clear()
	s.warmMu.Unlock()
	if s.sup != nil {
		s.sup.SetWarmIdle(false)
	}
}

func (s *Service) stopWarmTimerLocked() {
	if s.warmTimer != nil {
		s.warmTimer.Stop()
		s.warmTimer = nil
	}
}

func (s *Service) expireSystemProxyWarm(generation uint64) {
	if !s.captureMu.TryLock() {
		time.AfterFunc(25*time.Millisecond, func() {
			s.expireSystemProxyWarm(generation)
		})
		return
	}
	defer s.captureMu.Unlock()

	now := s.warmNow()
	s.warmMu.Lock()
	if generation != s.warmGeneration || !s.systemProxyWarm.idle {
		s.warmMu.Unlock()
		return
	}
	remaining := s.systemProxyWarm.remaining(now)
	if remaining > 0 {
		s.stopWarmTimerLocked()
		s.warmTimer = time.AfterFunc(remaining, func() {
			s.expireSystemProxyWarm(generation)
		})
		s.warmMu.Unlock()
		return
	}
	s.warmGeneration++
	s.stopWarmTimerLocked()
	s.systemProxyWarm.clear()
	s.warmMu.Unlock()

	s.sup.SetWarmIdle(false)
	if s.sup.State() == supervisor.StateStopped {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), captureRollbackTimeout)
	defer cancel()
	if err := s.stopCoreForCapture(ctx); err != nil {
		log.Printf("[service] stop expired System Proxy warm core: %v", err)
		return
	}
	log.Printf("[service] System Proxy warm core expired and stopped")
}
