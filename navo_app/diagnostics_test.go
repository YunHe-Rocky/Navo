package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRunProxyBenchmarkUsesConfiguredLocalProxy(t *testing.T) {
	t.Parallel()

	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/__down":
			size, _ := strconv.Atoi(request.URL.Query().Get("bytes"))
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(make([]byte, size))
		case "/__up":
			count, _ := io.Copy(io.Discard, request.Body)
			if count == 0 {
				http.Error(writer, "missing payload", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer proxy.Close()

	endpoints := benchmarkEndpoints{
		latency:  "http://benchmark.test/__down?bytes=0",
		download: "http://benchmark.test/__down",
		upload:   "http://benchmark.test/__up",
	}
	result, err := runProxyBenchmark(context.Background(), proxy.URL, endpoints, 128*1024, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.DownloadBytes != 128*1024 || result.UploadBytes != 64*1024 {
		t.Fatalf("unexpected transfer sizes: %#v", result)
	}
	if result.DownloadMbps <= 0 || result.UploadMbps <= 0 {
		t.Fatalf("throughput was not measured: %#v", result)
	}
	if result.ProxyEndpoint == "" || result.CheckedAt.IsZero() {
		t.Fatalf("missing benchmark metadata: %#v", result)
	}
}

func TestRunProxyBenchmarkRejectsNonLoopbackProxy(t *testing.T) {
	t.Parallel()

	_, err := runProxyBenchmark(
		context.Background(),
		"http://192.0.2.10:8080",
		benchmarkEndpoints{},
		1024,
		1024,
	)
	if err == nil {
		t.Fatal("expected non-loopback proxy rejection")
	}
}

func TestCheckCoreUpdatesVerifiesIntegrityAndVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	binaryPath := filepath.Join(root, "third_party", "sing-box", "sing-box.exe")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("verified core")
	if err := os.WriteFile(binaryPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	manifest := fmt.Sprintf(`{"cores":[{"type":"sing-box","version":"1.2.0","relative_path":"third_party/sing-box/sing-box.exe","sha256":"%s"}]}`, hex.EncodeToString(digest[:]))
	if err := os.WriteFile(filepath.Join(root, "CORE_MANIFEST.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	releaseAPI := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"tag_name":"v1.3.0","html_url":"https://github.com/SagerNet/sing-box/releases/tag/v1.3.0"}`)
	}))
	defer releaseAPI.Close()

	report := checkCoreUpdates(
		context.Background(),
		root,
		[]CoreOption{{ID: "sing-box", Name: "sing-box", Version: "1.2.0", Installed: true}},
		[]coreReleaseSource{{id: "sing-box", name: "sing-box", apiURL: releaseAPI.URL, releaseURL: "https://github.com/SagerNet/sing-box/releases/latest"}},
		&http.Client{Timeout: time.Second},
	)
	if len(report.Items) != 1 {
		t.Fatalf("items = %#v", report.Items)
	}
	item := report.Items[0]
	if !item.IntegrityOK || !item.UpdateAvailable || item.LatestVersion != "1.3.0" {
		t.Fatalf("unexpected update status: %#v", item)
	}
	if item.ReleaseURL != "https://github.com/SagerNet/sing-box/releases/tag/v1.3.0" {
		t.Fatalf("release URL = %q", item.ReleaseURL)
	}
}

func TestVersionGreater(t *testing.T) {
	t.Parallel()

	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{latest: "v1.13.15", current: "1.13.14", want: true},
		{latest: "1.19.28", current: "1.19.29", want: false},
		{latest: "26.7.1", current: "26.3.27", want: true},
		{latest: "1.13.14", current: "1.13.14", want: false},
	}
	for _, test := range tests {
		if got := versionGreater(test.latest, test.current); got != test.want {
			t.Fatalf("versionGreater(%q, %q) = %v, want %v", test.latest, test.current, got, test.want)
		}
	}
}
