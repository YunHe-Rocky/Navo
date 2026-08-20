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
	proxiedSites := []string{
		"google", "github", "chatgpt-web", "chatgpt-auth", "openai-api",
		"chatgpt-assets", "chatgpt-stream",
	}
	for _, test := range []struct {
		name string
		mode string
		want []string
	}{
		{name: "direct", mode: runtimeModeDirect, want: []string{"baidu", "xiaomi"}},
		{name: "global", mode: runtimeModeGlobal, want: proxiedSites},
		{name: "bypass mainland", mode: runtimeModeBypassMainland, want: proxiedSites},
	} {
		t.Run(test.name, func(t *testing.T) {
			probes := runtimeRoutingVerificationSites(test.mode)
			if len(probes) != len(test.want) {
				t.Fatalf("routing probe count = %d, want %d: %#v", len(probes), len(test.want), probes)
			}
			for index, want := range test.want {
				if probes[index].Name != want {
					t.Fatalf("routing probe %d = %q, want %q", index, probes[index].Name, want)
				}
			}
		})
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
