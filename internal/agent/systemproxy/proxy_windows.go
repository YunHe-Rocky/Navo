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

// ProbeDefaultProxy performs a real WinINet request with PRECONFIG settings.
// This proves that a normal current-user Windows application consumes the
// proxy Navo just committed, rather than only proving the explicit endpoint.
func ProbeDefaultProxy(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	agent, err := syscall.UTF16PtrFromString("Navo/1.0 WinINet verification")
	if err != nil {
		return err
	}
	session, _, callErr := procInternetOpenW.Call(
		uintptr(unsafe.Pointer(agent)),
		internetOpenTypePreconfig,
		0,
		0,
		0,
	)
	if session == 0 {
		return fmt.Errorf("InternetOpenW(PRECONFIG): %w", callErr)
	}
	defer procInternetClose.Call(session)

	timeout := uint32(1500)
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
	for _, endpoint := range []string{
		"https://connectivitycheck.gstatic.com/generate_204",
		"https://cp.cloudflare.com/generate_204",
		"https://www.msftconnecttest.com/connecttest.txt",
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		url, _ := syscall.UTF16PtrFromString(endpoint)
		request, _, requestErr := procInternetOpenURLW.Call(
			session,
			uintptr(unsafe.Pointer(url)),
			0,
			0,
			internetFlagReload|internetFlagNoCacheWrite,
			0,
		)
		if request == 0 {
			lastErr = fmt.Errorf("InternetOpenUrlW(%s): %w", endpoint, requestErr)
			continue
		}
		var status uint32
		size := uint32(unsafe.Sizeof(status))
		ok, _, statusErr := procHTTPQueryInfoW.Call(
			request,
			httpQueryStatusCode|httpQueryFlagNumber,
			uintptr(unsafe.Pointer(&status)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		procInternetClose.Call(request)
		if ok != 0 && status >= 200 && status < 400 {
			return nil
		}
		lastErr = fmt.Errorf("WinINet status for %s: status=%d error=%v", endpoint, status, statusErr)
	}
	return fmt.Errorf("current-user default proxy data plane failed: %w", lastErr)
}
