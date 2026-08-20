//go:build windows

package systemproxy

import (
	"context"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	wininet  = syscall.NewLazyDLL("wininet.dll")

	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
	procInternetOpenW    = wininet.NewProc("InternetOpenW")
	procInternetOpenURLW = wininet.NewProc("InternetOpenUrlW")
	procInternetSetOptW  = wininet.NewProc("InternetSetOptionW")
	procInternetClose    = wininet.NewProc("InternetCloseHandle")
	procHTTPQueryInfoW   = wininet.NewProc("HttpQueryInfoW")
)

const (
	internetOpenTypePreconfig = 0
	internetOpenTypeDirect    = 1
	internetFlagReload        = 0x80000000
	internetFlagNoCacheWrite  = 0x04000000
	internetOptionConnectMS   = 2
	internetOptionSendMS      = 5
	internetOptionReceiveMS   = 6
	httpQueryStatusCode       = 19
	httpQueryFlagNumber       = 0x20000000
)

const (
	HKEY_CURRENT_USER = 0x80000001

	KEY_READ  = 0x20019
	KEY_WRITE = 0x20006

	REG_DWORD         = 4
	REG_SZ            = 1
	REG_EXPAND_SZ     = 2
	errorFileNotFound = 2

	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxyEnable         = "ProxyEnable"
	proxyServer         = "ProxyServer"
	proxyOverride       = "ProxyOverride"
	autoConfigURL       = "AutoConfigURL"
	autoDetect          = "AutoDetect"
)

func openRegKey(key uintptr, subKey string, access uint32) (uintptr, error) {
	var hKey uintptr
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, err
	}

	ret, _, _ := procRegOpenKeyExW.Call(
		key,
		uintptr(unsafe.Pointer(subKeyPtr)),
		uintptr(0),
		uintptr(access),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("RegOpenKeyEx failed: %d", ret)
	}
	return hKey, nil
}

func closeRegKey(hKey uintptr) {
	procRegCloseKey.Call(hKey)
}

func queryDWORDValue(hKey uintptr, name string) (uint32, bool, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}

	var valType uint32
	var data uint32
	var dataSize uint32 = 4

	ret, _, _ := procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret == errorFileNotFound {
		return 0, false, nil
	}
	if ret != 0 {
		return 0, false, fmt.Errorf("RegQueryValueEx(DWORD %s): %d", name, ret)
	}
	if valType != REG_DWORD || dataSize != 4 {
		return 0, false, fmt.Errorf("registry value %s is not a DWORD", name)
	}
	return data, true, nil
}

func queryStringValue(hKey uintptr, name string) (string, bool, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false, err
	}

	var valType uint32
	var dataSize uint32
	ret, _, _ := procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(unsafe.Pointer(&valType)),
		uintptr(0),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret == errorFileNotFound {
		return "", false, nil
	}
	if ret != 0 {
		return "", false, fmt.Errorf("RegQueryValueEx(size %s): %d", name, ret)
	}
	if valType != REG_SZ && valType != REG_EXPAND_SZ {
		return "", false, fmt.Errorf("registry value %s is not a string", name)
	}
	if dataSize == 0 {
		return "", true, nil
	}
	buf := make([]uint16, (dataSize+1)/2)
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret != 0 {
		return "", false, fmt.Errorf("RegQueryValueEx(data %s): %d", name, ret)
	}
	return syscall.UTF16ToString(buf), true, nil
}

func deleteValue(hKey uintptr, name string) error {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	ret, _, _ := procRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(namePtr)))
	if ret != 0 && ret != errorFileNotFound {
		return fmt.Errorf("RegDeleteValue(%s): %d", name, ret)
	}
	return nil
}

func setDWORDValue(hKey uintptr, name string, val uint32) error {
	namePtr, _ := syscall.UTF16PtrFromString(name)
	ret, _, _ := procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(REG_DWORD),
		uintptr(unsafe.Pointer(&val)),
		uintptr(4),
	)
	if ret != 0 {
		return fmt.Errorf("RegSetValueEx(DWORD): %d", ret)
	}
	return nil
}

func setStringValue(hKey uintptr, name string, val string) error {
	namePtr, _ := syscall.UTF16PtrFromString(name)
	valUTF16, _ := syscall.UTF16FromString(val)
	valSize := uint32(len(valUTF16) * 2)

	ret, _, _ := procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&valUTF16[0])),
		uintptr(valSize),
	)
	if ret != 0 {
		return fmt.Errorf("RegSetValueEx(SZ): %d", ret)
	}
	return nil
}

func getSystemProxy() (*ProxyConfig, error) {
	hKey, err := openRegKey(HKEY_CURRENT_USER, internetSettingsKey, KEY_READ)
	if err != nil {
		return nil, err
	}
	defer closeRegKey(hKey)

	cfg := &ProxyConfig{}

	if v, present, err := queryDWORDValue(hKey, proxyEnable); err != nil {
		return nil, err
	} else if !present {
		return nil, fmt.Errorf("required registry value %s is missing", proxyEnable)
	} else {
		cfg.Enabled = v != 0
	}

	if v, present, err := queryStringValue(hKey, proxyServer); err != nil {
		return nil, err
	} else {
		cfg.ProxyServer, cfg.ProxyServerPresent = v, present
	}

	if v, present, err := queryStringValue(hKey, proxyOverride); err != nil {
		return nil, err
	} else {
		cfg.BypassList, cfg.BypassListPresent = v, present
	}

	if v, present, err := queryStringValue(hKey, autoConfigURL); err != nil {
		return nil, err
	} else {
		cfg.AutoConfigURL, cfg.AutoConfigURLPresent = v, present
	}
	if v, present, err := queryDWORDValue(hKey, autoDetect); err != nil {
		return nil, err
	} else {
		cfg.AutoDetect, cfg.AutoDetectPresent = v != 0, present
	}

	return cfg, nil
}

func setSystemProxy(server string) error {
	hKey, err := openRegKey(HKEY_CURRENT_USER, internetSettingsKey, KEY_WRITE)
	if err != nil {
		return err
	}
	defer closeRegKey(hKey)

	if err := setDWORDValue(hKey, proxyEnable, 1); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := setStringValue(hKey, proxyServer, server); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}

	return nil
}

func disableSystemProxy() error {
	hKey, err := openRegKey(HKEY_CURRENT_USER, internetSettingsKey, KEY_WRITE)
	if err != nil {
		return err
	}
	defer closeRegKey(hKey)

	return setDWORDValue(hKey, proxyEnable, 0)
}

func applySystemProxyConfig(cfg ProxyConfig) error {
	hKey, err := openRegKey(HKEY_CURRENT_USER, internetSettingsKey, KEY_WRITE)
	if err != nil {
		return err
	}
	defer closeRegKey(hKey)
	enabled := uint32(0)
	if cfg.Enabled && cfg.ProxyServer != "" {
		enabled = 1
	}
	if err := setDWORDValue(hKey, proxyEnable, enabled); err != nil {
		return fmt.Errorf("restore ProxyEnable: %w", err)
	}
	if cfg.ProxyServerPresent || cfg.ProxyServer != "" {
		if err := setStringValue(hKey, proxyServer, cfg.ProxyServer); err != nil {
			return fmt.Errorf("restore ProxyServer: %w", err)
		}
	} else if err := deleteValue(hKey, proxyServer); err != nil {
		return err
	}
	if cfg.BypassListPresent || cfg.BypassList != "" {
		if err := setStringValue(hKey, proxyOverride, cfg.BypassList); err != nil {
			return fmt.Errorf("restore ProxyOverride: %w", err)
		}
	} else if err := deleteValue(hKey, proxyOverride); err != nil {
		return err
	}
	if cfg.AutoConfigURLPresent || cfg.AutoConfigURL != "" {
		if err := setStringValue(hKey, autoConfigURL, cfg.AutoConfigURL); err != nil {
			return fmt.Errorf("restore AutoConfigURL: %w", err)
		}
	} else if err := deleteValue(hKey, autoConfigURL); err != nil {
		return err
	}
	if cfg.AutoDetectPresent {
		value := uint32(0)
		if cfg.AutoDetect {
			value = 1
		}
		if err := setDWORDValue(hKey, autoDetect, value); err != nil {
			return fmt.Errorf("restore AutoDetect: %w", err)
		}
	} else if err := deleteValue(hKey, autoDetect); err != nil {
		return err
	}
	return nil
}

func notifyProxyChange() error {
	for _, option := range []uintptr{39, 37} { // SETTINGS_CHANGED, REFRESH
		ok, _, callErr := procInternetSetOptW.Call(0, option, 0, 0)
		if ok == 0 {
			return fmt.Errorf("InternetSetOptionW(%d): %w", option, callErr)
		}
	}
	return nil
}

type winINetApplicationProbe struct {
	name     string
	endpoint string
	expected []uint32
}

var winINetChatGPTProbes = []winINetApplicationProbe{
	{name: "chatgpt-web", endpoint: "https://chatgpt.com/", expected: []uint32{200, 403}},
	{name: "chatgpt-auth", endpoint: "https://auth.openai.com/", expected: []uint32{200, 302, 303, 307, 308, 403}},
	{name: "openai-api", endpoint: "https://api.openai.com/v1/models", expected: []uint32{401}},
	{name: "chatgpt-assets", endpoint: "https://persistent.oaistatic.com/", expected: []uint32{200, 403, 404}},
	{name: "chatgpt-stream", endpoint: "https://ws.chatgpt.com/", expected: []uint32{200, 400, 401, 403, 404}},
}

var winINetProxyConnectivityEndpoints = []string{
	"https://connectivitycheck.gstatic.com/generate_204",
	"https://cp.cloudflare.com/generate_204",
	"https://www.msftconnecttest.com/connecttest.txt",
}

var winINetDirectConnectivityEndpoints = []string{
	"https://www.baidu.com/",
	"https://connect.rom.miui.com/generate_204",
}

const winINetApplicationProbeAttempts = 2

// ProbeDefaultProxy performs a real WinINet request with PRECONFIG settings.
// This proves that a normal current-user Windows application consumes the
// proxy Navo just committed, rather than only proving the explicit endpoint.
func ProbeDefaultProxy(ctx context.Context) error {
	return probeWinINet(ctx, internetOpenTypePreconfig, "default proxy", winINetProxyConnectivityEndpoints, true)
}

// ProbeDefaultDirectRouting proves that a PRECONFIG WinINet application still
// enters Navo while the active runtime policy intentionally routes direct.
func ProbeDefaultDirectRouting(ctx context.Context) error {
	return probeWinINet(ctx, internetOpenTypePreconfig, "default proxy/direct routing", winINetDirectConnectivityEndpoints, false)
}

// ProbeDirect performs a current-user WinINet request without an explicit or
// configured proxy. In TUN mode this exercises the Windows host routing path
// independently from the Service process verifier.
func ProbeDirect(ctx context.Context) error {
	return probeWinINet(ctx, internetOpenTypeDirect, "direct/TUN", winINetProxyConnectivityEndpoints, true)
}

// ProbeDirectRouting verifies an intentionally direct TUN policy without
// requiring resources that are expected to need a proxy in the current region.
func ProbeDirectRouting(ctx context.Context) error {
	return probeWinINet(ctx, internetOpenTypeDirect, "direct/TUN routing", winINetDirectConnectivityEndpoints, false)
}

func probeWinINet(ctx context.Context, accessType uintptr, pathName string, connectivityEndpoints []string, requireChatGPT bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	agent, err := syscall.UTF16PtrFromString("Navo/1.0 WinINet verification")
	if err != nil {
		return err
	}
	session, _, callErr := procInternetOpenW.Call(
		uintptr(unsafe.Pointer(agent)),
		accessType,
		0,
		0,
		0,
	)
	if session == 0 {
		return fmt.Errorf("InternetOpenW(%s): %w", pathName, callErr)
	}
	defer procInternetClose.Call(session)

	timeout := uint32(2500)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		if remaining < time.Duration(timeout)*time.Millisecond {
			timeout = uint32(max(250, remaining.Milliseconds()))
		}
	}
	for _, option := range []uintptr{internetOptionConnectMS, internetOptionSendMS, internetOptionReceiveMS} {
		ok, _, optionErr := procInternetSetOptW.Call(
			session,
			option,
			uintptr(unsafe.Pointer(&timeout)),
			unsafe.Sizeof(timeout),
		)
		if ok == 0 {
			return fmt.Errorf("InternetSetOptionW(timeout %d): %w", option, optionErr)
		}
	}

	var lastErr error
	genericReady := false
	for _, endpoint := range connectivityEndpoints {
		if err := ctx.Err(); err != nil {
			return err
		}
		status, probeErr := winINetStatus(session, endpoint)
		if probeErr == nil && status >= 200 && status < 400 {
			genericReady = true
			break
		}
		lastErr = fmt.Errorf("WinINet status for %s: status=%d error=%v", endpoint, status, probeErr)
	}
	if !genericReady {
		return fmt.Errorf("current-user %s data plane failed: %w", pathName, lastErr)
	}
	if !requireChatGPT {
		return nil
	}

	for _, probe := range winINetChatGPTProbes {
		if err := ctx.Err(); err != nil {
			return err
		}
		status, probeErr := winINetApplicationStatus(ctx, session, probe)
		if probeErr != nil {
			return fmt.Errorf("current-user %s ChatGPT route %s failed: %w", pathName, probe.name, probeErr)
		}
		if !winINetStatusAccepted(status, probe.expected) {
			return fmt.Errorf(
				"current-user %s ChatGPT route %s returned unexpected status %d (expected %v)",
				pathName, probe.name, status, probe.expected,
			)
		}
	}
	return nil
}

func winINetApplicationStatus(ctx context.Context, session uintptr, probe winINetApplicationProbe) (uint32, error) {
	var status uint32
	var lastErr error
	for attempt := 0; attempt < winINetApplicationProbeAttempts; attempt++ {
		status, lastErr = winINetStatus(session, probe.endpoint)
		if lastErr == nil && winINetStatusAccepted(status, probe.expected) {
			return status, nil
		}
		if attempt+1 == winINetApplicationProbeAttempts {
			break
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return status, ctx.Err()
		case <-timer.C:
		}
	}
	return status, lastErr
}

func winINetStatus(session uintptr, endpoint string) (uint32, error) {
	url, err := syscall.UTF16PtrFromString(endpoint)
	if err != nil {
		return 0, err
	}
	request, _, requestErr := procInternetOpenURLW.Call(
		session,
		uintptr(unsafe.Pointer(url)),
		0,
		0,
		internetFlagReload|internetFlagNoCacheWrite,
		0,
	)
	if request == 0 {
		return 0, fmt.Errorf("InternetOpenUrlW(%s): %w", endpoint, requestErr)
	}
	defer procInternetClose.Call(request)

	var status uint32
	size := uint32(unsafe.Sizeof(status))
	ok, _, statusErr := procHTTPQueryInfoW.Call(
		request,
		httpQueryStatusCode|httpQueryFlagNumber,
		uintptr(unsafe.Pointer(&status)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if ok == 0 {
		return status, fmt.Errorf("HttpQueryInfoW(%s): %w", endpoint, statusErr)
	}
	return status, nil
}

func winINetStatusAccepted(status uint32, expected []uint32) bool {
	for _, candidate := range expected {
		if status == candidate {
			return true
		}
	}
	return false
}
