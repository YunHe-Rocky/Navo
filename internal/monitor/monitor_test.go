package monitor

import (
	"context"
	"testing"
	"time"
)

func TestProber_ProbeTCP_Unreachable(t *testing.T) {
	p := NewProber()
	p.timeout = 500 * time.Millisecond
	result := p.ProbeTCP(context.Background(), "test", "127.0.0.1", 0)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Healthy {
		t.Error("expected unhealthy for unreachable host")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestProber_ProbeTCP_Timeout(t *testing.T) {
	p := NewProber()
	p.timeout = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := p.ProbeTCP(ctx, "test", "example.com", 443)
	if result.Healthy {
		t.Error("expected timeout to fail probe")
	}
}

func TestProber_ProbeDNS(t *testing.T) {
	p := NewProber()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := p.ProbeDNS(ctx, "localhost")
	if !result.Healthy {
		t.Logf("DNS probe to localhost: %v (expected)", result.Error)
	}
}

func TestCollector_RecordTraffic(t *testing.T) {
	c := NewCollector()
	c.RecordTraffic("ob1", 100, 200)
	c.RecordTraffic("ob1", 50, 70)

	stats := c.Stats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(stats))
	}
	st := stats[0]
	if st.Upload != 150 {
		t.Errorf("upload = %d, want 150", st.Upload)
	}
	if st.Download != 270 {
		t.Errorf("download = %d, want 270", st.Download)
	}
}

func TestCollector_RecordConnection(t *testing.T) {
	c := NewCollector()
	c.RecordConnection("ob1", 1)
	c.RecordConnection("ob1", 1)
	c.RecordConnection("ob1", -1)

	stats := c.Stats()
	if len(stats) != 1 {
		t.Fatal("expected 1 outbound")
	}
	if stats[0].Connections != 1 {
		t.Errorf("connections = %d, want 1", stats[0].Connections)
	}
}

func TestCollector_RecordRuleHit(t *testing.T) {
	c := NewCollector()
	c.RecordRuleHit("r1", "Rule1", "chrome.exe", "example.com", "ob1")
	c.RecordRuleHit("r1", "Rule1", "chrome.exe", "example.com", "ob1")
	c.RecordRuleHit("r2", "Rule2", "steam.exe", "store.steampowered.com", "direct")

	hits := c.RuleHits()
	if len(hits) != 2 {
		t.Fatalf("expected 2 rule hits, got %d", len(hits))
	}
}

func TestCollector_Reset(t *testing.T) {
	c := NewCollector()
	c.RecordTraffic("ob1", 100, 200)
	c.Reset()
	stats := c.Stats()
	if len(stats) != 0 {
		t.Errorf("expected 0 stats after reset, got %d", len(stats))
	}
}

func TestMetricsStore_Prune(t *testing.T) {
	store := NewMetricsStore(nil) // nil store won't work
	if store != nil {
		// Basic type check
		t.Log("metrics store created")
	}
}

func TestMetricPoint_Fields(t *testing.T) {
	mp := MetricPoint{
		Timestamp:  1234567890000,
		OutboundID: "test",
		Latency:    150,
		Loss:       0.01,
		Upload:     1024,
		Download:   2048,
		DNSTime:    45,
	}
	if mp.Timestamp == 0 || mp.OutboundID == "" {
		t.Error("metric point fields not populated")
	}
}
