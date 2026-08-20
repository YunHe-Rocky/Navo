package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"navo/internal/agent/systemproxy"
)

func TestCoreUpdateHTTPRoutesPreferLocalProxiesAndDeduplicate(t *testing.T) {
	t.Parallel()
	dashboard := Dashboard{
		Core:  CoreStatus{State: "running"},
		Proxy: ProxyStatus{Server: "127.0.0.1", Port: 12080},
	}
	routes := coreUpdateHTTPRoutes(dashboard, systemproxy.ProxyConfig{
		Enabled: true, ProxyServer: "https=127.0.0.1:10808;http=127.0.0.1:10809",
	}, func(string) (string, bool) { return "", false })
	if len(routes) != 3 {
		t.Fatalf("routes = %#v", routes)
	}
	if routes[0].name != "Navo local proxy" || routes[0].proxyURL.String() != "http://127.0.0.1:12080" {
		t.Fatalf("first route = %#v", routes[0])
	}
	if routes[1].name != "Windows system proxy" || routes[1].proxyURL.String() != "http://127.0.0.1:10808" {
		t.Fatalf("second route = %#v", routes[1])
	}
	if routes[2].name != "direct" || routes[2].proxyURL != nil || routes[2].useEnvironment {
		t.Fatalf("final route = %#v", routes[2])
	}

	deduplicated := coreUpdateHTTPRoutes(dashboard, systemproxy.ProxyConfig{
		Enabled: true, ProxyServer: "127.0.0.1:12080",
	}, func(string) (string, bool) { return "", false })
	if len(deduplicated) != 2 || deduplicated[1].name != "direct" {
		t.Fatalf("deduplicated routes = %#v", deduplicated)
	}
}

func TestLoopbackHTTPProxyURLRejectsUnsafeEndpoints(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", "proxy.example:8080", "http://127.0.0.1", "socks5://127.0.0.1:1080",
		"http://user:pass@127.0.0.1:8080", "http://127.0.0.1:8080/path", "127.0.0.1:70000",
	} {
		if got := loopbackHTTPProxyURL(value); got != nil {
			t.Errorf("loopbackHTTPProxyURL(%q) = %s", value, got)
		}
	}
	for _, value := range []string{"127.0.0.1:10808", "http://localhost:12080", "http://[::1]:8080"} {
		if got := loopbackHTTPProxyURL(value); got == nil {
			t.Errorf("loopbackHTTPProxyURL(%q) rejected", value)
		}
	}
}

func TestWithCoreUpdateHTTPRoutesFallsBackWithFreshTransports(t *testing.T) {
	t.Parallel()
	routes := []coreUpdateHTTPRoute{{name: "first"}, {name: "second"}}
	transports := make([]*http.Transport, 0, len(routes))
	attempt := 0
	result, err := withCoreUpdateHTTPRoutes(context.Background(), routes, time.Second, func(_ context.Context, client *http.Client) (string, error) {
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("transport = %T", client.Transport)
		}
		transports = append(transports, transport)
		attempt++
		if attempt == 1 {
			return "", errors.New("connection timeout")
		}
		return "downloaded", nil
	})
	if err != nil || result != "downloaded" || attempt != 2 {
		t.Fatalf("result = %q, attempts = %d, err = %v", result, attempt, err)
	}
	if transports[0] == transports[1] {
		t.Fatal("fallback reused the previous transport")
	}
}

func TestWithCoreUpdateHTTPRoutesGivesEachRouteIndependentTimeout(t *testing.T) {
	t.Parallel()
	routes := []coreUpdateHTTPRoute{{name: "slow"}, {name: "fallback"}}
	attempt := 0
	result, err := withCoreUpdateHTTPRoutes(context.Background(), routes, 25*time.Millisecond, func(attemptCtx context.Context, _ *http.Client) (string, error) {
		attempt++
		if attempt == 1 {
			<-attemptCtx.Done()
			return "", attemptCtx.Err()
		}
		if err := attemptCtx.Err(); err != nil {
			t.Fatalf("fallback inherited exhausted context: %v", err)
		}
		return "fallback-ok", nil
	})
	if err != nil || result != "fallback-ok" || attempt != 2 {
		t.Fatalf("result = %q, attempts = %d, err = %v", result, attempt, err)
	}
}

func TestTrustedCoreUpdateClientDoesNotLimitWholeResponseBody(t *testing.T) {
	t.Parallel()
	client := trustedCoreUpdateClient(coreUpdateHTTPRoute{name: "direct"})
	if client.Timeout != 0 {
		t.Fatalf("client timeout = %s, want 0 so body progress controls the download", client.Timeout)
	}
}

func TestDownloadCoreArchiveAllowsContinuousSlowProgress(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte("progress"), 4)
	asset := testCoreUpdateAsset(data)
	body := newPacedCoreUpdateBody(data, 10*time.Millisecond, -1)
	client := testCoreUpdateClient(body)
	started := time.Now()
	got, err := downloadCoreArchiveWithIdleTimeout(context.Background(), client, asset, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("archive = %q", got)
	}
	if elapsed := time.Since(started); elapsed <= 100*time.Millisecond {
		t.Fatalf("download completed in %s; test did not exceed the idle window", elapsed)
	}
}

func TestDownloadCoreArchiveRejectsStalledBody(t *testing.T) {
	t.Parallel()
	data := []byte("stalled-body")
	asset := testCoreUpdateAsset(data)
	body := newPacedCoreUpdateBody(data, 0, 1)
	client := testCoreUpdateClient(body)
	started := time.Now()
	_, err := downloadCoreArchiveWithIdleTimeout(context.Background(), client, asset, 30*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "未收到数据") {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stalled download took %s", elapsed)
	}
}

func TestDownloadCoreArchiveHonorsCallerCancellation(t *testing.T) {
	t.Parallel()
	data := []byte("cancel-body")
	asset := testCoreUpdateAsset(data)
	body := newPacedCoreUpdateBody(data, 0, 1)
	client := testCoreUpdateClient(body)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(25*time.Millisecond, cancel)
	_, err := downloadCoreArchiveWithIdleTimeout(ctx, client, asset, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context cancellation", err)
	}
}

func TestTrustedGitHubHostIncludesReleaseAssetRedirect(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com"} {
		if !trustedGitHubHost(host) {
			t.Errorf("trustedGitHubHost(%q) = false", host)
		}
	}
	if trustedGitHubHost("github.com.example.org") {
		t.Fatal("lookalike GitHub host accepted")
	}
}

func TestCoreUpdateLiveProxyDownload(t *testing.T) {
	proxyAddress := strings.TrimSpace(os.Getenv("NAVO_CORE_UPDATE_LIVE_PROXY"))
	if proxyAddress == "" {
		t.Skip("set NAVO_CORE_UPDATE_LIVE_PROXY to run the official read-only download test")
	}
	proxyURL := loopbackHTTPProxyURL(proxyAddress)
	if proxyURL == nil {
		t.Fatalf("invalid loopback proxy %q", proxyAddress)
	}
	source, ok := findCoreReleaseSource("sing-box")
	if !ok {
		t.Fatal("sing-box release source is missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), coreUpdateDownloadAttemptTimeout+coreUpdateMetadataAttemptTimeout+time.Minute)
	defer cancel()
	routes := []coreUpdateHTTPRoute{{name: "live proxy", proxyURL: proxyURL}}
	candidate, err := withCoreUpdateHTTPRoutes(ctx, routes, coreUpdateMetadataAttemptTimeout, func(attemptCtx context.Context, client *http.Client) (releaseCandidate, error) {
		return fetchInstallCandidate(attemptCtx, client, source)
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := withCoreUpdateHTTPRoutes(ctx, routes, coreUpdateDownloadAttemptTimeout, func(attemptCtx context.Context, client *http.Client) ([]byte, error) {
		return downloadCoreArchive(attemptCtx, client, candidate.asset)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extractCoreExecutable("sing-box", archive); err != nil {
		t.Fatal(err)
	}
	t.Logf("downloaded %s (%d bytes) through %s", candidate.asset.Name, len(archive), proxyURL)
}

func TestSelectReleaseAssetRequiresExactTrustedArtifact(t *testing.T) {
	t.Parallel()
	assets := []githubAsset{
		{Name: "sing-box-1.13.18-windows-arm64.zip", BrowserDownloadURL: "https://github.com/SagerNet/sing-box/releases/download/v1.13.18/arm.zip", Digest: "sha256:" + strings.Repeat("a", 64), Size: 100},
		{Name: "sing-box-1.13.18-windows-amd64.zip", BrowserDownloadURL: "https://example.com/core.zip", Digest: "sha256:" + strings.Repeat("b", 64), Size: 100},
		{Name: "sing-box-1.13.18-windows-amd64.zip", BrowserDownloadURL: "https://github.com/SagerNet/sing-box/releases/download/v1.13.18/core.zip", Digest: "sha256:" + strings.Repeat("c", 64), Size: 100},
	}
	got := selectReleaseAsset("sing-box", "1.13.18", assets)
	if got == nil || got.Digest != assets[2].Digest {
		t.Fatalf("selected asset = %#v", got)
	}
}

func TestExtractCoreExecutableSelectsOneExpectedBinary(t *testing.T) {
	t.Parallel()
	archive := makeZip(t, map[string][]byte{
		"sing-box-1.13.18/LICENSE":      []byte("license"),
		"sing-box-1.13.18/sing-box.exe": []byte("binary"),
	})
	got, err := extractCoreExecutable("sing-box", archive)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary" {
		t.Fatalf("binary = %q", got)
	}
}

func TestExtractCoreExecutableRejectsAmbiguousArchive(t *testing.T) {
	t.Parallel()
	archive := makeZip(t, map[string][]byte{
		"a/xray.exe": []byte("first"),
		"b/xray.exe": []byte("second"),
	})
	if _, err := extractCoreExecutable("xray", archive); err == nil {
		t.Fatal("expected ambiguous archive rejection")
	}
}

func TestRewriteReleaseChecksumsRequiresBothTransactionEntries(t *testing.T) {
	t.Parallel()
	binary := sha256.Sum256([]byte("new binary"))
	manifest := sha256.Sum256([]byte("new manifest"))
	input := []byte(strings.Repeat("1", 64) + "  CORE_MANIFEST.json\n" + strings.Repeat("2", 64) + "  third_party/sing-box/sing-box.exe\n")
	got, err := rewriteReleaseChecksums(input, map[string]string{
		"CORE_MANIFEST.json":                hex.EncodeToString(manifest[:]),
		"third_party/sing-box/sing-box.exe": hex.EncodeToString(binary[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(hex.EncodeToString(binary[:]))) || !bytes.Contains(got, []byte(hex.EncodeToString(manifest[:]))) {
		t.Fatalf("rewritten checksums = %q", got)
	}
	if _, err := rewriteReleaseChecksums([]byte(strings.Repeat("1", 64)+"  CORE_MANIFEST.json\n"), map[string]string{"missing.exe": strings.Repeat("a", 64)}); err == nil {
		t.Fatal("expected missing checksum entry rejection")
	}
}

func TestCommitCoreUpdateAtomicallyReplacesPayloadFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binaryPath := filepath.Join(root, "core.exe")
	manifestPath := filepath.Join(root, "CORE_MANIFEST.json")
	sumsPath := filepath.Join(root, "SHA256SUMS.txt")
	for path, data := range map[string][]byte{
		binaryPath: []byte("old binary"), manifestPath: []byte("old manifest"), sumsPath: []byte("old sums"),
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	staged, cleanup, err := stageCoreExecutable(binaryPath, []byte("new binary"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := commitCoreUpdate(staged, binaryPath, manifestPath, []byte("new manifest"), sumsPath, []byte("new sums")); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		binaryPath: "new binary", manifestPath: "new manifest", sumsPath: "new sums",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", path, got, err)
		}
	}
}

type coreUpdateTestRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip coreUpdateTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type pacedCoreUpdateBody struct {
	data       []byte
	delay      time.Duration
	stallAfter int
	index      int
	closed     chan struct{}
	closeOnce  sync.Once
}

func newPacedCoreUpdateBody(data []byte, delay time.Duration, stallAfter int) *pacedCoreUpdateBody {
	return &pacedCoreUpdateBody{
		data: append([]byte(nil), data...), delay: delay, stallAfter: stallAfter, closed: make(chan struct{}),
	}
}

func (body *pacedCoreUpdateBody) Read(buffer []byte) (int, error) {
	if body.stallAfter >= 0 && body.index >= body.stallAfter {
		<-body.closed
		return 0, errors.New("body closed")
	}
	if body.index >= len(body.data) {
		return 0, io.EOF
	}
	if body.delay > 0 {
		timer := time.NewTimer(body.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-body.closed:
			return 0, errors.New("body closed")
		}
	}
	buffer[0] = body.data[body.index]
	body.index++
	return 1, nil
}

func (body *pacedCoreUpdateBody) Close() error {
	body.closeOnce.Do(func() { close(body.closed) })
	return nil
}

func testCoreUpdateClient(body io.ReadCloser) *http.Client {
	return &http.Client{Transport: coreUpdateTestRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
		}, nil
	})}
}

func testCoreUpdateAsset(data []byte) githubAsset {
	digest := sha256.Sum256(data)
	return githubAsset{
		Name:               "core.zip",
		BrowserDownloadURL: "https://github.com/example/core/releases/download/v1/core.zip",
		Digest:             "sha256:" + hex.EncodeToString(digest[:]),
		Size:               int64(len(data)),
	}
}

func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(entry, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
