//go:build windows

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var requiredDirectRuntimeSites = []externalSiteProbe{
	{Name: "baidu", URL: "https://www.baidu.com/", ExpectedStatus: []int{http.StatusOK}},
	{Name: "xiaomi", URL: "https://connect.rom.miui.com/generate_204", ExpectedStatus: []int{http.StatusNoContent}},
}

func runtimeRoutingVerificationSites(mode string) []externalSiteProbe {
	if mode == runtimeModeDirect {
		return append([]externalSiteProbe(nil), requiredDirectRuntimeSites...)
	}
	probes := make([]externalSiteProbe, 0, len(genericProxySites)+len(chatGPTApplicationSites))
	probes = append(probes, genericProxySites...)
	return append(probes, chatGPTApplicationSites...)
}

func verifyRuntimeRouting(ctx context.Context, proxyPort int, mode string) (RuntimeRoutingVerification, error) {
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	if err != nil {
		return RuntimeRoutingVerification{}, err
	}
	directIP, err := fetchCloudflareTraceIPWithRetry(ctx, func() *http.Transport {
		return directHTTPTransport()
	})
	if err != nil {
		return RuntimeRoutingVerification{}, proxyRoutingVerificationError(
			"direct_exit_ip", "bounded direct public IP", "unavailable", err,
		)
	}
	proxyExitIP, err := fetchCloudflareTraceIPWithRetry(ctx, func() *http.Transport {
		transport := directHTTPTransport()
		transport.Proxy = http.ProxyURL(proxyURL)
		return transport
	})
	if err != nil {
		return RuntimeRoutingVerification{}, proxyRoutingVerificationError(
			"proxy_exit_ip", "bounded public IP through local proxy", "unavailable", err,
		)
	}
	if err := validateRuntimeExitIdentity(mode, directIP, proxyExitIP); err != nil {
		return RuntimeRoutingVerification{
			DirectIP: directIP, ProxyExitIP: proxyExitIP, VerifiedAt: time.Now().UTC(),
		}, err
	}
	type probeResult struct {
		verification SiteVerification
		err          error
	}
	probes := runtimeRoutingVerificationSites(mode)
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
					return verifyExternalSiteViaProxy(probeCtx, probe, proxyURL)
				},
			)
		}()
	}
	wait.Wait()
	verification := RuntimeRoutingVerification{
		Verified: true, DirectIP: directIP, ProxyExitIP: proxyExitIP,
		Sites: make(map[string]SiteVerification, len(results)), VerifiedAt: time.Now().UTC(),
	}
	for index, probe := range probes {
		verification.Sites[probe.Name] = results[index].verification
		if results[index].err != nil {
			verification.Verified = false
			return verification, results[index].err
		}
	}
	return verification, nil
}

func verifyExternalSiteViaProxy(ctx context.Context, probe externalSiteProbe, proxyURL *url.URL) (SiteVerification, error) {
	result := SiteVerification{}
	transport := directHTTPTransport()
	transport.Proxy = http.ProxyURL(proxyURL)
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.URL, nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("User-Agent", "Navo-Routing-Health/1")
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return result, proxyRoutingVerificationError(
			probe.Name, "TLS and HTTP through local proxy", "failed", err,
		)
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	closeErr := response.Body.Close()
	result.StatusCode = response.StatusCode
	if !acceptedProbeStatus(response.StatusCode, probe.ExpectedStatus) || readErr != nil || closeErr != nil {
		return result, proxyRoutingVerificationError(
			probe.Name, "valid bounded HTTP response through local proxy",
			fmt.Sprintf("status=%d", response.StatusCode), errorsJoin(readErr, closeErr),
		)
	}
	// A successful HTTPS CONNECT proves the core resolved the destination and
	// established the TCP/TLS path; the client intentionally performs neither
	// operation outside the proxy.
	result.DNS, result.TCP, result.HTTPS = true, true, true
	return result, nil
}
