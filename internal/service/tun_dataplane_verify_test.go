//go:build windows

package service

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDirectHTTPTransportDisablesExplicitAndEnvironmentProxy(t *testing.T) {
	transport := directHTTPTransport()
	defer transport.CloseIdleConnections()
	if transport.Proxy != nil {
		t.Fatal("TUN data-plane transport can use an explicit or environment proxy")
	}
}

func TestFetchPublicIPUsesBoundedNoProxyClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("203.0.113.9\n"))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := fetchPublicIPFromEndpoints(ctx, directHTTPTransport(), []string{server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got != "203.0.113.9" {
		t.Fatalf("public IP = %q", got)
	}
}

func TestParseCloudflareTraceIP(t *testing.T) {
	got, err := parseCloudflareTraceIP([]byte("fl=1\nip=203.0.113.11\nts=1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "203.0.113.11" {
		t.Fatalf("trace IP = %q", got)
	}
	if _, err := parseCloudflareTraceIP([]byte("fl=1\nip=not-an-ip\n")); err == nil {
		t.Fatal("invalid trace IP was accepted")
	}
}

func TestVerifyUDPDNSRequiresMatchingResponse(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() {
		buffer := make([]byte, 512)
		n, remote, readErr := server.ReadFrom(buffer)
		if readErr != nil || n < 12 {
			return
		}
		response := append([]byte(nil), buffer[:n]...)
		response[2] |= 0x80
		binary.BigEndian.PutUint16(response[6:8], 0)
		_, _ = server.WriteTo(response, remote)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := verifyUDPDNS(ctx, server.LocalAddr().String(), "session"); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredExternalSitesCoverChatGPTApplicationFlow(t *testing.T) {
	got := make(map[string]string, len(requiredExternalSites))
	for _, probe := range requiredExternalSites {
		got[probe.Name] = probe.URL
	}
	expected := map[string]string{
		"google":         "https://www.google.com/generate_204",
		"github":         "https://github.com/",
		"chatgpt-web":    "https://chatgpt.com/",
		"chatgpt-auth":   "https://auth.openai.com/",
		"openai-api":     "https://api.openai.com/v1/models",
		"chatgpt-assets": "https://persistent.oaistatic.com/",
		"chatgpt-stream": "https://ws.chatgpt.com/",
	}
	if len(got) != len(expected) {
		t.Fatalf("external TUN probes = %#v", got)
	}
	for name, wantURL := range expected {
		if got[name] != wantURL {
			t.Fatalf("probe %s URL = %q, want %q", name, got[name], wantURL)
		}
	}
}

func TestVerifyExternalSiteUsesDNSTCPAndHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := verifyExternalSite(ctx, externalSiteProbe{Name: "local", URL: server.URL, ExpectedStatus: []int{http.StatusNoContent}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DNS || !result.TCP || !result.HTTPS || result.StatusCode != http.StatusNoContent {
		t.Fatalf("incomplete site verification: %#v", result)
	}
}

func TestVerifyExternalSitesDoesNotBurstConcurrentUpstreamRequests(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	probes := []externalSiteProbe{
		{Name: "first", URL: server.URL, ExpectedStatus: []int{http.StatusNoContent}},
		{Name: "second", URL: server.URL, ExpectedStatus: []int{http.StatusNoContent}},
		{Name: "third", URL: server.URL, ExpectedStatus: []int{http.StatusNoContent}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := verifyExternalSites(ctx, probes)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(probes) {
		t.Fatalf("verified sites = %d, want %d", len(result), len(probes))
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent site probes = %d, want 1", maximum.Load())
	}
}

func TestVerifyExternalSiteRejectsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := verifyExternalSite(ctx, externalSiteProbe{
		Name: "openai-api", URL: server.URL, ExpectedStatus: []int{http.StatusUnauthorized},
	})
	if err == nil {
		t.Fatal("unexpected HTTP status was accepted")
	}
}

func TestRetryTUNProbeRecoversTransientReadOnlyFailure(t *testing.T) {
	attempts := 0
	value, err := retryTUNProbe(
		context.Background(), 3, time.Second, time.Millisecond,
		func(context.Context) (string, error) {
			attempts++
			if attempts == 1 {
				return "", io.ErrUnexpectedEOF
			}
			return "healthy", nil
		},
	)
	if err != nil || value != "healthy" || attempts != 2 {
		t.Fatalf("retry result = %q, attempts=%d, err=%v", value, attempts, err)
	}
}

func TestLookupSystemResolverWithRetryRecoversAfterColdStartTimeout(t *testing.T) {
	attempts := 0
	addresses, err := lookupSystemResolverWithRetry(
		context.Background(), "www.cloudflare.com",
		func(context.Context, string) ([]net.IPAddr, error) {
			attempts++
			if attempts == 1 {
				return nil, context.DeadlineExceeded
			}
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
		},
	)
	if err != nil || len(addresses) != 1 || attempts != 2 {
		t.Fatalf("resolver result = %#v, attempts=%d, err=%v", addresses, attempts, err)
	}
}

func TestLookupSystemResolverWithRetryFailsAfterBoundedExhaustion(t *testing.T) {
	attempts := 0
	_, err := lookupSystemResolverWithRetry(
		context.Background(), "www.cloudflare.com",
		func(context.Context, string) ([]net.IPAddr, error) {
			attempts++
			return nil, context.DeadlineExceeded
		},
	)
	if err == nil || attempts != 3 {
		t.Fatalf("attempts=%d, err=%v", attempts, err)
	}
}
