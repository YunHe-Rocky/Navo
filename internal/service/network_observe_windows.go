//go:build windows

package service

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"navo/internal/network"
	"navo/internal/networkenv"
)

func observePlatformNetwork(ctx context.Context) (platformNetworkObservation, error) {
	result := platformNetworkObservation{Physical: networkenv.PhysicalSnapshot{Known: true}}
	interfaces, err := net.Interfaces()
	if err != nil {
		result.Physical.Known = false
		result.Physical.LastError = fmt.Sprintf("enumerate Windows interfaces: %v", err)
		return result, err
	}
	for _, adapter := range interfaces {
		if err := ctx.Err(); err != nil {
			result.Physical.Known = false
			result.Physical.LastError = err.Error()
			return result, err
		}
		if adapter.Flags&net.FlagLoopback != 0 || strings.EqualFold(strings.TrimSpace(adapter.Name), network.OwnedTUNAdapterName) {
			continue
		}
		state := "down"
		if adapter.Flags&net.FlagUp != 0 {
			state = "up"
		}
		if likelyVirtualAdapter(adapter.Name) {
			result.External = append(result.External, networkenv.ExternalAdapterRef{
				Name: adapter.Name, InterfaceIndex: adapter.Index, State: state,
			})
			continue
		}
		if adapter.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, addressErr := adapter.Addrs()
		if addressErr != nil {
			continue
		}
		if hasUsableAddress(addresses) {
			result.Physical.Available = true
			result.Physical.ActiveInterfaces = append(result.Physical.ActiveInterfaces, adapter.Name)
		}
	}
	sort.Strings(result.Physical.ActiveInterfaces)
	sort.Slice(result.External, func(i, j int) bool {
		return strings.ToLower(result.External[i].Name) < strings.ToLower(result.External[j].Name)
	})
	return result, nil
}

func likelyVirtualAdapter(name string) bool {
	value := strings.ToLower(strings.TrimSpace(name))
	for _, token := range []string{
		"wintun", "wireguard", "tailscale", "zerotier", "tap", "tun", "vpn",
		"vethernet", "hyper-v", "vmware", "virtualbox", "docker", "wsl", "clash",
	} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func hasUsableAddress(addresses []net.Addr) bool {
	for _, address := range addresses {
		raw := address.String()
		if host, _, err := net.ParseCIDR(raw); err == nil {
			if !host.IsLoopback() && !host.IsUnspecified() && !host.IsLinkLocalUnicast() {
				return true
			}
			continue
		}
		if host := net.ParseIP(raw); host != nil && !host.IsLoopback() && !host.IsUnspecified() && !host.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}
