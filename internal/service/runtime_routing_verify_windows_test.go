//go:build windows

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRuntimeRoutingVerificationSitesMatchRouteSemantics(t *testing.T) {
	direct := runtimeRoutingVerificationSites(runtimeModeDirect)
	if len(direct) != 2 || direct[0].Name != "baidu" || direct[1].Name != "xiaomi" {
		t.Fatalf("direct-mode probes = %#v", direct)
	}
	proxied := runtimeRoutingVerificationSites(runtimeModeGlobal)
	if len(proxied) != 2 || proxied[0].Name != "google" || proxied[1].Name != "github" {
		t.Fatalf("proxied-mode probes = %#v", proxied)
	}
}

func TestVerifyExternalSiteViaProxyDoesNotResolveTargetLocally(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Host != "unresolvable.invalid" {
			t.Errorf("proxy received host %q", request.URL.Host)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := verifyExternalSiteViaProxy(context.Background(), externalSiteProbe{
		Name: "test", URL: "http://unresolvable.invalid/probe", ExpectedStatus: []int{http.StatusNoContent},
	}, proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DNS || !result.TCP || !result.HTTPS || result.StatusCode != http.StatusNoContent {
		t.Fatalf("verification = %#v", result)
	}
}

func TestVerifyExternalSiteViaProxyRejectsUnexpectedStatus(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifyExternalSiteViaProxy(context.Background(), externalSiteProbe{
		Name: "openai-api", URL: "http://unresolvable.invalid/probe", ExpectedStatus: []int{http.StatusUnauthorized},
	}, proxyURL)
	if err == nil {
		t.Fatal("unexpected proxied HTTP status was accepted")
	}
}
