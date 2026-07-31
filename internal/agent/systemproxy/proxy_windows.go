//go:build windows

package systemproxy

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

const (
	HKEY_CURRENT_USER = 0x80000001

	KEY_READ  = 0x20019
	KEY_WRITE = 0x20006

	REG_DWORD = 4
	REG_SZ    = 1

	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxyEnable         = "ProxyEnable"
	proxyServer         = "ProxyServer"
	proxyOverride       = "ProxyOverride"
	autoConfigURL       = "AutoConfigURL"
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

func queryDWORDValue(hKey uintptr, name string) (uint32, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
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
	if ret != 0 {
		return 0, fmt.Errorf("RegQueryValueEx(DWORD): %d", ret)
	}
	return data, nil
}

func queryStringValue(hKey uintptr, name string) (string, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}

	var valType uint32
	var dataSize uint32 = 4096
	buf := make([]uint16, dataSize/2)

	ret, _, _ := procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(0),
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegQueryValueEx(SZ): %d", ret)
	}
	return syscall.UTF16ToString(buf[:dataSize/2-1]), nil
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

	if v, err := queryDWORDValue(hKey, proxyEnable); err == nil {
		cfg.Enabled = v != 0
	}

	if v, err := queryStringValue(hKey, proxyServer); err == nil {
		cfg.ProxyServer = v
	}

	if v, err := queryStringValue(hKey, proxyOverride); err == nil {
		cfg.BypassList = v
	}

	if v, err := queryStringValue(hKey, autoConfigURL); err == nil {
		cfg.AutoConfigURL = v
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
	if cfg.ProxyServer != "" {
		if err := setStringValue(hKey, proxyServer, cfg.ProxyServer); err != nil {
			return fmt.Errorf("restore ProxyServer: %w", err)
		}
	}
	if cfg.BypassList != "" {
		if err := setStringValue(hKey, proxyOverride, cfg.BypassList); err != nil {
			return fmt.Errorf("restore ProxyOverride: %w", err)
		}
	}
	if cfg.AutoConfigURL != "" {
		if err := setStringValue(hKey, autoConfigURL, cfg.AutoConfigURL); err != nil {
			return fmt.Errorf("restore AutoConfigURL: %w", err)
		}
	}
	return nil
}

func notifyProxyChange() error {
	user32 := syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeout := user32.NewProc("SendMessageTimeoutW")

	const HWND_BROADCAST = 0xFFFF
	const WM_SETTINGCHANGE = 0x001A
	const SMTO_ABORTIFHUNG = 0x0002

	envPtr, _ := syscall.UTF16PtrFromString("Environment")

	procSendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		uintptr(0),
		uintptr(unsafe.Pointer(envPtr)),
		uintptr(SMTO_ABORTIFHUNG),
		uintptr(5000),
		uintptr(0),
	)

	return nil
}
