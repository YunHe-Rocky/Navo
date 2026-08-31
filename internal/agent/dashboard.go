package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"navo/internal/domain/capture"
	"navo/internal/ipdetect"
)

const ipProbeCooldown = 5 * time.Minute

const (
	connectionKindDirect              = "direct"
	connectionKindNavo                = "navo"
	connectionKindExternalSystemProxy = "external_system_proxy"
)

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
	proxyAddress := ""
	if proxyStatus.Enabled {
		proxyAddress = proxyStatus.ProxyServer
	}
	proxyServer, proxyPort := dashboardProxyEndpoint(proxyAddress, a.cfg.ProxyPort)

	probeKey, externalProxyURL, connectionKind, probeTargetErr := a.dashboardIPProbeTarget(runtimeStatus)
	probePending := a.scheduleIPProbe(probeKey, externalProxyURL, connectionKind, probeTargetErr)
	ipStatus := a.dashboardIPStatus(runtimeStatus, probeKey, connectionKind, probePending, probeTargetErr)
	return agentResponse(requestID, map[string]interface{}{
		"core":        payloads["core.status"],
		"cores":       coreList["cores"],
		"runtime":     runtimeStatus,
		"tun":         payloads["tun.status"],
		"metrics":     payloads["metrics.current"],
		"capture":     a.captureStatusPayload(),
		"environment": a.environmentSnapshot(),
		"proxy": map[string]interface{}{
			"enabled": proxyStatus.Enabled,
			"server":  proxyServer,
			"port":    proxyPort,
		},
		"ip": ipStatus,
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

func (a *Agent) dashboardIPProbeTarget(
	runtimeStatus map[string]interface{},
) (key, proxyURL, connectionKind string, targetErr error) {
	captureState := a.captureSnapshot()
	activeID, _ := runtimeStatus["active_id"].(string)
	if captureState.CommittedMode != capture.ModeOff {
		connectionKind = connectionKindNavo
		key = fmt.Sprintf("%s:%s:%s", connectionKind, captureState.CommittedMode, strings.TrimSpace(activeID))
		return
	}

	environment := a.environmentSnapshot()
	raw := environment.SystemProxy
	if raw.Enabled && raw.Ownership == "external" {
		connectionKind = connectionKindExternalSystemProxy
		key = connectionKind + ":" + strings.ToLower(strings.TrimSpace(raw.ProxyServer))
		proxyURL, targetErr = normalizeExternalSystemProxyURL(raw.ProxyServer)
		return
	}
	connectionKind = connectionKindDirect
	key = connectionKind
	return
}

func (a *Agent) handleIPCheck(ctx context.Context, requestID string) map[string]interface{} {
	key, proxyURL, connectionKind, targetErr := a.dashboardIPProbeTarget(nil)
	if connectionKind != connectionKindExternalSystemProxy {
		response, err := a.SendToServiceContext(ctx, map[string]interface{}{
			"request_id": requestID,
			"method":     "ip.check",
		})
		if err != nil {
			return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
		}
		return response
	}
	if targetErr != nil {
		return agentError(requestID, "EXTERNAL_PROXY_ENDPOINT_INVALID", targetErr)
	}
	result, err := a.externalIPCheck(ctx, proxyURL)
	if err != nil {
		return agentError(requestID, "EXTERNAL_PROXY_IP_CHECK_FAILED", err)
	}
	payload := dualIPPayload(result, connectionKindExternalSystemProxy)
	a.rememberIPProbe(key, payload, time.Now())
	return agentResponse(requestID, payload)
}

func (a *Agent) scheduleIPProbe(
	key, externalProxyURL, connectionKind string,
	targetErr error,
) bool {
	a.ipProbeMu.Lock()
	if a.ipProbeRunning {
		a.ipProbeMu.Unlock()
		return true
	}
	if !a.lastIPProbe.IsZero() && time.Since(a.lastIPProbe) < ipProbeCooldown &&
		(a.lastIPProbeKey == "" || a.lastIPProbeKey == key) {
		a.ipProbeMu.Unlock()
		return false
	}
	a.ipProbeRunning = true
	a.ipProbeMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var payload map[string]interface{}
		var probeErr error
		if connectionKind == connectionKindExternalSystemProxy {
			if targetErr != nil {
				probeErr = targetErr
				payload = unavailableExternalIPPayload(targetErr)
			} else {
				var result ipdetect.DualIPResult
				result, probeErr = a.externalIPCheck(ctx, externalProxyURL)
				if probeErr == nil {
					payload = dualIPPayload(result, connectionKindExternalSystemProxy)
				}
			}
		} else {
			response := a.callServiceContext(ctx, "background-ip-check", "ip.check")
			if isErrorResponse(response) {
				probeErr = fmt.Errorf("%s", responseMessage(response))
			} else {
				payload, _ = response["payload"].(map[string]interface{})
				if payload != nil {
					payload = cloneIPPayload(payload)
					payload["connection_kind"] = connectionKind
				}
			}
		}
		if probeErr != nil {
			log.Printf("[agent] background exit IP check failed: %v", probeErr)
		}
		a.ipProbeMu.Lock()
		a.ipProbeRunning = false
		a.lastIPProbe = time.Now()
		a.lastIPProbeKey = key
		if payload != nil {
			a.ipProbePayload = cloneIPPayload(payload)
		}
		a.ipProbeMu.Unlock()
	}()
	return true
}

func (a *Agent) rememberIPProbe(key string, payload map[string]interface{}, checkedAt time.Time) {
	a.ipProbeMu.Lock()
	a.lastIPProbe = checkedAt
	a.lastIPProbeKey = key
	a.ipProbePayload = cloneIPPayload(payload)
	a.ipProbeMu.Unlock()
}

func (a *Agent) dashboardIPStatus(
	runtimeStatus map[string]interface{},
	probeKey, connectionKind string,
	probePending bool,
	targetErr error,
) map[string]interface{} {
	result := map[string]interface{}{
		"connection_kind":   connectionKind,
		"proxy_ip":          "",
		"proxy_country":     "",
		"direct_ip":         "",
		"proxy_error":       "",
		"direct_error":      "",
		"proxy_provider":    "",
		"direct_provider":   "",
		"proxy_checked_at":  "",
		"direct_checked_at": "",
		"probe_pending":     probePending,
	}
	if connectionKind != connectionKindExternalSystemProxy {
		result["proxy_ip"] = runtimeStatus["exit_ip"]
		result["proxy_country"] = runtimeStatus["exit_country"]
	}

	a.ipProbeMu.Lock()
	cacheMatches := a.lastIPProbeKey == probeKey
	cached := cloneIPPayload(a.ipProbePayload)
	a.ipProbeMu.Unlock()
	if cacheMatches {
		mergeDashboardIPResult(result, "direct", nestedIPResult(cached, "source"))
		mergeDashboardIPResult(result, "proxy", nestedIPResult(cached, "proxy"))
	}
	if targetErr != nil && strings.TrimSpace(stringValue(result["proxy_error"])) == "" {
		result["proxy_error"] = targetErr.Error()
	}
	return result
}

func mergeDashboardIPResult(target map[string]interface{}, prefix string, source map[string]interface{}) {
	if source == nil {
		return
	}
	target[prefix+"_ip"] = source["ip"]
	target[prefix+"_error"] = source["error"]
	target[prefix+"_provider"] = source["provider"]
	target[prefix+"_checked_at"] = source["checked_at"]
	if prefix == "proxy" {
		target["proxy_country"] = source["country"]
	}
}

func nestedIPResult(payload map[string]interface{}, key string) map[string]interface{} {
	value, _ := payload[key].(map[string]interface{})
	return value
}

func cloneIPPayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(payload))
	for key, value := range payload {
		if nested, ok := value.(map[string]interface{}); ok {
			copyNested := make(map[string]interface{}, len(nested))
			for nestedKey, nestedValue := range nested {
				copyNested[nestedKey] = nestedValue
			}
			cloned[key] = copyNested
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func normalizeExternalSystemProxyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("external System Proxy endpoint is empty")
	}
	candidate, proxyScheme := raw, "http"
	if strings.Contains(raw, "=") {
		entries := make(map[string]string)
		for _, entry := range strings.Split(raw, ";") {
			key, value, found := strings.Cut(strings.TrimSpace(entry), "=")
			if found && strings.TrimSpace(value) != "" {
				entries[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
		switch {
		case entries["https"] != "":
			candidate = entries["https"]
		case entries["http"] != "":
			candidate = entries["http"]
		case entries["socks"] != "":
			candidate, proxyScheme = entries["socks"], "socks5"
		default:
			return "", fmt.Errorf("external System Proxy has no supported HTTP, HTTPS, or SOCKS endpoint")
		}
	}
	if !strings.Contains(candidate, "://") {
		candidate = proxyScheme + "://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("parse external System Proxy endpoint: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" {
		return "", fmt.Errorf("external System Proxy scheme %q is unsupported", parsed.Scheme)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("external System Proxy credentials are not accepted")
	}
	if parsed.Hostname() == "" || parsed.Port() == "" || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("external System Proxy endpoint must contain only host and port")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("external System Proxy port is invalid")
	}
	return scheme + "://" + parsed.Host, nil
}

func checkExternalSystemProxyIP(
	ctx context.Context,
	proxyURL string,
) (ipdetect.DualIPResult, error) {
	if _, err := url.ParseRequestURI(proxyURL); err != nil {
		return ipdetect.DualIPResult{}, fmt.Errorf("parse external proxy URL: %w", err)
	}
	directDetector := ipdetect.NewDetector()
	proxyDetector := ipdetect.NewDetectorWithProxy(proxyURL)
	var result ipdetect.DualIPResult
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		result.Source = checkIPDetector(ctx, directDetector, "source")
	}()
	go func() {
		defer wait.Done()
		result.Proxy = checkIPDetector(ctx, proxyDetector, "external-system-proxy")
	}()
	wait.Wait()
	return result, nil
}

func checkIPDetector(ctx context.Context, detector *ipdetect.Detector, outboundID string) *ipdetect.IPResult {
	result, err := detector.CheckFresh(ctx, outboundID)
	if err == nil {
		return result
	}
	return &ipdetect.IPResult{OutboundID: outboundID, Error: err.Error(), CheckedAt: time.Now()}
}

func dualIPPayload(result ipdetect.DualIPResult, connectionKind string) map[string]interface{} {
	return map[string]interface{}{
		"connection_kind": connectionKind,
		"source":          ipResultPayload(result.Source, false),
		"proxy":           ipResultPayload(result.Proxy, false),
	}
}

func unavailableExternalIPPayload(err error) map[string]interface{} {
	checkedAt := time.Now()
	return map[string]interface{}{
		"connection_kind": connectionKindExternalSystemProxy,
		"source": ipResultPayload(&ipdetect.IPResult{
			OutboundID: "source", Error: "direct baseline was not checked", CheckedAt: checkedAt,
		}, false),
		"proxy": ipResultPayload(&ipdetect.IPResult{
			OutboundID: "external-system-proxy", Error: err.Error(), CheckedAt: checkedAt,
		}, false),
	}
}

func ipResultPayload(result *ipdetect.IPResult, inactive bool) map[string]interface{} {
	if result == nil {
		result = &ipdetect.IPResult{CheckedAt: time.Now(), Error: "IP check returned no result"}
	}
	available := net.ParseIP(strings.TrimSpace(result.IP)) != nil && result.Error == ""
	state := "unavailable"
	if inactive {
		state = "inactive"
	} else if available {
		state = "available"
	}
	return map[string]interface{}{
		"available": available, "state": state, "outbound_id": result.OutboundID,
		"ip": result.IP, "country": result.Country, "city": result.City,
		"asn": result.ASN, "isp": result.ISP, "network": result.Network,
		"provider": result.Provider, "mobile": result.Mobile, "proxy": result.Proxy,
		"hosting": result.Hosting, "checked_at": result.CheckedAt, "error": result.Error,
	}
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}
