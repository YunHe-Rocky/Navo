package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
	"navo/internal/networkenv"
)

const (
	environmentObservationInterval = 5 * time.Second
	environmentObservationTimeout  = 5 * time.Second
	environmentEndpointTimeout     = 1200 * time.Millisecond
)

func (a *Agent) environmentSnapshot() networkenv.Snapshot {
	return a.environmentStore.Load()
}

func (a *Agent) requestEnvironmentRefresh() {
	if a == nil || a.environmentRefresh == nil {
		return
	}
	select {
	case a.environmentRefresh <- struct{}{}:
	default:
	}
}

func (a *Agent) monitorEnvironment(ctx context.Context) {
	ticker := time.NewTicker(environmentObservationInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
		case <-a.environmentRefresh:
		}
		a.refreshEnvironment(ctx)
	}
}

func (a *Agent) refreshEnvironment(parent context.Context) networkenv.Snapshot {
	a.environmentRefreshMu.Lock()
	defer a.environmentRefreshMu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, environmentObservationTimeout)
	defer cancel()

	collectedAt := time.Now().UTC()
	captureState := a.captureSnapshot()
	transaction := a.coordinator.Snapshot()
	snapshot := networkenv.Snapshot{
		Version: networkenv.SnapshotVersion, CollectedAt: collectedAt,
		Transition: networkenv.TransitionSnapshot{
			Busy: transaction.Busy, ID: transaction.ID,
			Operation: string(transaction.Operation), Intent: string(transaction.Intent),
			Domain: string(transaction.Domain), Priority: transaction.Priority,
			Phase:       string(transaction.Phase),
			FaultDomain: transaction.FaultDomain,
		},
		Capture: networkenv.CaptureSnapshot{
			State: string(captureState.State), DesiredMode: string(captureState.DesiredMode),
			CommittedMode: string(captureState.CommittedMode), FaultID: captureState.FaultID,
			ReadinessState: captureState.Readiness.State, ReadinessError: captureState.Readiness.Error,
		},
	}

	machine, machineErrors := a.collectMachineEnvironment(ctx)
	snapshot.Physical = machine.Physical
	snapshot.TUN = machine.TUN
	snapshot.DNS = machine.DNS
	snapshot.Routes = machine.Routes
	snapshot.NRPT = machine.NRPT
	snapshot.Firewall = machine.Firewall
	snapshot.Journal = machine.Journal
	snapshot.ObservationErrors = append(snapshot.ObservationErrors, machine.ObservationErrors...)
	snapshot.ObservationErrors = append(snapshot.ObservationErrors, machineErrors...)

	raw, rawErr := a.currentProxy()
	ownership := a.proxy.OwnershipStatus()
	snapshot.SystemProxy = systemProxyEnvironment(raw, ownership, a.cfg.ProxyPort)
	if rawErr != nil {
		snapshot.SystemProxy.LastError = rawErr.Error()
		snapshot.ObservationErrors = append(snapshot.ObservationErrors, "read WinINet: "+rawErr.Error())
	}
	if ownership.LastError != "" {
		snapshot.SystemProxy.LastError = joinEnvironmentErrors(snapshot.SystemProxy.LastError, ownership.LastError)
		snapshot.ObservationErrors = append(snapshot.ObservationErrors, ownership.LastError)
	}
	if ownership.Owned && rawErr == nil {
		snapshot.SystemProxy.ReachableKnown = true
		probeCtx, probeCancel := context.WithTimeout(ctx, environmentEndpointTimeout)
		snapshot.SystemProxy.Reachable = probeLocalProxyEndpoint(probeCtx, raw.ProxyServer) == nil
		probeCancel()
	}
	if ctx.Err() != nil {
		snapshot.ObservationErrors = appendUniqueEnvironmentError(snapshot.ObservationErrors, ctx.Err().Error())
	}
	snapshot = networkenv.Analyze(snapshot)
	a.environmentStore.Publish(snapshot)
	return snapshot
}

func (a *Agent) collectMachineEnvironment(ctx context.Context) (networkenv.MachineSnapshot, []string) {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("environment-observe-%d", time.Now().UnixNano()),
		"method":     "network.observe",
	})
	if err != nil {
		return networkenv.MachineSnapshot{}, []string{"Service network observation: " + err.Error()}
	}
	if isErrorResponse(response) {
		return networkenv.MachineSnapshot{}, []string{"Service network observation: " + responseMessage(response)}
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return networkenv.MachineSnapshot{}, []string{"Service network observation returned an invalid payload"}
	}
	value, exists := payload["environment"]
	if !exists {
		return networkenv.MachineSnapshot{}, []string{"Service network observation omitted environment"}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return networkenv.MachineSnapshot{}, []string{"encode Service network observation: " + err.Error()}
	}
	var machine networkenv.MachineSnapshot
	if err := json.Unmarshal(encoded, &machine); err != nil {
		return networkenv.MachineSnapshot{}, []string{"decode Service network observation: " + err.Error()}
	}
	return machine, nil
}

func systemProxyEnvironment(
	raw systemproxy.ProxyConfig,
	ownership systemproxy.OwnershipStatus,
	proxyPort int,
) networkenv.SystemProxySnapshot {
	expectedEndpoint := fmt.Sprintf("127.0.0.1:%d", proxyPort)
	result := networkenv.SystemProxySnapshot{
		Enabled: raw.Enabled, ProxyServer: raw.ProxyServer, AutoDetect: raw.AutoDetect,
		AutoConfigConfigured: strings.TrimSpace(raw.AutoConfigURL) != "",
		BypassConfigured:     strings.TrimSpace(raw.BypassList) != "",
		OwnedByNavo:          ownership.Owned, OwnershipMarker: ownership.Present,
		OwnershipLost: ownership.Lost,
		LocalEndpoint: strings.EqualFold(strings.TrimSpace(raw.ProxyServer), expectedEndpoint),
		Ownership:     networkenv.OwnerNone,
	}
	switch {
	case ownership.Owned:
		result.Ownership = networkenv.OwnerNavo
	case raw.Enabled:
		result.Ownership = networkenv.OwnerExternal
	case ownership.Present:
		result.Ownership = networkenv.OwnerUnknown
	}
	return result
}

func probeLocalProxyEndpoint(ctx context.Context, endpoint string) error {
	if strings.TrimSpace(endpoint) == "" {
		return fmt.Errorf("proxy endpoint is empty")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	return connection.Close()
}

func joinEnvironmentErrors(left, right string) string {
	if strings.TrimSpace(left) == "" {
		return strings.TrimSpace(right)
	}
	if strings.TrimSpace(right) == "" {
		return strings.TrimSpace(left)
	}
	return left + "; " + right
}

func appendUniqueEnvironmentError(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func (a *Agent) environmentFinding(code string) (networkenv.Finding, bool) {
	snapshot := a.environmentStore.Load()
	if snapshot.Stale {
		return networkenv.Finding{}, false
	}
	for _, finding := range snapshot.Findings {
		if finding.Code == code {
			return finding, true
		}
	}
	return networkenv.Finding{}, false
}

func (a *Agent) handleEnvironmentRepair(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	code, _ := msg["code"].(string)
	code = strings.TrimSpace(code)
	if code == "" {
		return agentError(requestID, "ENVIRONMENT_FINDING_REQUIRED", fmt.Errorf("environment finding code is required"))
	}
	snapshot := a.refreshEnvironment(ctx)
	var finding networkenv.Finding
	found := false
	for _, candidate := range snapshot.Findings {
		if candidate.Code == code {
			finding, found = candidate, true
			break
		}
	}
	switch {
	case snapshot.Stale:
		return agentError(requestID, "ENVIRONMENT_STALE", fmt.Errorf("network environment snapshot is stale"))
	case !found:
		return agentError(requestID, "ENVIRONMENT_FINDING_CLEARED", fmt.Errorf("finding %s is no longer present", code))
	case snapshot.Transition.Busy || finding.Transitional:
		return agentError(requestID, "ENVIRONMENT_TRANSITION_ACTIVE", fmt.Errorf("finding %s belongs to an active transition", code))
	case !finding.Recoverable || finding.Ownership != networkenv.OwnerNavo:
		return agentError(requestID, "ENVIRONMENT_NOT_OWNED", fmt.Errorf("finding %s is not a Navo-owned recoverable resource", code))
	}
	mode := a.captureSnapshot().CommittedMode
	if mode == capture.ModeOff {
		desired := a.captureSnapshot().DesiredMode
		if desired == capture.ModeTUN || desired == capture.ModeSystemProxy {
			mode = desired
		}
	}
	fault := attributedFaultFromFinding(mode, finding)
	if err := a.recoverUnhealthyCapture(mode, fault); err != nil {
		a.refreshEnvironment(ctx)
		return agentError(requestID, "ENVIRONMENT_REPAIR_FAILED", err)
	}
	return agentResponse(requestID, map[string]interface{}{
		"environment": a.refreshEnvironment(ctx),
	})
}
