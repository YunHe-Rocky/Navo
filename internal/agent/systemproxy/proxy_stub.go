//go:build !windows

package systemproxy

import "context"

// On non-Windows platforms, system proxy operations are no-ops.
// Android/Linux/macOS use different proxy mechanisms (VPN mode, etc.).

func getSystemProxy() (*ProxyConfig, error) {
	return &ProxyConfig{}, nil
}

func setSystemProxy(server string) error {
	return nil
}

func disableSystemProxy() error {
	return nil
}

func applySystemProxyConfig(cfg ProxyConfig) error {
	if cfg.Enabled && cfg.ProxyServer != "" {
		return setSystemProxy(cfg.ProxyServer)
	}
	return disableSystemProxy()
}

func notifyProxyChange() error {
	return nil
}

func ProbeDefaultProxy(context.Context) error { return nil }
