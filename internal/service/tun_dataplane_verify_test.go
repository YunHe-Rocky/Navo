//go:build windows

package service

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestRequiredExternalSitesCoverGenericDataPlaneOnly(t *testing.T) {
	got := make(map[string]string, len(requiredExternalSites))
	for _, probe := range requiredExternalSites {
		got[probe.Name] = probe.URL
	}
	if got["google"] != "https://www.google.com/generate_204" || got["github"] != "https://github.com/" {
		t.Fatalf("external TUN probes are incomplete: %#v", got)
	}
	if len(got) != 2 {
		t.Fatalf("product health must not gate proxy activation on application-specific services: %#v", got)
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
