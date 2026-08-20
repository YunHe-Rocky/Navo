package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"navo/internal/compiler"
)

const (
	failoverProbeTimeout     = 3 * time.Second
	failoverProbeConcurrency = 4
)

type failoverProbeResult struct {
	OutboundID string `json:"outbound_id"`
	SourceType string `json:"source_type"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Reachable  bool   `json:"reachable"`
	Error      string `json:"error,omitempty"`
}

func outboundSourceType(outbound compiler.Outbound) string {
	if outbound.ProviderID == "upstream_proxy" {
		return "upstream_proxy"
	}
	return "airport_subscription"
}

func sameChannelFailoverPool(outbounds []compiler.Outbound, activeID string, coreID string) (string, []compiler.Outbound, []failoverProbeResult, error) {
	var active *compiler.Outbound
	for index := range outbounds {
		if outbounds[index].ID == activeID {
			active = &outbounds[index]
			break
		}
	}
	if active == nil {
		return "", nil, nil, fmt.Errorf("active outbound %q was not found", activeID)
	}

	sourceType := outboundSourceType(*active)
	eligible := make([]compiler.Outbound, 0, len(outbounds))
	rejected := make([]failoverProbeResult, 0)
	for _, outbound := range outbounds {
		if outbound.ID == activeID || outboundSourceType(outbound) != sourceType {
			continue
		}
		result := failoverProbeResult{OutboundID: outbound.ID, SourceType: sourceType}
		switch {
		case !outbound.Enabled:
			result.Error = "endpoint is disabled"
			rejected = append(rejected, result)
		case !compiler.Compatible(coreID, outbound):
			result.Error = fmt.Sprintf("%s does not support protocol %s", coreID, outbound.Type)
			rejected = append(rejected, result)
		default:
			eligible = append(eligible, outbound)
		}
	}
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].OutboundID < rejected[j].OutboundID })
	return sourceType, eligible, rejected, nil
}

func (s *Service) probeFailoverCandidates(ctx context.Context, sourceType string, candidates []compiler.Outbound) ([]failoverProbeResult, []failoverProbeResult) {
	results := make([]failoverProbeResult, 0, len(candidates))
	var resultsMu sync.Mutex
	var workers sync.WaitGroup
	limit := make(chan struct{}, failoverProbeConcurrency)

	for _, candidate := range candidates {
		candidate := candidate
		workers.Add(1)
		go func() {
			defer workers.Done()
			select {
			case limit <- struct{}{}:
				defer func() { <-limit }()
			case <-ctx.Done():
				resultsMu.Lock()
				results = append(results, failoverProbeResult{OutboundID: candidate.ID, SourceType: sourceType, Error: ctx.Err().Error()})
				resultsMu.Unlock()
				return
			}

			probeCtx, cancel := context.WithTimeout(ctx, failoverProbeTimeout)
			defer cancel()
			result := failoverProbeResult{OutboundID: candidate.ID, SourceType: sourceType}
			if s.prober == nil {
				result.Error = "outbound prober is unavailable"
			} else if probe := s.prober.ProbeTCP(probeCtx, candidate.ID, candidate.Server, candidate.Port); probe == nil {
				result.Error = "outbound probe returned no result"
			} else {
				result.Reachable = probe.Healthy
				result.LatencyMS = probe.Latency.Milliseconds()
				result.Error = probe.Error
				s.recordEndpointProbe(candidate.ID, probe.Healthy, probe.Error, probe.Latency)
			}
			resultsMu.Lock()
			results = append(results, result)
			resultsMu.Unlock()
		}()
	}
	workers.Wait()

	reachable := make([]failoverProbeResult, 0, len(results))
	rejected := make([]failoverProbeResult, 0, len(results))
	for _, result := range results {
		if result.Reachable {
			reachable = append(reachable, result)
		} else {
			if result.Error == "" {
				result.Error = "connection test failed"
			}
			rejected = append(rejected, result)
		}
	}
	sort.Slice(reachable, func(i, j int) bool {
		if reachable[i].LatencyMS == reachable[j].LatencyMS {
			return reachable[i].OutboundID < reachable[j].OutboundID
		}
		return reachable[i].LatencyMS < reachable[j].LatencyMS
	})
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].OutboundID < rejected[j].OutboundID })
	return reachable, rejected
}

func (s *Service) handleOutboundFailoverCandidates(ctx context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	activeID, _ := msg["active_id"].(string)
	if activeID == "" {
		s.runtimeMu.Lock()
		activeID = activeOutboundID(s.runtime)
		s.runtimeMu.Unlock()
	}
	if activeID == "" {
		return errorResponse(requestID, "FAILOVER_ACTIVE_REQUIRED", fmt.Errorf("active outbound is required"))
	}

	sourceType, pool, rejected, err := sameChannelFailoverPool(s.currentOutbounds(ctx), activeID, s.host.ID())
	if err != nil {
		return errorResponse(requestID, "FAILOVER_ACTIVE_NOT_FOUND", err)
	}
	reachable, probeRejected := s.probeFailoverCandidates(ctx, sourceType, pool)
	rejected = append(rejected, probeRejected...)
	sort.Slice(rejected, func(i, j int) bool { return rejected[i].OutboundID < rejected[j].OutboundID })
	return response(requestID, map[string]interface{}{
		"active_id": activeID, "source_type": sourceType,
		"candidates": reachable, "rejected": rejected,
	})
}
