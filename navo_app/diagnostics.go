package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	diagnosticsAppVersion = "1.0.0"
	downloadSampleBytes   = 4 * 1024 * 1024
	uploadSampleBytes     = 1024 * 1024
)

type HostStatus struct {
	OS                 string  `json:"os"`
	Arch               string  `json:"arch"`
	AppVersion         string  `json:"app_version"`
	GoVersion          string  `json:"go_version"`
	LogicalCPU         int     `json:"logical_cpu"`
	MemoryTotalBytes   uint64  `json:"memory_total_bytes"`
	MemoryAvailable    uint64  `json:"memory_available_bytes"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	SystemUptime       int64   `json:"system_uptime_seconds"`
	ProcessUptime      int64   `json:"process_uptime_seconds"`
}

type ProxyBenchmark struct {
	ProxyEndpoint string  `json:"proxy_endpoint"`
	TestServer    string  `json:"test_server"`
	LatencyMS     float64 `json:"latency_ms"`
	JitterMS      float64 `json:"jitter_ms"`
	DownloadMbps  float64 `json:"download_mbps"`
	UploadMbps    float64 `json:"upload_mbps"`
	DownloadBytes int64   `json:"download_bytes"`
	UploadBytes   int64   `json:"upload_bytes"`
	DurationMS    int64   `json:"duration_ms"`
	CheckedAt     string  `json:"checked_at"`
}

type LatencyResult struct {
	OutboundID       string `json:"outbound_id"`
	State            string `json:"state"`
	TCPConnectMS     int64  `json:"tcp_connect_ms"`
	ProxyHandshakeMS int64  `json:"proxy_handshake_ms"`
	DNSMS            int64  `json:"dns_ms"`
	TLSMS            int64  `json:"tls_ms"`
	TTFBMS           int64  `json:"ttfb_ms"`
	TotalMS          int64  `json:"total_ms"`
	ExitIP           string `json:"exit_ip,omitempty"`
	CheckedAt        string `json:"checked_at"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
	DNSObservable    bool   `json:"dns_observable"`
}

type CoreUpdateStatus struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	CurrentVersion       string `json:"current_version"`
	LatestVersion        string `json:"latest_version"`
	UpdateAvailable      bool   `json:"update_available"`
	IntegrityOK          bool   `json:"integrity_ok"`
	ReleaseURL           string `json:"release_url"`
	Error                string `json:"error"`
	State                string `json:"state"`
	InstallSupported     bool   `json:"install_supported"`
	InstallBlockedReason string `json:"install_blocked_reason,omitempty"`
	AssetName            string `json:"asset_name,omitempty"`
}

type CoreUpdateReport struct {
	Items     []CoreUpdateStatus `json:"items"`
	CheckedAt string             `json:"checked_at"`
}

type benchmarkEndpoints struct {
	latency  string
	download string
	upload   string
}

var defaultBenchmarkEndpoints = benchmarkEndpoints{
	latency:  "https://speed.cloudflare.com/__down?bytes=0",
	download: "https://speed.cloudflare.com/__down",
	upload:   "https://speed.cloudflare.com/__up",
}

type coreReleaseSource struct {
	id         string
	name       string
	apiURL     string
	releaseURL string
}

var coreReleaseSources = []coreReleaseSource{
	{id: "sing-box", name: "sing-box", apiURL: "https://api.github.com/repos/SagerNet/sing-box/releases/latest", releaseURL: "https://github.com/SagerNet/sing-box/releases/latest"},
	{id: "mihomo", name: "Mihomo", apiURL: "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest", releaseURL: "https://github.com/MetaCubeX/mihomo/releases/latest"},
	{id: "xray", name: "Xray-core", apiURL: "https://api.github.com/repos/XTLS/Xray-core/releases/latest", releaseURL: "https://github.com/XTLS/Xray-core/releases/latest"},
}

type coreManifest struct {
	SchemaVersion int                 `json:"schema_version"`
	Cores         []coreManifestEntry `json:"cores"`
}

type coreManifestEntry struct {
	Type           string   `json:"type"`
	Version        string   `json:"version"`
	RelativePath   string   `json:"relative_path"`
	SHA256         string   `json:"sha256"`
	ConfigFormat   string   `json:"config_format,omitempty"`
	VersionArgs    []string `json:"version_args,omitempty"`
	ValidationArgs []string `json:"validation_args,omitempty"`
	RunArgs        []string `json:"run_args,omitempty"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

func (a *App) GetHostStatus() HostStatus {
	return collectHostStatus(a.startedAt)
}

func (a *App) RunProxyBenchmark() (ProxyBenchmark, error) {
	a.benchmarkMu.Lock()
	if a.benchmarkRunning {
		a.benchmarkMu.Unlock()
		return ProxyBenchmark{}, errors.New("测速任务已在运行")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	a.benchmarkRunning = true
	a.benchmarkCancel = cancel
	a.benchmarkMu.Unlock()

	defer func() {
		cancel()
		a.benchmarkMu.Lock()
		a.benchmarkRunning = false
		a.benchmarkCancel = nil
		a.benchmarkMu.Unlock()
	}()

	snapshot, err := call[Dashboard](a, "dashboard.snapshot", nil)
	if err != nil {
		return ProxyBenchmark{}, err
	}
	if snapshot.Core.State != "running" {
		return ProxyBenchmark{}, errors.New("代理内核尚未运行，请先连接节点")
	}
	host := strings.TrimSpace(snapshot.Proxy.Server)
	if host == "" {
		host = "127.0.0.1"
	}
	if snapshot.Proxy.Port < 1 || snapshot.Proxy.Port > 65535 {
		return ProxyBenchmark{}, errors.New("本地代理端口不可用")
	}
	proxyAddress := net.JoinHostPort(host, strconv.Itoa(snapshot.Proxy.Port))
	return runProxyBenchmark(ctx, "http://"+proxyAddress, defaultBenchmarkEndpoints, downloadSampleBytes, uploadSampleBytes)
}

// RunTrafficTransfer performs an explicitly user-triggered, bounded transfer
// through the current local proxy. It is separate from synthetic chart preview.
func (a *App) RunTrafficTransfer(sizeMiB int64, direction string) (ProxyBenchmark, error) {
	if sizeMiB < 1 || sizeMiB > 32 {
		return ProxyBenchmark{}, errors.New("真实传输大小必须在 1 到 32 MiB 之间")
	}
	if direction != "download" && direction != "upload" && direction != "both" {
		return ProxyBenchmark{}, errors.New("真实传输方向无效")
	}
	a.benchmarkMu.Lock()
	if a.benchmarkRunning {
		a.benchmarkMu.Unlock()
		return ProxyBenchmark{}, errors.New("已有测速任务正在运行")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	a.benchmarkRunning = true
	a.benchmarkCancel = cancel
	a.benchmarkMu.Unlock()
	defer func() {
		cancel()
		a.benchmarkMu.Lock()
		a.benchmarkRunning = false
		a.benchmarkCancel = nil
		a.benchmarkMu.Unlock()
	}()

	snapshot, err := a.GetDashboard()
	if err != nil {
		return ProxyBenchmark{}, err
	}
	if snapshot.Core.State != "running" {
		return ProxyBenchmark{}, errors.New("当前核心未运行，不能执行真实传输")
	}
	host := strings.TrimSpace(snapshot.Proxy.Server)
	if host == "" {
		host = "127.0.0.1"
	}
	if snapshot.Proxy.Port < 1 || snapshot.Proxy.Port > 65535 {
		return ProxyBenchmark{}, errors.New("本地代理端点无效")
	}
	bytes := sizeMiB * 1024 * 1024
	downloadBytes, uploadBytes := int64(0), int64(0)
	if direction == "download" || direction == "both" {
		downloadBytes = bytes
	}
	if direction == "upload" || direction == "both" {
		uploadBytes = bytes
	}
	return runProxyBenchmark(ctx, "http://"+net.JoinHostPort(host, strconv.Itoa(snapshot.Proxy.Port)),
		defaultBenchmarkEndpoints, downloadBytes, uploadBytes)
}

func (a *App) CancelProxyBenchmark() {
	a.benchmarkMu.Lock()
	cancel := a.benchmarkCancel
	a.benchmarkMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// RunLatencyTest measures the active route without changing route or Capture
// state. Remote DNS is resolved inside the proxy core and is deliberately
// reported as unobservable instead of being replaced with a fabricated value.
func (a *App) RunLatencyTest(outboundID string) (LatencyResult, error) {
	a.benchmarkMu.Lock()
	if a.benchmarkRunning {
		a.benchmarkMu.Unlock()
		return LatencyResult{}, errors.New("已有测速任务正在运行")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	a.benchmarkRunning = true
	a.benchmarkCancel = cancel
	a.benchmarkMu.Unlock()
	defer func() {
		cancel()
		a.benchmarkMu.Lock()
		a.benchmarkRunning = false
		a.benchmarkCancel = nil
		a.benchmarkMu.Unlock()
	}()

	routes, err := a.ListRoutes()
	if err != nil {
		return LatencyResult{}, err
	}
	requested := strings.TrimSpace(outboundID)
	if requested == "" {
		requested = routes.ActiveID
	}
	result := LatencyResult{
		OutboundID: requested,
		State:      "testing",
		CheckedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if requested == "" || requested != routes.ActiveID {
		result.State = "failed"
		result.ErrorCode = "OUTBOUND_NOT_ACTIVE"
		result.ErrorMessage = "分层测速仅允许当前节点，测试不会临时切换线路"
		return result, nil
	}
	tcp, err := a.TestRoute(requested)
	if err != nil {
		return LatencyResult{}, err
	}
	result.TCPConnectMS = tcp.LatencyMS
	if !tcp.Reachable {
		result.State = "failed"
		result.ErrorCode = "TCP_UNREACHABLE"
		result.ErrorMessage = tcp.Error
		return result, nil
	}

	snapshot, err := a.GetDashboard()
	if err != nil {
		return LatencyResult{}, err
	}
	if snapshot.Core.State != "running" {
		result.State = "failed"
		result.ErrorCode = "CORE_NOT_RUNNING"
		result.ErrorMessage = "当前核心未运行，无法验证代理握手与出口"
		return result, nil
	}
	proxyHost := strings.TrimSpace(snapshot.Proxy.Server)
	if proxyHost == "" {
		proxyHost = "127.0.0.1"
	}
	proxyURL, err := url.Parse("http://" + net.JoinHostPort(proxyHost, strconv.Itoa(snapshot.Proxy.Port)))
	if err != nil || snapshot.Proxy.Port < 1 || snapshot.Proxy.Port > 65535 {
		return LatencyResult{}, errors.New("本地代理端点无效")
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true, ForceAttemptHTTP2: true,
		DialContext:         (&net.Dialer{Timeout: 4 * time.Second}).DialContext,
		TLSHandshakeTimeout: 6 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
	}
	defer transport.CloseIdleConnections()
	var gotConnAt, tlsStart, tlsDone, firstByte time.Time
	trace := &httptrace.ClientTrace{
		GotConn:              func(httptrace.GotConnInfo) { gotConnAt = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet,
		addQuery(defaultBenchmarkEndpoints.latency, "r", strconv.FormatInt(time.Now().UnixNano(), 10)), nil)
	if err != nil {
		return LatencyResult{}, err
	}
	req.Header.Set("Cache-Control", "no-store")
	response, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		result.State = "failed"
		result.ErrorCode = latencyErrorCode(ctx, err)
		result.ErrorMessage = err.Error()
		return result, nil
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	closeErr := response.Body.Close()
	result.TotalMS = time.Since(started).Milliseconds()
	if !gotConnAt.IsZero() && !tlsStart.IsZero() {
		result.ProxyHandshakeMS = tlsStart.Sub(gotConnAt).Milliseconds()
	}
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		result.TLSMS = tlsDone.Sub(tlsStart).Milliseconds()
	}
	if !firstByte.IsZero() {
		result.TTFBMS = firstByte.Sub(started).Milliseconds()
	}
	if readErr != nil || closeErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		result.State = "failed"
		result.ErrorCode = "HTTP_RESPONSE_FAILED"
		result.ErrorMessage = fmt.Sprintf("受控请求返回 HTTP %d", response.StatusCode)
		return result, nil
	}
	ipResult, ipErr := a.CheckIP()
	if ipErr != nil || ipResult.Proxy.Error != "" || strings.TrimSpace(ipResult.Proxy.IP) == "" {
		result.State = "partial"
		result.ErrorCode = "EXIT_IP_UNAVAILABLE"
		result.ErrorMessage = "协议链路可用，但出口 IP 暂时无法确认"
		return result, nil
	}
	result.ExitIP = ipResult.Proxy.IP
	result.State = "completed"
	return result, nil
}

func latencyErrorCode(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "LATENCY_TIMEOUT"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "LATENCY_TIMEOUT"
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return "TLS_FAILED"
	}
	return "PROXY_HANDSHAKE_FAILED"
}

func (a *App) CheckCoreUpdates() (CoreUpdateReport, error) {
	snapshot, err := call[Dashboard](a, "dashboard.snapshot", nil)
	if err != nil {
		return CoreUpdateReport{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	report := checkCoreUpdates(ctx, runtimeRoot(), snapshot.Cores, coreReleaseSources, &http.Client{Timeout: 8 * time.Second})
	a.coreUpdateMu.Lock()
	a.coreUpdateCache = report
	a.coreUpdateMu.Unlock()
	return report, nil
}

func (a *App) GetCoreUpdateStatus() CoreUpdateReport {
	a.coreUpdateMu.RLock()
	defer a.coreUpdateMu.RUnlock()
	return a.coreUpdateCache
}

func (a *App) OpenCoreRelease(coreID string) error {
	for _, source := range coreReleaseSources {
		if source.id != strings.TrimSpace(coreID) {
			continue
		}
		if a.context == nil {
			return errors.New("桌面运行时尚未就绪")
		}
		wailsruntime.BrowserOpenURL(a.context, source.releaseURL)
		return nil
	}
	return fmt.Errorf("不支持的内核：%s", coreID)
}

func runProxyBenchmark(
	ctx context.Context,
	proxyAddress string,
	endpoints benchmarkEndpoints,
	downloadBytes int64,
	uploadBytes int64,
) (ProxyBenchmark, error) {
	proxyURL, err := url.Parse(proxyAddress)
	if err != nil || proxyURL.Scheme != "http" || proxyURL.Hostname() == "" {
		return ProxyBenchmark{}, errors.New("本地代理地址无效")
	}
	if host := proxyURL.Hostname(); host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return ProxyBenchmark{}, errors.New("测速仅允许使用本机代理端点")
		}
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		DialContext:           (&net.Dialer{Timeout: 4 * time.Second, KeepAlive: 20 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       15 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	started := time.Now()

	latencies := make([]float64, 0, 3)
	for index := 0; index < 3; index++ {
		requestURL := addQuery(endpoints.latency, "r", strconv.FormatInt(time.Now().UnixNano(), 10))
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if reqErr != nil {
			return ProxyBenchmark{}, fmt.Errorf("创建延迟请求失败：%w", reqErr)
		}
		req.Header.Set("Cache-Control", "no-store")
		sampleStarted := time.Now()
		resp, requestErr := client.Do(req)
		if requestErr != nil {
			return ProxyBenchmark{}, benchmarkError(ctx, "代理延迟检测失败", requestErr)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ProxyBenchmark{}, fmt.Errorf("代理延迟检测返回 HTTP %d", resp.StatusCode)
		}
		latencies = append(latencies, float64(time.Since(sampleStarted).Microseconds())/1000)
	}

	var downloaded, uploaded int64
	var downloadDuration, uploadDuration time.Duration
	if downloadBytes > 0 {
		downloadURL := addQuery(endpoints.download, "bytes", strconv.FormatInt(downloadBytes, 10))
		downloaded, downloadDuration, err = transferDownload(ctx, client, downloadURL, downloadBytes)
		if err != nil {
			return ProxyBenchmark{}, err
		}
	}
	if uploadBytes > 0 {
		uploaded, uploadDuration, err = transferUpload(ctx, client, endpoints.upload, uploadBytes)
		if err != nil {
			return ProxyBenchmark{}, err
		}
	}
	downloadMbps := 0.0
	uploadMbps := 0.0
	if downloadDuration > 0 {
		downloadMbps = round(float64(downloaded)*8/downloadDuration.Seconds()/1_000_000, 2)
	}
	if uploadDuration > 0 {
		uploadMbps = round(float64(uploaded)*8/uploadDuration.Seconds()/1_000_000, 2)
	}

	return ProxyBenchmark{
		ProxyEndpoint: proxyURL.Host,
		TestServer:    hostOf(endpoints.download),
		LatencyMS:     round(mean(latencies), 1),
		JitterMS:      round(standardDeviation(latencies), 1),
		DownloadMbps:  downloadMbps,
		UploadMbps:    uploadMbps,
		DownloadBytes: downloaded,
		UploadBytes:   uploaded,
		DurationMS:    time.Since(started).Milliseconds(),
		CheckedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func transferDownload(ctx context.Context, client *http.Client, target string, expected int64) (int64, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("创建下载测速请求失败：%w", err)
	}
	req.Header.Set("Cache-Control", "no-store")
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, benchmarkError(ctx, "下载测速失败", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("下载测速返回 HTTP %d", resp.StatusCode)
	}
	count, err := io.Copy(io.Discard, io.LimitReader(resp.Body, expected+1))
	duration := time.Since(started)
	if duration <= 0 {
		// Very small loopback fixtures can complete inside one Windows clock tick.
		duration = time.Nanosecond
	}
	if err != nil {
		return 0, 0, benchmarkError(ctx, "读取测速数据失败", err)
	}
	if count < expected/2 {
		return 0, 0, fmt.Errorf("下载测速数据不足：收到 %d 字节", count)
	}
	return count, duration, nil
}

func transferUpload(ctx context.Context, client *http.Client, target string, size int64) (int64, time.Duration, error) {
	payload := bytes.Repeat([]byte{0x4e, 0x41, 0x56, 0x4f}, int((size+3)/4))
	payload = payload[:size]
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return 0, 0, fmt.Errorf("创建上传测速请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = size
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, benchmarkError(ctx, "上传测速失败", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("上传测速返回 HTTP %d", resp.StatusCode)
	}
	duration := time.Since(started)
	if duration <= 0 {
		duration = time.Nanosecond
	}
	return size, duration, nil
}

func checkCoreUpdates(
	ctx context.Context,
	root string,
	cores []CoreOption,
	sources []coreReleaseSource,
	client *http.Client,
) CoreUpdateReport {
	manifest, manifestErr := loadCoreManifest(filepath.Join(root, "CORE_MANIFEST.json"))
	current := make(map[string]CoreOption, len(cores))
	for _, core := range cores {
		current[core.ID] = core
	}

	items := make([]CoreUpdateStatus, len(sources))
	var wait sync.WaitGroup
	for index, source := range sources {
		index, source := index, source
		wait.Add(1)
		go func() {
			defer wait.Done()
			status := CoreUpdateStatus{
				ID: source.id, Name: source.name, ReleaseURL: source.releaseURL, State: "checking",
				InstallSupported:     false,
				InstallBlockedReason: "当前发布未包含受信更新资产 SHA-256；仅允许查看官方版本，不执行不受信安装",
			}
			entry, ok := manifest[source.id]
			if ok {
				status.CurrentVersion = entry.Version
				status.IntegrityOK = verifyFileSHA256(filepath.Join(root, filepath.FromSlash(entry.RelativePath)), entry.SHA256)
			}
			if option, exists := current[source.id]; exists && strings.TrimSpace(option.Version) != "" {
				status.CurrentVersion = strings.TrimSpace(option.Version)
			}
			if manifestErr != nil {
				status.Error = manifestErr.Error()
			} else if !ok {
				status.Error = "内核清单缺少该项"
			} else if !status.IntegrityOK {
				status.Error = "本地文件完整性校验失败"
			}

			latest, releaseURL, err := fetchLatestRelease(ctx, client, source)
			if err != nil {
				if status.Error != "" {
					status.Error += "；"
				}
				status.Error += err.Error()
			} else {
				status.LatestVersion = latest
				if releaseURL != "" {
					status.ReleaseURL = releaseURL
				}
				status.UpdateAvailable = status.IntegrityOK && versionGreater(latest, status.CurrentVersion)
				if status.UpdateAvailable {
					installCandidate, installErr := fetchInstallCandidate(ctx, client, source)
					if installErr != nil {
						status.InstallSupported = false
						status.InstallBlockedReason = installErr.Error()
					} else {
						status.InstallSupported = true
						status.InstallBlockedReason = ""
						status.AssetName = installCandidate.asset.Name
					}
				}
			}
			switch {
			case status.Error != "":
				status.State = "failed"
			case status.UpdateAvailable:
				status.State = "update_available"
			default:
				status.State = "up_to_date"
			}
			items[index] = status
		}()
	}
	wait.Wait()
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return CoreUpdateReport{
		Items: items, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

type manifestCore struct {
	Version      string
	RelativePath string
	SHA256       string
}

func loadCoreManifest(path string) (map[string]manifestCore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取内核清单失败：%w", err)
	}
	var manifest coreManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析内核清单失败：%w", err)
	}
	result := make(map[string]manifestCore, len(manifest.Cores))
	for _, item := range manifest.Cores {
		result[item.Type] = manifestCore{Version: item.Version, RelativePath: item.RelativePath, SHA256: strings.ToLower(item.SHA256)}
	}
	return result, nil
}

func fetchLatestRelease(ctx context.Context, client *http.Client, source coreReleaseSource) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建更新请求失败：%w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Navo/"+diagnosticsAppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("检查官方版本失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("检查官方版本返回 HTTP %d", resp.StatusCode)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&release); err != nil {
		return "", "", fmt.Errorf("解析官方版本失败：%w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(release.TagName, "v"))
	if version == "" {
		return "", "", errors.New("官方版本号为空")
	}
	releaseURL := source.releaseURL
	if parsed, parseErr := url.Parse(release.HTMLURL); parseErr == nil && parsed.Scheme == "https" && parsed.Host == "github.com" {
		releaseURL = parsed.String()
	}
	return version, releaseURL, nil
}

func runtimeRoot() string {
	candidates := make([]string, 0, 5)
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates, executableDir, filepath.Dir(executableDir))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd, filepath.Dir(cwd))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "CORE_MANIFEST.json")); err == nil {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "."
}

func verifyFileSHA256(path string, expected string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	return strings.EqualFold(actual, strings.TrimSpace(expected))
}

func versionGreater(latest string, current string) bool {
	left, leftOK := parseSemanticVersion(latest)
	right, rightOK := parseSemanticVersion(current)
	if !leftOK || !rightOK {
		return false
	}
	length := len(left)
	if len(right) > length {
		length = len(right)
	}
	for index := 0; index < length; index++ {
		var l, r int
		if index < len(left) {
			l = left[index]
		}
		if index < len(right) {
			r = right[index]
		}
		if l != r {
			return l > r
		}
	}
	leftPre := semanticPrerelease(latest)
	rightPre := semanticPrerelease(current)
	if leftPre == rightPre {
		return false
	}
	if leftPre == "" {
		return true
	}
	if rightPre == "" {
		return false
	}
	return comparePrerelease(leftPre, rightPre) > 0
}

var semanticVersionPattern = regexp.MustCompile(`^[vV]?[0-9]+(?:\.[0-9]+){1,3}(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func parseSemanticVersion(value string) ([]int, bool) {
	value = strings.TrimSpace(value)
	if !semanticVersionPattern.MatchString(value) {
		return nil, false
	}
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	core, _, _ := strings.Cut(value, "+")
	core, _, _ = strings.Cut(core, "-")
	parts := strings.Split(core, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		result = append(result, number)
	}
	return result, true
}

func semanticPrerelease(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V"))
	value, _, _ = strings.Cut(value, "+")
	_, prerelease, found := strings.Cut(value, "-")
	if !found {
		return ""
	}
	return prerelease
}

func comparePrerelease(left, right string) int {
	lParts, rParts := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < max(len(lParts), len(rParts)); index++ {
		if index >= len(lParts) {
			return -1
		}
		if index >= len(rParts) {
			return 1
		}
		lNumber, lErr := strconv.Atoi(lParts[index])
		rNumber, rErr := strconv.Atoi(rParts[index])
		switch {
		case lErr == nil && rErr == nil && lNumber != rNumber:
			if lNumber > rNumber {
				return 1
			}
			return -1
		case lErr == nil && rErr != nil:
			return -1
		case lErr != nil && rErr == nil:
			return 1
		case lParts[index] != rParts[index]:
			if lParts[index] > rParts[index] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func addQuery(rawURL string, key string, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Hostname()
}

func benchmarkError(ctx context.Context, message string, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return errors.New("测速已取消")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("测速超时，请检查当前代理链路")
	}
	return fmt.Errorf("%s：%w", message, err)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func standardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	average := mean(values)
	var total float64
	for _, value := range values {
		delta := value - average
		total += delta * delta
	}
	return math.Sqrt(total / float64(len(values)))
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
