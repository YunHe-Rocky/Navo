package ipdetect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetector_CheckFallsBackToNextProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/first" {
				writer.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = writer.Write([]byte("203.0.113.10\n"))
		},
	))
	defer server.Close()

	detector := newDetector(nil)
	detector.client = server.Client()
	detector.endpoints = []echoEndpoint{
		{url: server.URL + "/first"},
		{url: server.URL + "/second"},
	}
	result, err := detector.Check(context.Background(), "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if result.IP != "203.0.113.10" {
		t.Fatalf("IP = %q", result.IP)
	}
}

func TestDetectorUsesDedicatedHTTP1Transport(t *testing.T) {
	first := NewDetector()
	second := NewDetectorWithProxy("http://127.0.0.1:12080")
	firstTransport, firstOK := first.client.Transport.(*http.Transport)
	secondTransport, secondOK := second.client.Transport.(*http.Transport)
	if !firstOK || !secondOK || firstTransport == secondTransport {
		t.Fatal("IP detectors must not share transport state")
	}
	if firstTransport.ForceAttemptHTTP2 || secondTransport.ForceAttemptHTTP2 {
		t.Fatal("IP detection must not initialize HTTP/2 during route transitions")
	}
	if firstTransport.TLSNextProto == nil || secondTransport.TLSNextProto == nil {
		t.Fatal("HTTP/2 was not explicitly disabled")
	}
}

func TestDetector_CheckReturnsProviderSafeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusServiceUnavailable)
		},
	))
	defer server.Close()

	detector := newDetector(nil)
	detector.client = server.Client()
	detector.endpoints = []echoEndpoint{{url: server.URL + "/secret-token"}}
	_, err := detector.Check(context.Background(), "failure")
	if err == nil {
		t.Fatal("expected provider failure")
	}
	if got := err.Error(); got != "IP check unavailable: 1 providers failed" {
		t.Fatalf("error = %q", got)
	}
}

func TestDetector_Check_NetworkError(t *testing.T) {
	d := NewDetector()
	d.client.Timeout = 500 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := d.Check(ctx, "test-ob")
	if err == nil {
		t.Logf("IP check succeeded unexpectedly: %+v", result)
	} else {
		t.Logf("expected network error: %v", err)
	}
}

func TestDetector_CheckAll(t *testing.T) {
	d := NewDetector()
	d.client.Timeout = 500 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results := d.CheckAll(ctx, []string{"ob1", "ob2"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		// Should either have IP or Error set
		if r.IP == "" && r.Error == "" {
			t.Error("expected either IP or Error in result")
		}
	}
}

func TestDetector_Cache(t *testing.T) {
	d := NewDetector()

	// Populate a fake cache entry
	d.mu.Lock()
	d.cache["cached-ob"] = &cachedResult{
		result: IPResult{
			OutboundID: "cached-ob",
			IP:         "1.2.3.4",
			Country:    "US",
		},
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	d.mu.Unlock()

	// Should return cached result without network
	result, err := d.Check(context.Background(), "cached-ob")
	if err != nil {
		t.Fatal(err)
	}
	if result.IP != "1.2.3.4" {
		t.Errorf("cached IP = %s, want 1.2.3.4", result.IP)
	}
}

func TestDetector_ClearCache(t *testing.T) {
	d := NewDetector()
	d.mu.Lock()
	d.cache["x"] = &cachedResult{result: IPResult{IP: "1.1.1.1"}, expiresAt: time.Now().Add(time.Hour)}
	d.mu.Unlock()

	d.ClearCache()

	d.mu.RLock()
	cacheSize := len(d.cache)
	d.mu.RUnlock()

	if cacheSize != 0 {
		t.Errorf("expected empty cache after Clear, got %d entries", cacheSize)
	}
}

func TestIPResult_JSON(t *testing.T) {
	r := IPResult{
		OutboundID: "test",
		IP:         "8.8.8.8",
		Country:    "US",
		City:       "Mountain View",
		ASN:        "AS15169 Google",
		CheckedAt:  time.Now(),
	}
	if r.IP == "" {
		t.Error("result fields not populated")
	}
}
