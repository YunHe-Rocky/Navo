//go:build windows

package service

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"navo/internal/network"
)

type windowsTUNDataPlaneVerifier struct{}

type externalSiteProbe struct {
	Name           string
	URL            string
	ExpectedStatus []int
}

var requiredExternalSites = []externalSiteProbe{
	{Name: "google", URL: "https://www.google.com/generate_204", ExpectedStatus: []int{http.StatusNoContent}},
	{Name: "github", URL: "https://github.com/", ExpectedStatus: []int{http.StatusOK}},
}

func newTUNDataPlaneVerifier() TUNDataPlaneVerifier {
	return &windowsTUNDataPlaneVerifier{}
}

func (*windowsTUNDataPlaneVerifier) CaptureDirectIP(ctx context.Context) (string, error) {
	return fetchPublicIP(ctx, directIPv4HTTPTransport())
}

func (*windowsTUNDataPlaneVerifier) Verify(ctx context.Context, request VerifyRequest) (VerifyResult, error) {
	result := VerifyResult{DirectIP: request.DirectIP, UDP: UDPUnsupported, VerifiedAt: time.Now().UTC()}
	if net.ParseIP(request.DirectIP) == nil {
		return result, &network.TUNError{Code: network.ErrTUNExitIPMismatch, Stage: network.TUNStageDataPlaneVerified, Resource: "direct_ip", Expected: "valid pre-TUN public IP", Actual: request.DirectIP}
	}

	dnsStarted := time.Now()
	addresses, dnsErr := lookupSystemResolverWithRetry(ctx, "www.cloudflare.com", net.DefaultResolver.LookupIPAddr)
	result.DNSLatency = time.Since(dnsStarted)
	result.DNSLatencyMS = result.DNSLatency.Milliseconds()
	if dnsErr != nil || len(addresses) == 0 {
		return result, &network.TUNError{Code: network.ErrTUNDNSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: "system_resolver", Expected: "at least one address", Actual: fmt.Sprintf("addresses=%d", len(addresses)), Cause: dnsErr}
	}
	result.DNS = true

	dialCtx, dialCancel := context.WithTimeout(ctx, 6*time.Second)
	connection, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", "www.cloudflare.com:443")
	dialCancel()
	if err != nil {
		return result, &network.TUNError{Code: network.ErrTUNTCPVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: "www.cloudflare.com:443", Expected: "TCP connection", Actual: "failed", Cause: err}
	}
	_ = connection.Close()
	result.TCP = true
	if tunCrashPoint() == "during-dataplane" {
		os.Exit(91)
	}

	result.ExitIP, err = fetchCloudflareTraceIPWithRetry(ctx, func() *http.Transport {
		return directHTTPTransport()
	})
	if err != nil {
		return result, &network.TUNError{Code: network.ErrTUNHTTPSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: "www.cloudflare.com", Expected: "TLS and HTTP response after bounded retries", Actual: "failed", Cause: err}
	}
	result.HTTPS = true
	if request.DirectMode {
		if result.ExitIP != request.DirectIP {
			return result, &network.TUNError{Code: network.ErrTUNExitIPMismatch, Stage: network.TUNStageDataPlaneVerified, Resource: "direct_mode_exit", Expected: request.DirectIP, Actual: result.ExitIP}
		}
	} else {
		proxyURL, parseErr := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", request.ProxyPort))
		if parseErr != nil {
			return result, parseErr
		}
		result.ProxyExitIP, err = fetchCloudflareTraceIPWithRetry(ctx, func() *http.Transport {
			transport := directHTTPTransport()
			transport.Proxy = http.ProxyURL(proxyURL)
			return transport
		})
		if err != nil || result.ExitIP == request.DirectIP || result.ExitIP != result.ProxyExitIP {
			return result, &network.TUNError{Code: network.ErrTUNExitIPMismatch, Stage: network.TUNStageDataPlaneVerified, Resource: "proxy_exit_ip", Expected: fmt.Sprintf("TUN != %s and TUN == local-proxy", request.DirectIP), Actual: fmt.Sprintf("tun=%s proxy=%s", result.ExitIP, result.ProxyExitIP), Cause: err}
		}
		result.Sites, err = verifyExternalSites(ctx, requiredExternalSites)
		if err != nil {
			return result, err
		}
	}

	if !request.UDPRequired {
		result.UDP = UDPUnsupported
		result.UDPReason = "selected outbound does not declare UDP support"
		return result, nil
	}
	udpCtx, udpCancel := context.WithTimeout(ctx, 5*time.Second)
	err = verifyUDPDNS(udpCtx, net.JoinHostPort(request.TUNDNSIPv4, "53"), request.SessionID)
	udpCancel()
	if err != nil {
		result.UDP = UDPFailed
		result.UDPReason = err.Error()
		return result, &network.TUNError{Code: network.ErrTUNUDPVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: request.TUNDNSIPv4 + ":53", Expected: "DNS response over UDP", Actual: "failed", Cause: err}
	}
	result.UDP = UDPVerified
	return result, nil
}

func lookupSystemResolverWithRetry(
	ctx context.Context,
	host string,
	lookup func(context.Context, string) ([]net.IPAddr, error),
) ([]net.IPAddr, error) {
	// A newly activated TUN can briefly saturate the selected upstream while
	// Windows drains existing host traffic. DNS is a read-only readiness probe,
	// so retry it without retrying any network mutation.
	return retryTUNProbe(
		ctx, 3, 8*time.Second, 500*time.Millisecond,
		func(attemptCtx context.Context) ([]net.IPAddr, error) {
			addresses, err := lookup(attemptCtx, host)
			if err != nil {
				return nil, err
			}
			if len(addresses) == 0 {
				return nil, fmt.Errorf("system resolver returned no addresses for %s", host)
			}
			return addresses, nil
		},
	)
}

func verifyExternalSites(ctx context.Context, probes []externalSiteProbe) (map[string]SiteVerification, error) {
	type probeResult struct {
		verification SiteVerification
		err          error
	}
	results := make([]probeResult, len(probes))
	var wait sync.WaitGroup
	for index, probe := range probes {
		index, probe := index, probe
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index].verification, results[index].err = retryTUNProbe(
				ctx, 3, 10*time.Second, 500*time.Millisecond,
				func(probeCtx context.Context) (SiteVerification, error) {
					return verifyExternalSite(probeCtx, probe)
				},
			)
		}()
	}
	wait.Wait()
	verified := make(map[string]SiteVerification, len(probes))
	for index, probe := range probes {
		verified[probe.Name] = results[index].verification
		if results[index].err != nil {
			return verified, results[index].err
		}
	}
	return verified, nil
}

func fetchCloudflareTraceIPWithRetry(ctx context.Context, transportFactory func() *http.Transport) (string, error) {
	return retryTUNProbe(
		ctx, 3, 10*time.Second, 500*time.Millisecond,
		func(attemptCtx context.Context) (string, error) {
			return fetchCloudflareTraceIP(attemptCtx, transportFactory())
		},
	)
}

func retryTUNProbe[T any](
	ctx context.Context,
	attempts int,
	attemptTimeout time.Duration,
	delay time.Duration,
	probe func(context.Context) (T, error),
) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		value, err := probe(attemptCtx)
		cancel()
		if err == nil {
			return value, nil
		}
		lastErr = err
		if attempt == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return zero, errorsJoin(lastErr, ctx.Err())
		case <-timer.C:
		}
	}
	return zero, lastErr
}

func verifyExternalSite(ctx context.Context, probe externalSiteProbe) (SiteVerification, error) {
	result := SiteVerification{}
	parsed, err := url.Parse(probe.URL)
	if err != nil || parsed.Hostname() == "" {
		return result, &network.TUNError{Code: network.ErrTUNHTTPSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: probe.Name, Expected: "valid probe URL", Actual: probe.URL, Cause: err}
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	addresses, dnsErr := net.DefaultResolver.LookupIPAddr(ctx, host)
	if dnsErr != nil || len(addresses) == 0 {
		return result, &network.TUNError{Code: network.ErrTUNDNSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: host, Expected: "at least one address", Actual: fmt.Sprintf("addresses=%d", len(addresses)), Cause: dnsErr}
	}
	result.DNS = true
	connection, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if dialErr != nil {
		return result, &network.TUNError{Code: network.ErrTUNTCPVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: net.JoinHostPort(host, port), Expected: "TCP connection", Actual: "failed", Cause: dialErr}
	}
	_ = connection.Close()
	result.TCP = true
	transport := directHTTPTransport()
	defer transport.CloseIdleConnections()
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, probe.URL, nil)
	if requestErr != nil {
		return result, requestErr
	}
	request.Header.Set("User-Agent", "Navo-TUN-Health/1")
	response, requestErr := (&http.Client{Transport: transport}).Do(request)
	if requestErr != nil {
		return result, &network.TUNError{Code: network.ErrTUNHTTPSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: parsed.Host, Expected: "TLS and HTTP response", Actual: "failed", Cause: requestErr}
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	closeErr := response.Body.Close()
	result.StatusCode = response.StatusCode
	if !acceptedProbeStatus(response.StatusCode, probe.ExpectedStatus) || readErr != nil || closeErr != nil {
		return result, &network.TUNError{Code: network.ErrTUNHTTPSVerifyFailed, Stage: network.TUNStageDataPlaneVerified, Resource: parsed.Host, Expected: "valid bounded HTTP response", Actual: fmt.Sprintf("status=%d", response.StatusCode), Cause: errorsJoin(readErr, closeErr)}
	}
	result.HTTPS = true
	return result, nil
}

func acceptedProbeStatus(status int, expected []int) bool {
	if len(expected) == 0 {
		return status >= 100 && status <= 599
	}
	for _, candidate := range expected {
		if status == candidate {
			return true
		}
	}
	return false
}

func fetchCloudflareTraceIP(ctx context.Context, transport *http.Transport) (string, error) {
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil)
	if err != nil {
		return "", err
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("cloudflare trace returned HTTP %d", response.StatusCode)
	}
	return parseCloudflareTraceIP(body)
}

func parseCloudflareTraceIP(body []byte) (string, error) {
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "ip=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "ip="))
		if net.ParseIP(value) != nil {
			return value, nil
		}
		return "", fmt.Errorf("invalid cloudflare trace IP %q", value)
	}
	return "", fmt.Errorf("cloudflare trace response has no ip field")
}

func directHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}).DialContext,
		TLSHandshakeTimeout: 6 * time.Second, ResponseHeaderTimeout: 8 * time.Second,
	}
}

func directIPv4HTTPTransport() *http.Transport {
	transport := directHTTPTransport()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}
	transport.DialContext = func(ctx context.Context, _, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", address)
	}
	return transport
}

func fetchPublicIP(ctx context.Context, transport *http.Transport) (string, error) {
	return fetchPublicIPFromEndpoints(ctx, transport, []string{"https://api.ipify.org", "https://icanhazip.com"})
}

func fetchPublicIPFromEndpoints(ctx context.Context, transport *http.Transport, endpoints []string) (string, error) {
	defer transport.CloseIdleConnections()
	var lastErr error
	for _, endpoint := range endpoints {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		response, err := (&http.Client{Transport: transport}).Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 128))
		_ = response.Body.Close()
		value := strings.TrimSpace(string(data))
		if readErr == nil && response.StatusCode >= 200 && response.StatusCode < 300 && net.ParseIP(value) != nil {
			return value, nil
		}
		lastErr = fmt.Errorf("%s returned HTTP %d and %q: %w", endpoint, response.StatusCode, value, readErr)
	}
	return "", lastErr
}

func verifyUDPDNS(ctx context.Context, address, sessionID string) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "udp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	deadline, ok := ctx.Deadline()
	if ok {
		_ = connection.SetDeadline(deadline)
	}
	id := uint16(time.Now().UnixNano())
	label := strings.ToLower(sessionID)
	if len(label) > 20 {
		label = label[:20]
	}
	query := dnsQuery(id, label+".www.cloudflare.com")
	if _, err := connection.Write(query); err != nil {
		return err
	}
	response := make([]byte, 1500)
	n, err := connection.Read(response)
	if err != nil {
		return err
	}
	if n < 12 || binary.BigEndian.Uint16(response[:2]) != id || response[2]&0x80 == 0 {
		return fmt.Errorf("invalid DNS response")
	}
	return nil
}

func dnsQuery(id uint16, name string) []byte {
	message := make([]byte, 12, 512)
	binary.BigEndian.PutUint16(message[0:2], id)
	binary.BigEndian.PutUint16(message[2:4], 0x0100)
	binary.BigEndian.PutUint16(message[4:6], 1)
	for _, label := range strings.Split(name, ".") {
		message = append(message, byte(len(label)))
		message = append(message, label...)
	}
	message = append(message, 0, 0, 1, 0, 1)
	return message
}

func errorsJoin(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
