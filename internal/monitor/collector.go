package monitor

import (
	"sync"
	"time"
)

// TrafficStats holds traffic statistics for an outbound.
type TrafficStats struct {
	OutboundID string `json:"outbound_id"`
	Upload     int64  `json:"upload_bytes"`
	Download   int64  `json:"download_bytes"`
	Connections int   `json:"connections"`
}

// RuleHit record tracks which rules are being triggered.
type RuleHit struct {
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	ProcessName string    `json:"process_name,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	OutboundID  string    `json:"outbound_id"`
	Count       int64     `json:"count"`
	LastHit     time.Time `json:"last_hit"`
}

// Collector aggregates passive monitoring data.
type Collector struct {
	mu         sync.RWMutex
	stats      map[string]*TrafficStats
	ruleHits   map[string]*RuleHit
}

// NewCollector creates a new Collector.
func NewCollector() *Collector {
	return &Collector{
		stats:    make(map[string]*TrafficStats),
		ruleHits: make(map[string]*RuleHit),
	}
}

// RecordTraffic updates traffic stats for an outbound.
func (c *Collector) RecordTraffic(outboundID string, upload, download int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	st, ok := c.stats[outboundID]
	if !ok {
		st = &TrafficStats{OutboundID: outboundID}
		c.stats[outboundID] = st
	}
	st.Upload += upload
	st.Download += download
}

// RecordConnection updates active connection count.
func (c *Collector) RecordConnection(outboundID string, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	st, ok := c.stats[outboundID]
	if !ok {
		st = &TrafficStats{OutboundID: outboundID}
		c.stats[outboundID] = st
	}
	st.Connections += delta
	if st.Connections < 0 {
		st.Connections = 0
	}
}

// RecordRuleHit tracks a rule hit.
func (c *Collector) RecordRuleHit(ruleID, ruleName, processName, domain, outboundID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := ruleID + ":" + outboundID
	hit, ok := c.ruleHits[key]
	if !ok {
		hit = &RuleHit{
			RuleID:      ruleID,
			RuleName:    ruleName,
			ProcessName: processName,
			Domain:      domain,
			OutboundID:  outboundID,
		}
		c.ruleHits[key] = hit
	}
	hit.Count++
	hit.LastHit = time.Now()
}

// Stats returns a snapshot of current traffic stats.
func (c *Collector) Stats() []TrafficStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]TrafficStats, 0, len(c.stats))
	for _, st := range c.stats {
		result = append(result, *st)
	}
	return result
}

// RuleHits returns a snapshot of rule hit counters.
func (c *Collector) RuleHits() []RuleHit {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]RuleHit, 0, len(c.ruleHits))
	for _, h := range c.ruleHits {
		result = append(result, *h)
	}
	return result
}

// Reset clears all statistics.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = make(map[string]*TrafficStats)
	c.ruleHits = make(map[string]*RuleHit)
}
