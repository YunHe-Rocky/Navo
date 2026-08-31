package agent

import (
	"context"
	"fmt"
	"time"

	"navo/internal/connection"
	"navo/internal/domain/capture"
)

func (a *Agent) handleTraySnapshot(requestID string) map[string]interface{} {
	methods := []string{
		"core.status",
		"core.list",
		"runtime.status",
		"outbound.list",
		"tun.status",
		"metrics.current",
		"subscription.list",
	}
	payload := make(map[string]interface{}, len(methods)+1)
	for _, method := range methods {
		resp := a.callService(requestID, method)
		if isErrorResponse(resp) {
			return resp
		}
		payload[method] = resp["payload"]
	}
	payload["proxy.status"] = a.ProxyStatus()
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"payload":    payload,
	}
}

func (a *Agent) handleConnectionEnable(ctx context.Context, requestID string) map[string]interface{} {
	runtimeStatus := a.callServiceContext(ctx, requestID, "runtime.status")
	if isErrorResponse(runtimeStatus) {
		return runtimeStatus
	}
	runtimePayload, _ := runtimeStatus["payload"].(map[string]interface{})
	selectedID, _ := runtimePayload["selected_id"].(string)
	if selectedID == "" {
		selectedID, _ = runtimePayload["candidate_id"].(string)
	}
	if selectedID == "" {
		selectedID, _ = runtimePayload["active_id"].(string)
	}
	if selectedID == "" {
		return agentError(
			requestID,
			"ACTIVE_SELECTION_REQUIRED",
			fmt.Errorf("select an endpoint before enabling the proxy"),
		)
	}
	status := a.captureSnapshot()
	if status.CommittedMode != "off" {
		return agentResponse(requestID, map[string]interface{}{"status": "connected"})
	}
	// Generic connect promises application-wide capture, matching the main UI.
	// System Proxy remains an explicit advanced choice for proxy-aware apps.
	return a.setCaptureModeContext(ctx, requestID, map[string]interface{}{"mode": "tun"})
}

func (a *Agent) handleConnectionDisable(ctx context.Context, requestID string) map[string]interface{} {
	if resp := a.setCaptureModeContext(
		ctx,
		requestID,
		map[string]interface{}{"mode": "off"},
	); isErrorResponse(resp) {
		return resp
	}
	return agentResponse(requestID, map[string]interface{}{"status": "disconnected"})
}

func (a *Agent) handleConnectionRestart(
	ctx context.Context,
	requestID string,
) (result map[string]interface{}) {
	transaction, err := a.beginConnection(
		ctx, requestID, connection.OperationRecovery, connection.OriginUser, "capture",
	)
	if err != nil {
		return connectionAdmissionResponse(requestID, "CONNECTION_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}

	target := a.captureSnapshot().CommittedMode
	if target == capture.ModeOff {
		return agentResponse(requestID, map[string]interface{}{"status": "disconnected"})
	}
	if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
		return agentError(requestID, "CAPTURE_RESTART_FAILED", err)
	}
	_ = transaction.SetPhase(connection.PhaseVerifying)
	if err := a.transitionCaptureModeLocked(ctx, target); err != nil {
		return agentError(requestID, "CAPTURE_RESTART_FAILED", err)
	}
	return agentResponse(requestID, map[string]interface{}{"status": "connected"})
}

func (a *Agent) handleNetworkRecover(
	ctx context.Context,
	requestID string,
) (result map[string]interface{}) {
	transaction, err := a.beginConnection(
		ctx, requestID, connection.OperationRecovery, connection.OriginUser, "network",
	)
	if err != nil {
		return connectionAdmissionResponse(requestID, "CONNECTION_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}

	if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
		return agentError(requestID, "NETWORK_RECOVERY_STOP_FAILED", err)
	}
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID,
		"method":     "network.recover",
	})
	if err != nil {
		return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	return response
}

func agentResponse(requestID string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "RESPONSE",
		"timestamp":  time.Now().UnixMilli(),
		"payload":    payload,
	}
}

func trayRequest(method string) map[string]interface{} {
	return map[string]interface{}{
		"request_id": fmt.Sprintf("tray-%d", time.Now().UnixNano()),
		"method":     method,
		"type":       "REQUEST",
	}
}
