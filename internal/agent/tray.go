package agent

import (
	"fmt"
	"time"
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

func (a *Agent) handleConnectionEnable(requestID string) map[string]interface{} {
	runtimeStatus := a.callService(requestID, "runtime.status")
	if isErrorResponse(runtimeStatus) {
		return runtimeStatus
	}
	runtimePayload, _ := runtimeStatus["payload"].(map[string]interface{})
	mode, _ := runtimePayload["mode"].(string)
	activeID, _ := runtimePayload["active_id"].(string)
	if mode != "direct" && activeID == "" {
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
	return a.setCaptureMode(requestID, map[string]interface{}{"mode": "system_proxy"})
}

func (a *Agent) handleConnectionDisable(requestID string) map[string]interface{} {
	if resp := a.setCaptureMode(
		requestID,
		map[string]interface{}{"mode": "off"},
	); isErrorResponse(resp) {
		return resp
	}
	return agentResponse(requestID, map[string]interface{}{"status": "disconnected"})
}

func (a *Agent) handleNetworkRecover(requestID string) map[string]interface{} {
	if resp := a.setCaptureMode(
		requestID,
		map[string]interface{}{"mode": "off"},
	); isErrorResponse(resp) {
		return resp
	}
	return a.callService(requestID, "network.recover")
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
