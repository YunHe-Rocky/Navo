//go:build windows

package tun

import (
	"context"
	"fmt"
	"strings"
)

type dnsManager struct{}

// NewDNSManager creates a Windows DNS manager using netsh.
func NewDNSManager() DNSManager {
	return &dnsManager{}
}

func (d *dnsManager) Set(ctx context.Context, adapterName string, servers []string) error {
	for _, server := range servers {
		cmd := hiddenCommandContext(ctx, "netsh",
			"interface", "ip", "set", "dns",
			adapterName, "static", server,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: netsh set dns %s failed: %v (output: %s)",
				ErrNet005, server, err, string(out))
		}
	}
	return nil
}

func (d *dnsManager) Reset(ctx context.Context, adapterName string) error {
	cmd := hiddenCommandContext(ctx, "netsh",
		"interface", "ip", "delete", "dns",
		adapterName, "all",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: netsh delete dns failed: %v (output: %s)",
			ErrNet005, err, string(out))
	}
	return nil
}

func (d *dnsManager) IsConfigured(ctx context.Context, adapterName string) bool {
	cmd := hiddenCommandContext(ctx, "netsh",
		"interface", "ip", "show", "dns",
		adapterName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// If output contains any DNS servers other than DHCP-assigned, it's configured
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "DNS") && !strings.Contains(line, "DHCP") && !strings.Contains(line, "None") {
			return true
		}
	}
	return false
}
