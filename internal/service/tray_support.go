package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"navo/internal/compiler"
	"navo/internal/network"
)

// EndpointStatus is the Service-owned availability view consumed by UI clients.
// Tray never derives or persists this state itself.
type EndpointStatus struct {
	EndpointID string    `json:"endpoint_id"`
	Available  bool      `json:"available"`
	Color      string    `json:"color"`
	Reason     string    `json:"reason,omitempty"`
	CheckedAt  time.Time `json:"checked_at,omitempty"`
	LatencyMS  int64     `json:"latency_ms,omitempty"`
}

func (s *Service) recordEndpointProbe(
	id string,
	available bool,
	probeError string,
	latency time.Duration,
) {
	reason := ""
	if !available {
		reason = probeError
		if reason == "" {
			reason = "connection test failed"
		}
	} else {
		reason = fmt.Sprintf("reachable in %d ms", latency.Milliseconds())
	}
	s.endpointStatusMu.Lock()
	s.endpointStatuses[id] = EndpointStatus{
		EndpointID: id,
		Available:  available,
		Color:      map[bool]string{true: "green", false: "red"}[available],
		Reason:     reason,
		CheckedAt:  time.Now().UTC(),
		LatencyMS:  latency.Milliseconds(),
	}
	s.endpointStatusMu.Unlock()
}

func (s *Service) endpointStatus(outbound compiler.Outbound) EndpointStatus {
	if !compiler.Compatible(s.host.ID(), outbound) {
		return EndpointStatus{
			EndpointID: outbound.ID,
			Color:      "red",
			Reason: fmt.Sprintf(
				"%s does not support protocol %s",
				s.host.ID(),
				outbound.Type,
			),
		}
	}
	if !outbound.Enabled {
		return EndpointStatus{
			EndpointID: outbound.ID,
			Color:      "red",
			Reason:     "endpoint is disabled",
		}
	}
	s.endpointStatusMu.RLock()
	status, ok := s.endpointStatuses[outbound.ID]
	s.endpointStatusMu.RUnlock()
	if ok {
		return status
	}
	return EndpointStatus{
		EndpointID: outbound.ID,
		Color:      "yellow",
		Reason:     "connection has not been tested",
	}
}

func (s *Service) handleNetworkRecover(
	ctx context.Context,
	requestID string,
) map[string]interface{} {
	if s.sup.State() == "running" {
		return errorResponse(
			requestID,
			"NETWORK_RECOVERY_REQUIRES_STOPPED_CORE",
			fmt.Errorf("stop the proxy core before network recovery"),
		)
	}
	result, err := s.reconciler.Reconcile(
		ctx,
		&network.ReconcileConfig{ListenPort: s.cfg.ProxyPort},
	)
	if err != nil {
		return errorResponse(requestID, "NETWORK_RECOVERY_FAILED", err)
	}
	return response(requestID, map[string]interface{}{
		"status":         "recovered",
		"issues_found":   result.IssuesFound,
		"issues_fixed":   result.IssuesFixed,
		"issues_unfixed": result.IssuesUnfixed,
	})
}

func (s *Service) handleDiagnosticsExport(
	_ context.Context,
	requestID string,
) map[string]interface{} {
	if s.cfg.ConfigDir == "" {
		return errorResponse(
			requestID,
			"DIAGNOSTICS_PATH_UNAVAILABLE",
			fmt.Errorf("configuration directory is unavailable"),
		)
	}
	s.runtimeMu.Lock()
	runtimeSnapshot := s.runtime
	s.runtimeMu.Unlock()
	coreStatus := s.sup.Status()
	payload := map[string]interface{}{
		"generated_at": time.Now().UTC(),
		"core": map[string]interface{}{
			"id": s.host.ID(), "state": coreStatus.State,
			"pid": coreStatus.PID, "last_error": coreStatus.LastError,
			"restart_count": coreStatus.RestartCount,
		},
		"runtime": map[string]interface{}{
			"mode":              runtimeSnapshot.Mode,
			"selected_outbound": runtimeSnapshot.SelectedOutbound,
			"tun_enabled":       runtimeSnapshot.TUNEnabled,
			"revision_id":       runtimeSnapshot.RevisionID,
			"revision_status":   runtimeSnapshot.RevisionStatus,
		},
		"counts": map[string]interface{}{
			"subscriptions": len(s.subMgr.List()),
			"endpoints":     len(s.currentOutbounds(context.Background())),
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return errorResponse(requestID, "DIAGNOSTICS_EXPORT_FAILED", err)
	}
	exportDir := filepath.Join(s.cfg.ConfigDir, "diagnostics")
	if err := os.MkdirAll(exportDir, 0700); err != nil {
		return errorResponse(requestID, "DIAGNOSTICS_EXPORT_FAILED", err)
	}
	path := filepath.Join(
		exportDir,
		fmt.Sprintf("navo-diagnostics-%s.json", time.Now().Format("20060102-150405")),
	)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return errorResponse(requestID, "DIAGNOSTICS_EXPORT_FAILED", err)
	}
	return response(requestID, map[string]interface{}{
		"status": "exported",
		"path":   path,
	})
}
