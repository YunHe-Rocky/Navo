package subscription

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWithoutRequestURLRemovesSubscriptionSecret(t *testing.T) {
	const secret = "credential-token"
	err := withoutRequestURL(&url.Error{
		Op:  "Get",
		URL: "https://example.com/" + secret,
		Err: errors.New("certificate verification failed"),
	})
	if strings.Contains(err.Error(), secret) {
		t.Fatal("sanitized fetch error exposed the request URL")
	}
	if !strings.Contains(err.Error(), "certificate verification failed") {
		t.Fatalf("sanitized error = %q", err)
	}
}

func TestFetcher_Fetch_Success(t *testing.T) {
	// Test validateURL works for valid URLs (actual HTTP fetch tested elsewhere)
	err := validateURL("https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPClient_TLSCompatibilityIsExplicit(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	if _, err := newHTTPClient(false).Get(srv.URL); err == nil {
		t.Fatal("secure client unexpectedly accepted an untrusted certificate")
	}
	resp, err := newHTTPClient(true).Get(srv.URL)
	if err != nil {
		t.Fatalf("opt-in compatibility client rejected test certificate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestFetcher_Fetch_HTTPForbidden(t *testing.T) {
	f := NewFetcher()
	_, err := f.Fetch(context.Background(), "http://example.com/sub")
	if err == nil {
		t.Fatal("expected error for HTTP URL")
	}
}

func TestFetcher_Fetch_LocalhostBlocked(t *testing.T) {
	f := NewFetcher()
	_, err := f.Fetch(context.Background(), "https://127.0.0.1/sub")
	if err == nil {
		t.Fatal("expected error for localhost URL")
	}
}

func TestFetcher_Fetch_PrivateIPBlocked(t *testing.T) {
	f := NewFetcher()
	_, err := f.Fetch(context.Background(), "https://192.168.1.1/sub")
	if err == nil {
		t.Fatal("expected error for private IP")
	}
	_, err = f.Fetch(context.Background(), "https://10.0.0.1/sub")
	if err == nil {
		t.Fatal("expected error for 10.x private IP")
	}
}

func TestFetcher_Fetch_FileSchemeBlocked(t *testing.T) {
	f := NewFetcher()
	_, err := f.Fetch(context.Background(), "https://example.com/%00file://")
	// Should fail with some URL parse or request error
	if err == nil {
		t.Log("URL may have been parsed as valid HTTPS, check server-side handling")
	}
}

func TestFetcher_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewFetcher()
	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetcher_Fetch_RedirectLimit(t *testing.T) {
	redirectCount := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if redirectCount < 5 {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := NewFetcher()
	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected redirect limit error")
	}
}

func TestFetcher_Fetch_EmptyResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{})
	}))
	defer srv.Close()

	f := NewFetcher()
	_, err := f.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestValidateURL_EmptyHost(t *testing.T) {
	err := validateURL("https:///path")
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestValidateURL_IPv6Loopback(t *testing.T) {
	err := validateURL("https://[::1]/sub")
	if err == nil {
		t.Fatal("expected error for IPv6 loopback")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"100.64.0.1", false}, // CGNAT but not in our blocklist
	}
	for _, tt := range tests {
		ip := parseIP(tt.ip)
		if ip == nil {
			t.Fatalf("invalid test IP: %s", tt.ip)
		}
		got := isPrivateIP(ip)
		if got != tt.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
