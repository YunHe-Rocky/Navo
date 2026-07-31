package monitor

import (
	"fmt"
	"time"

	"navo/internal/storage"
)

// MetricPoint represents a single measurement at a point in time.
type MetricPoint struct {
	Timestamp  int64  `json:"timestamp"`
	OutboundID string `json:"outbound_id"`
	Latency    int64  `json:"latency_ms"`
	Loss       float64 `json:"loss_rate"`
	Upload     int64  `json:"upload_bytes"`
	Download   int64  `json:"download_bytes"`
	DNSTime    int64  `json:"dns_time_ms"`
}

// MetricsStore persists monitoring data using the existing storage.Store.
type MetricsStore struct {
	store *storage.Store
}

// NewMetricsStore creates a new MetricsStore backed by a JSON file.
func NewMetricsStore(store *storage.Store) *MetricsStore {
	return &MetricsStore{store: store}
}

// Save records a metric point for an outbound.
func (ms *MetricsStore) Save(obID string, point MetricPoint) error {
	key := fmt.Sprintf("metrics:%s:%d", obID, point.Timestamp)
	return ms.store.Put(key, point)
}

// Get retrieves a single metric point.
func (ms *MetricsStore) Get(obID string, timestamp int64) (MetricPoint, bool) {
	key := fmt.Sprintf("metrics:%s:%d", obID, timestamp)
	var mp MetricPoint
	if err := ms.store.Get(key, &mp); err != nil {
		return MetricPoint{}, false
	}
	return mp, true
}

// List returns metrics for an outbound within a time range.
func (ms *MetricsStore) List(obID string, since time.Time) []MetricPoint {
	keys := ms.store.Keys()
	prefix := fmt.Sprintf("metrics:%s:", obID)

	var result []MetricPoint
	for _, k := range keys {
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		var mp MetricPoint
		if err := ms.store.Get(k, &mp); err != nil {
			continue
		}
		if mp.Timestamp >= since.UnixMilli() {
			result = append(result, mp)
		}
	}
	return result
}

// Prune removes metrics older than the given duration.
func (ms *MetricsStore) Prune(maxAge time.Duration) int {
	keys := ms.store.Keys()
	cutoff := time.Now().Add(-maxAge).UnixMilli()
	pruned := 0

	for _, k := range keys {
		prefix := "metrics:"
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		var mp MetricPoint
		if err := ms.store.Get(k, &mp); err != nil {
			ms.store.Delete(k)
			pruned++
			continue
		}
		if mp.Timestamp < cutoff {
			ms.store.Delete(k)
			pruned++
		}
	}
	return pruned
}
