package networkenv

import (
	"testing"
	"time"
)

func TestStoreReturnsDefensiveCopies(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Publish(Snapshot{
		CollectedAt: now, Health: HealthHealthy,
		Physical:          PhysicalSnapshot{Known: true, Available: true, ActiveInterfaces: []string{"Ethernet"}},
		TUN:               TUNSnapshot{External: []ExternalAdapterRef{{Name: "VPN"}}},
		Findings:          []Finding{{Code: FindingExternalTUNPresent}},
		ObservationErrors: []string{"sample"},
	})

	first := store.LoadAt(now)
	first.Physical.ActiveInterfaces[0] = "changed"
	first.TUN.External[0].Name = "changed"
	first.Findings[0].Code = "changed"
	first.ObservationErrors[0] = "changed"

	second := store.LoadAt(now)
	if second.Physical.ActiveInterfaces[0] != "Ethernet" || second.TUN.External[0].Name != "VPN" ||
		second.Findings[0].Code != FindingExternalTUNPresent || second.ObservationErrors[0] != "sample" {
		t.Fatalf("Store leaked mutable state: %+v", second)
	}
}

func TestStoreMarksExpiredSnapshotStaleWithoutMutatingStoredValue(t *testing.T) {
	collected := time.Now().UTC()
	store := NewStore()
	store.Publish(Snapshot{CollectedAt: collected, Health: HealthHealthy})

	expired := store.LoadAt(collected.Add(SnapshotStaleAfter + time.Millisecond))
	if !expired.Stale || expired.Health != HealthChecking {
		t.Fatalf("expired snapshot = %+v", expired)
	}
	fresh := store.LoadAt(collected.Add(time.Second))
	if fresh.Stale || fresh.Health != HealthHealthy {
		t.Fatalf("stored snapshot was mutated by stale read: %+v", fresh)
	}
}

func TestNewStoreStartsChecking(t *testing.T) {
	snapshot := NewStore().Load()
	if snapshot.Health != HealthChecking || !snapshot.Stale || snapshot.Version != SnapshotVersion {
		t.Fatalf("initial snapshot = %+v", snapshot)
	}
}
