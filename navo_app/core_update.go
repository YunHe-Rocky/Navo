package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/fsatomic"
	"navo/internal/winprocess"
)

const maxCoreArchiveBytes int64 = 64 * 1024 * 1024

type releaseCandidate struct {
	version    string
	releaseURL string
	asset      githubAsset
}

type coreUpdateBackup struct {
	path string
	data []byte
	mode os.FileMode
}

type coreUpdateHTTPRoute struct {
	name           string
	proxyURL       *url.URL
	useEnvironment bool
}

var checksumLinePattern = regexp.MustCompile(`^([0-9a-fA-F]{64})\s{2}(.+)$`)

// InstallCoreUpdate performs a user-triggered update from a fixed official
// GitHub release source. Download, digest, archive and version validation all
// complete before the running core or capture mode is touched.
func (a *App) InstallCoreUpdate(coreID string) (CoreUpdateStatus, error) {
	a.coreInstallMu.Lock()
	defer a.coreInstallMu.Unlock()

	coreID = strings.TrimSpace(coreID)
	source, ok := findCoreReleaseSource(coreID)
	if !ok {
		return CoreUpdateStatus{}, fmt.Errorf("不支持的内核：%s", coreID)
	}
	root := runtimeRoot()
	manifestPath := filepath.Join(root, "CORE_MANIFEST.json")
	sumsPath := filepath.Join(root, "SHA256SUMS.txt")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return CoreUpdateStatus{}, fmt.Errorf("读取内核清单失败：%w", err)
	}
	sumsData, err := os.ReadFile(sumsPath)
	if err != nil {
		return CoreUpdateStatus{}, fmt.Errorf("运行时升级仅支持包含 SHA256SUMS.txt 的正式安装包：%w", err)
	}
	var manifest coreManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return CoreUpdateStatus{}, fmt.Errorf("解析内核清单失败：%w", err)
	}
	entryIndex := -1
	for index := range manifest.Cores {
		if manifest.Cores[index].Type == coreID {
			entryIndex = index
			break
		}
	}
	if entryIndex < 0 {
		return CoreUpdateStatus{}, fmt.Errorf("内核清单缺少 %s", coreID)
	}
	entry := manifest.Cores[entryIndex]
	binaryPath := filepath.Join(root, filepath.FromSlash(entry.RelativePath))
	if !verifyFileSHA256(binaryPath, entry.SHA256) {
		return CoreUpdateStatus{}, errors.New("当前内核完整性校验失败，拒绝覆盖")
	}

	dashboard, err := a.GetDashboard()
	if err != nil {
		return CoreUpdateStatus{}, err
	}
	currentSystemProxy, _ := systemproxy.CurrentConfig()
	routes := coreUpdateHTTPRoutes(dashboard, currentSystemProxy, os.LookupEnv)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	candidate, err := withCoreUpdateHTTPRoutes(ctx, routes, func(client *http.Client) (releaseCandidate, error) {
		return fetchInstallCandidate(ctx, client, source)
	})
	if err != nil {
		return CoreUpdateStatus{}, err
	}
	if !versionGreater(candidate.version, entry.Version) {
		return CoreUpdateStatus{}, fmt.Errorf("%s 已是最新版本 %s", source.name, entry.Version)
	}
	archiveData, err := withCoreUpdateHTTPRoutes(ctx, routes, func(client *http.Client) ([]byte, error) {
		return downloadCoreArchive(ctx, client, candidate.asset)
	})
	if err != nil {
		return CoreUpdateStatus{}, err
	}
	binaryData, err := extractCoreExecutable(coreID, archiveData)
	if err != nil {
		return CoreUpdateStatus{}, err
	}
	stagedPath, cleanup, err := stageCoreExecutable(binaryPath, binaryData)
	if err != nil {
		return CoreUpdateStatus{}, err
	}
	defer cleanup()
	if err := validateCoreVersion(ctx, stagedPath, entry.VersionArgs, candidate.version); err != nil {
		return CoreUpdateStatus{}, err
	}

	newDigest := sha256.Sum256(binaryData)
	manifest.Cores[entryIndex].Version = candidate.version
	manifest.Cores[entryIndex].SHA256 = hex.EncodeToString(newDigest[:])
	updatedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return CoreUpdateStatus{}, fmt.Errorf("生成内核清单失败：%w", err)
	}
	updatedManifest = append(updatedManifest, '\n')
	manifestDigest := sha256.Sum256(updatedManifest)
	updatedSums, err := rewriteReleaseChecksums(sumsData, map[string]string{
		filepath.ToSlash(entry.RelativePath): hex.EncodeToString(newDigest[:]),
		"CORE_MANIFEST.json":                 hex.EncodeToString(manifestDigest[:]),
	})
	if err != nil {
		return CoreUpdateStatus{}, err
	}

	previousMode := dashboard.Capture.CommittedMode
	activeRunning := dashboard.Core.CoreID == coreID && dashboard.Core.State == "running"
	if previousMode != "" && previousMode != "off" {
		if err := a.SetCaptureMode("off"); err != nil {
			return CoreUpdateStatus{}, fmt.Errorf("升级前关闭网络接管失败：%w", err)
		}
	}
	restoreCapture := func() error {
		if previousMode == "" || previousMode == "off" {
			return nil
		}
		return a.SetCaptureMode(previousMode)
	}
	if activeRunning {
		if _, err := call[struct{}](a, "core.update.stop", map[string]any{"core_id": coreID}); err != nil {
			_ = restoreCapture()
			return CoreUpdateStatus{}, fmt.Errorf("停止当前内核失败：%w", err)
		}
	}

	backups, err := readCoreUpdateBackups(binaryPath, manifestPath, sumsPath)
	if err != nil {
		if activeRunning {
			_, _ = call[struct{}](a, "core.update.start", map[string]any{"core_id": coreID})
		}
		_ = restoreCapture()
		return CoreUpdateStatus{}, err
	}
	commitErr := commitCoreUpdate(stagedPath, binaryPath, manifestPath, updatedManifest, sumsPath, updatedSums)
	startedUpdatedCore := false
	if commitErr == nil && activeRunning {
		_, commitErr = call[struct{}](a, "core.update.start", map[string]any{"core_id": coreID})
		startedUpdatedCore = commitErr == nil
	}
	if commitErr == nil {
		commitErr = restoreCapture()
	}
	if commitErr != nil {
		var rollbackErr error
		if startedUpdatedCore {
			_, stopErr := call[struct{}](a, "core.update.stop", map[string]any{"core_id": coreID})
			rollbackErr = errors.Join(rollbackErr, stopErr)
		}
		rollbackErr = errors.Join(rollbackErr, restoreCoreUpdate(backups))
		if activeRunning {
			_, restartErr := call[struct{}](a, "core.update.start", map[string]any{"core_id": coreID})
			rollbackErr = errors.Join(rollbackErr, restartErr)
		}
		rollbackErr = errors.Join(rollbackErr, restoreCapture())
		return CoreUpdateStatus{}, errors.Join(fmt.Errorf("内核升级失败：%w", commitErr), rollbackErr)
	}

	status := CoreUpdateStatus{
		ID: coreID, Name: source.name, CurrentVersion: candidate.version,
		LatestVersion: candidate.version, IntegrityOK: true, ReleaseURL: candidate.releaseURL,
		State: "up_to_date", InstallSupported: false, AssetName: candidate.asset.Name,
	}
	a.coreUpdateMu.Lock()
	a.coreUpdateCache = CoreUpdateReport{Items: []CoreUpdateStatus{status}, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	a.coreUpdateMu.Unlock()
	return status, nil
}

func findCoreReleaseSource(coreID string) (coreReleaseSource, bool) {
	for _, source := range coreReleaseSources {
		if source.id == coreID {
			return source, true
		}
	}
	return coreReleaseSource{}, false
}

func trustedCoreUpdateClient(route coreUpdateHTTPRoute) *http.Client {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 20 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}
	switch {
	case route.proxyURL != nil:
		transport.Proxy = http.ProxyURL(route.proxyURL)
	case route.useEnvironment:
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 || req.URL.Scheme != "https" || !trustedGitHubHost(req.URL.Hostname()) {
				return errors.New("拒绝非 GitHub HTTPS 下载跳转")
			}
			return nil
		},
	}
}

func coreUpdateHTTPRoutes(
	dashboard Dashboard,
	systemProxy systemproxy.ProxyConfig,
	lookupEnv func(string) (string, bool),
) []coreUpdateHTTPRoute {
	routes := make([]coreUpdateHTTPRoute, 0, 4)
	seen := make(map[string]struct{}, 4)
	addProxy := func(name string, proxyURL *url.URL) {
		if proxyURL == nil {
			return
		}
		key := strings.ToLower(proxyURL.String())
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		routes = append(routes, coreUpdateHTTPRoute{name: name, proxyURL: proxyURL})
	}

	if dashboard.Core.State == "running" {
		host := strings.TrimSpace(dashboard.Proxy.Server)
		if host == "" {
			host = "127.0.0.1"
		}
		addProxy("Navo local proxy", loopbackHTTPProxyURL(net.JoinHostPort(host, strconv.Itoa(dashboard.Proxy.Port))))
	}
	if systemProxy.Enabled {
		addProxy("Windows system proxy", winINetHTTPSProxyURL(systemProxy.ProxyServer))
	}
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if value, ok := lookupEnv(key); ok && strings.TrimSpace(value) != "" {
			routes = append(routes, coreUpdateHTTPRoute{name: "environment proxy", useEnvironment: true})
			break
		}
	}
	routes = append(routes, coreUpdateHTTPRoute{name: "direct"})
	return routes
}

func winINetHTTPSProxyURL(value string) *url.URL {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	selected := ""
	if strings.Contains(value, "=") {
		fallback := ""
		for _, item := range strings.Split(value, ";") {
			key, endpoint, found := strings.Cut(strings.TrimSpace(item), "=")
			if !found {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "https":
				selected = strings.TrimSpace(endpoint)
			case "http":
				fallback = strings.TrimSpace(endpoint)
			}
		}
		if selected == "" {
			selected = fallback
		}
	} else {
		selected = value
	}
	return loopbackHTTPProxyURL(selected)
}

func loopbackHTTPProxyURL(value string) *url.URL {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return nil
	}
	host := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil
	}
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil
	}
	return parsed
}

func withCoreUpdateHTTPRoutes[T any](
	ctx context.Context,
	routes []coreUpdateHTTPRoute,
	operation func(*http.Client) (T, error),
) (T, error) {
	var zero T
	var attemptErrors []error
	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			attemptErrors = append(attemptErrors, err)
			break
		}
		client := trustedCoreUpdateClient(route)
		result, err := operation(client)
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
		if err == nil {
			return result, nil
		}
		attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", route.name, err))
	}
	return zero, errors.Join(attemptErrors...)
}

func trustedGitHubHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "github.com" || host == "objects.githubusercontent.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

func fetchInstallCandidate(ctx context.Context, client *http.Client, source coreReleaseSource) (releaseCandidate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.apiURL, nil)
	if err != nil {
		return releaseCandidate{}, fmt.Errorf("创建更新请求失败：%w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Navo/"+diagnosticsAppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return releaseCandidate{}, fmt.Errorf("检查官方版本失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return releaseCandidate{}, fmt.Errorf("检查官方版本返回 HTTP %d", resp.StatusCode)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&release); err != nil {
		return releaseCandidate{}, fmt.Errorf("解析官方版本失败：%w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(release.TagName, "v"))
	if version == "" {
		return releaseCandidate{}, errors.New("官方版本号为空")
	}
	asset := selectReleaseAsset(source.id, version, release.Assets)
	if asset == nil {
		return releaseCandidate{}, errors.New("官方发布未提供可验证的 Windows amd64 asset 与 SHA-256 digest")
	}
	releaseURL := source.releaseURL
	if parsed, parseErr := url.Parse(release.HTMLURL); parseErr == nil && parsed.Scheme == "https" && parsed.Hostname() == "github.com" {
		releaseURL = parsed.String()
	}
	return releaseCandidate{version: version, releaseURL: releaseURL, asset: *asset}, nil
}

func selectReleaseAsset(coreID, version string, assets []githubAsset) *githubAsset {
	wanted := ""
	switch coreID {
	case "sing-box":
		wanted = fmt.Sprintf("sing-box-%s-windows-amd64.zip", version)
	case "mihomo":
		wanted = fmt.Sprintf("mihomo-windows-amd64-compatible-v%s.zip", version)
	case "xray":
		wanted = "Xray-windows-64.zip"
	default:
		return nil
	}
	for index := range assets {
		asset := &assets[index]
		if asset.Name == wanted && asset.Size > 0 && asset.Size <= maxCoreArchiveBytes && validSHA256Digest(asset.Digest) && validGitHubAssetURL(asset.BrowserDownloadURL) {
			return asset
		}
	}
	return nil
}

func validSHA256Digest(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return strings.HasPrefix(value, "sha256:") && err == nil && len(decoded) == sha256.Size
}

func validGitHubAssetURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() == "github.com"
}

func downloadCoreArchive(ctx context.Context, client *http.Client, asset githubAsset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建内核下载请求失败：%w", err)
	}
	req.Header.Set("User-Agent", "Navo/"+diagnosticsAppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载内核失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载内核返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCoreArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取内核下载失败：%w", err)
	}
	if int64(len(data)) > maxCoreArchiveBytes || int64(len(data)) != asset.Size {
		return nil, errors.New("内核下载大小与官方 asset 不一致")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return nil, errors.New("内核下载 SHA-256 校验失败")
	}
	return data, nil
}

func extractCoreExecutable(coreID string, archiveData []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return nil, fmt.Errorf("解析内核压缩包失败：%w", err)
	}
	var selected *zip.File
	for _, file := range reader.File {
		name := strings.ToLower(filepath.Base(filepath.ToSlash(file.Name)))
		match := (coreID == "sing-box" && name == "sing-box.exe") ||
			(coreID == "xray" && name == "xray.exe") ||
			(coreID == "mihomo" && (name == "mihomo.exe" || strings.HasPrefix(name, "mihomo-windows-amd64-")) && strings.HasSuffix(name, ".exe"))
		if !match || file.FileInfo().IsDir() {
			continue
		}
		if selected != nil {
			return nil, errors.New("内核压缩包包含多个候选 executable")
		}
		selected = file
	}
	if selected == nil || selected.UncompressedSize64 == 0 || selected.UncompressedSize64 > uint64(maxCoreArchiveBytes) {
		return nil, errors.New("内核压缩包缺少受支持的 executable")
	}
	stream, err := selected.Open()
	if err != nil {
		return nil, fmt.Errorf("打开内核 executable 失败：%w", err)
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, maxCoreArchiveBytes+1))
	if err != nil || uint64(len(data)) != selected.UncompressedSize64 {
		return nil, errors.New("读取内核 executable 失败")
	}
	return data, nil
}

func stageCoreExecutable(destination string, data []byte) (string, func(), error) {
	temp, err := os.CreateTemp(filepath.Dir(destination), ".navo-core-update-*.exe")
	if err != nil {
		return "", nil, fmt.Errorf("创建内核 staging 文件失败：%w", err)
	}
	path := temp.Name()
	cleanup := func() { _ = temp.Close(); _ = os.Remove(path) }
	if err := temp.Chmod(0o755); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func validateCoreVersion(parent context.Context, path string, args []string, expected string) error {
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	winprocess.ConfigureHidden(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("新内核版本验证失败：%w", err)
	}
	if !strings.Contains(strings.ToLower(string(output)), strings.ToLower(expected)) {
		return fmt.Errorf("新内核输出未包含预期版本 %s", expected)
	}
	return nil
}

func rewriteReleaseChecksums(data []byte, replacements map[string]string) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	found := make(map[string]bool, len(replacements))
	for index, line := range lines {
		match := checksumLinePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		path := filepath.ToSlash(strings.TrimSpace(match[2]))
		if digest, ok := replacements[path]; ok {
			lines[index] = strings.ToLower(digest) + "  " + path
			found[path] = true
		}
	}
	for path := range replacements {
		if !found[path] {
			return nil, fmt.Errorf("SHA256SUMS.txt 缺少 %s", path)
		}
	}
	return []byte(strings.Join(lines, "\r\n")), nil
}

func readCoreUpdateBackups(paths ...string) ([]coreUpdateBackup, error) {
	backups := make([]coreUpdateBackup, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("读取升级备份失败：%w", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取升级备份失败：%w", err)
		}
		backups = append(backups, coreUpdateBackup{path: path, data: data, mode: info.Mode()})
	}
	return backups, nil
}

func commitCoreUpdate(stagedBinary, binaryPath, manifestPath string, manifestData []byte, sumsPath string, sumsData []byte) error {
	if err := fsatomic.ReplaceFile(stagedBinary, binaryPath); err != nil {
		return fmt.Errorf("替换内核 executable 失败：%w", err)
	}
	if err := atomicReplaceBytes(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("替换内核清单失败：%w", err)
	}
	if err := atomicReplaceBytes(sumsPath, sumsData, 0o644); err != nil {
		return fmt.Errorf("替换 release hashes 失败：%w", err)
	}
	return nil
}

func atomicReplaceBytes(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".navo-update-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = temp.Close(); _ = os.Remove(tempPath) }()
	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return fsatomic.ReplaceFile(tempPath, path)
}

func restoreCoreUpdate(backups []coreUpdateBackup) error {
	var result error
	for _, backup := range backups {
		result = errors.Join(result, atomicReplaceBytes(backup.path, backup.data, backup.mode))
	}
	return result
}
