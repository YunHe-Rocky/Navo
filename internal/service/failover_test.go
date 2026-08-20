package service

import (
	"context"
	"testing"
	"time"

	"navo/internal/compiler"
	"navo/internal/monitor"
)

type failoverTestProber struct {
	results map[string]*monitor.ProbeResult
}

func (p failoverTestProber) ProbeTCP(_ context.Context, id string, _ string, _ int) *monitor.ProbeResult {
	return p.results[id]
}

func TestSameChannelFailoverPoolEnforcesSourceAndCompatibility(t *testing.T) {
	outbounds := []compiler.Outbound{
		{ID: "airport-active", ProviderID: "sub-a", Type: compiler.OutboundVLESS, Enabled: true},
		{ID: "airport-fast", ProviderID: "sub-b", Type: compiler.OutboundVLESS, Enabled: true},
		{ID: "airport-disabled", ProviderID: "sub-c", Type: compiler.OutboundVLESS},
		{ID: "airport-incompatible", ProviderID: "sub-d", Type: compiler.OutboundWireGuard, Enabled: true},
		{ID: "upstream-cross-channel", ProviderID: "upstream_proxy", Type: compiler.OutboundSOCKS, Enabled: true},
	}

	sourceType, eligible, rejected, err := sameChannelFailoverPool(outbounds, "airport-active", compiler.CoreXray)
	if err != nil {
		t.Fatal(err)
	}
	if sourceType != "airport_subscription" {
		t.Fatalf("source type = %q", sourceType)
	}
	if len(eligible) != 1 || eligible[0].ID != "airport-fast" {
		t.Fatalf("eligible = %#v", eligible)
	}
	if len(rejected) != 2 || rejected[0].OutboundID != "airport-disabled" || rejected[1].OutboundID != "airport-incompatible" {
		t.Fatalf("rejected = %#v", rejected)
	}
}

func TestProbeFailoverCandidatesRanksReachableAndRejectsFailures(t *testing.T) {
	svc := &Service{
		prober: failoverTestProber{results: map[string]*monitor.ProbeResult{
			"slow": {Healthy: true, Latency: 80 * time.Millisecond},
			"fast": {Healthy: true, Latency: 12 * time.Millisecond},
			"down": {Error: "i/o timeout", Latency: 3 * time.Second},
		}},
		endpointStatuses: make(map[string]EndpointStatus),
	}
	candidates := []compiler.Outbound{
		{ID: "slow", Server: "slow.example", Port: 443},
		{ID: "down", Server: "down.example", Port: 443},
		{ID: "fast", Server: "fast.example", Port: 443},
	}

	reachable, rejected := svc.probeFailoverCandidates(context.Background(), "airport_subscription", candidates)
	if len(reachable) != 2 || reachable[0].OutboundID != "fast" || reachable[1].OutboundID != "slow" {
		t.Fatalf("reachable = %#v", reachable)
	}
	if len(rejected) != 1 || rejected[0].OutboundID != "down" || rejected[0].Reachable {
		t.Fatalf("rejected = %#v", rejected)
	}
	if status := svc.endpointStatuses["fast"]; !status.Available || status.LatencyMS != 12 {
		t.Fatalf("endpoint status = %#v", status)
	}
}
