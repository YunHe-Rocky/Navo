package networkenv

import (
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func NewStore() *Store {
	return &Store{snapshot: Snapshot{
		Version:  SnapshotVersion,
		Health:   HealthChecking,
		Stale:    true,
		Findings: []Finding{},
	}}
}

func (s *Store) Publish(snapshot Snapshot) {
	if s == nil {
		return
	}
	if snapshot.Version == 0 {
		snapshot.Version = SnapshotVersion
	}
	snapshot.Stale = false
	s.mu.Lock()
	s.snapshot = cloneSnapshot(snapshot)
	s.mu.Unlock()
}

func (s *Store) Load() Snapshot {
	return s.LoadAt(time.Now().UTC())
}

func (s *Store) LoadAt(now time.Time) Snapshot {
	if s == nil {
		return Snapshot{Version: SnapshotVersion, Health: HealthChecking, Stale: true, Findings: []Finding{}}
	}
	s.mu.RLock()
	result := cloneSnapshot(s.snapshot)
	s.mu.RUnlock()
	if result.CollectedAt.IsZero() || now.Sub(result.CollectedAt) > SnapshotStaleAfter {
		result.Stale = true
		result.Health = HealthChecking
	}
	return result
}

func cloneSnapshot(value Snapshot) Snapshot {
	value.Physical.ActiveInterfaces = append([]string(nil), value.Physical.ActiveInterfaces...)
	value.TUN.External = append([]ExternalAdapterRef(nil), value.TUN.External...)
	value.Findings = append([]Finding(nil), value.Findings...)
	value.ObservationErrors = append([]string(nil), value.ObservationErrors...)
	if value.Findings == nil {
		value.Findings = []Finding{}
	}
	return value
}
