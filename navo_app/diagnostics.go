package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	ProxyEndpoint string    `json:"proxy_endpoint"`
	TestServer    string    `json:"test_server"`
	LatencyMS     float64   `json:"latency_ms"`
	JitterMS      float64   `json:"jitter_ms"`
	DownloadMbps  float64   `json:"download_mbps"`
	UploadMbps    float64   `json:"upload_mbps"`
	DownloadBytes int64     `json:"download_bytes"`
	UploadBytes   int64     `json:"upload_bytes"`
	DurationMS    int64     `json:"duration_ms"`
	CheckedAt     time.Time `json:"checked_at"`
}

type CoreUpdateStatus struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	IntegrityOK     bool   `json:"integrity_ok"`
	ReleaseURL      string `json:"release_url"`
	Error           string `json:"error"`
}

type CoreUpdateReport struct {
	Items     []CoreUpdateStatus `json:"items"`
	CheckedAt time.Time          `json:"checked_at"`
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
	Cores []struct {
		Type         string `json:"type"`
		Version      string `json:"version"`
		RelativePath string `json:"relative_path"`
		SHA256       string `json:"sha256"`
	} `json:"cores"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
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

func (a *App) CancelProxyBenchmark() {
	a.benchmarkMu.Lock()
	cancel := a.benchmarkCancel
	a.benchmarkMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) CheckCoreUpdates() (CoreUpdateReport, error) {
	snapshot, err := call[Dashboard](a, "dashboard.snapshot", nil)
	if err != nil {
		return CoreUpdateReport{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return checkCoreUpdates(ctx, runtimeRoot(), snapshot.Cores, coreReleaseSources, &http.Client{Timeout: 8 * time.Second}), nil
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

	downloadURL := addQuery(endpoints.download, "bytes", strconv.FormatInt(downloadBytes, 10))
	downloaded, downloadDuration, err := transferDownload(ctx, client, downloadURL, downloadBytes)
	if err != nil {
		return ProxyBenchmark{}, err
	}
	uploaded, uploadDuration, err := transferUpload(ctx, client, endpoints.upload, uploadBytes)
	if err != nil {
		return ProxyBenchmark{}, err
	}

	return ProxyBenchmark{
		ProxyEndpoint: proxyURL.Host,
		TestServer:    hostOf(endpoints.download),
		LatencyMS:     round(mean(latencies), 1),
		JitterMS:      round(standardDeviation(latencies), 1),
		DownloadMbps:  round(float64(downloaded)*8/downloadDuration.Seconds()/1_000_000, 2),
		UploadMbps:    round(float64(uploaded)*8/uploadDuration.Seconds()/1_000_000, 2),
		DownloadBytes: downloaded,
		UploadBytes:   uploaded,
		DurationMS:    time.Since(started).Milliseconds(),
		CheckedAt:     time.Now().UTC(),
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
	return size, time.Since(started), nil
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
			status := CoreUpdateStatus{ID: source.id, Name: source.name, ReleaseURL: source.releaseURL}
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
			}
			items[index] = status
		}()
	}
	wait.Wait()
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return CoreUpdateReport{Items: items, CheckedAt: time.Now().UTC()}
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
	left := numericVersion(latest)
	right := numericVersion(current)
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
	return false
}

func numericVersion(value string) []int {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	parts := strings.FieldsFunc(value, func(r rune) bool { return r < '0' || r > '9' })
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err == nil {
			result = append(result, number)
		}
	}
	return result
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
