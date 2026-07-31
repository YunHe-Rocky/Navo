package agent

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

const ipProbeCooldown = 5 * time.Minute

func (a *Agent) handleDashboardSnapshot(requestID string) map[string]interface{} {
	payloads := make(map[string]map[string]interface{}, 5)
	for _, method := range []string{
		"core.status",
		"core.list",
		"runtime.status",
		"tun.status",
		"metrics.current",
	} {
		response := a.callService(requestID, method)
		if isErrorResponse(response) {
			return response
		}
		payload, ok := response["payload"].(map[string]interface{})
		if !ok {
			return agentError(
				requestID,
				"DASHBOARD_INVALID_RESPONSE",
				fmt.Errorf("%s returned an invalid payload", method),
			)
		}
		payloads[method] = payload
	}

	runtimeStatus := payloads["runtime.status"]
	coreList := payloads["core.list"]
	proxyStatus := a.ProxyStatus()
	a.refreshCaptureFault(payloads["tun.status"])
	proxyServer, proxyPort := dashboardProxyEndpoint(proxyStatus.ProxyServer, a.cfg.ProxyPort)

	probePending := a.scheduleIPProbe()
	return agentResponse(requestID, map[string]interface{}{
		"core":    payloads["core.status"],
		"cores":   coreList["cores"],
		"runtime": runtimeStatus,
		"tun":     payloads["tun.status"],
		"metrics": payloads["metrics.current"],
		"capture": a.captureStatusPayload(),
		"proxy": map[string]interface{}{
			"enabled": proxyStatus.Enabled,
			"server":  proxyServer,
			"port":    proxyPort,
		},
		"ip": map[string]interface{}{
			"proxy_ip":      runtimeStatus["exit_ip"],
			"proxy_country": runtimeStatus["exit_country"],
			"direct_ip":     "",
			"probe_pending": probePending,
		},
	})
}

func dashboardProxyEndpoint(address string, fallbackPort int) (string, int) {
	host, rawPort, err := net.SplitHostPort(address)
	if err == nil {
		port, parseErr := strconv.Atoi(rawPort)
		if parseErr == nil && port > 0 {
			return host, port
		}
	}
	return "127.0.0.1", fallbackPort
}

func (a *Agent) scheduleIPProbe() bool {
	a.ipProbeMu.Lock()
	if a.ipProbeRunning {
		a.ipProbeMu.Unlock()
		return true
	}
	if !a.lastIPProbe.IsZero() && time.Since(a.lastIPProbe) < ipProbeCooldown {
		a.ipProbeMu.Unlock()
		return false
	}
	a.ipProbeRunning = true
	a.ipProbeMu.Unlock()

	go func() {
		response := a.callService("background-ip-check", "ip.check")
		if isErrorResponse(response) {
			log.Printf("[agent] background exit IP check failed: %s", responseMessage(response))
		}
		a.ipProbeMu.Lock()
		a.ipProbeRunning = false
		a.lastIPProbe = time.Now()
		a.ipProbeMu.Unlock()
	}()
	return true
}
