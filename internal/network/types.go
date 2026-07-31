// Package network manages the privileged Windows networking changes required by TUN mode.
package network

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// IPv6Mode controls how IPv6 traffic is handled while TUN mode is active.
type IPv6Mode string

const (
	IPv6Block       IPv6Mode = "block"
	IPv6Tunnel      IPv6Mode = "tunnel"
	IPv6Passthrough IPv6Mode = "passthrough"
	// BundledWintunSHA256 is the official Wintun 0.14.1 AMD64 DLL digest.
	BundledWintunSHA256 = "e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce"
)

var adapterNamePattern = regexp.MustCompile(`^[\pL\pN_. -]{1,128}$`)
var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// Config describes the system networking policy surrounding a sing-box TUN inbound.
type Config struct {
	Enabled          bool          `json:"enabled"`
	AdapterName      string        `json:"adapter_name"`
	WintunDLLPath    string        `json:"wintun_dll_path"`
	WintunSHA256     string        `json:"wintun_sha256"`
	JournalPath      string        `json:"journal_path"`
	TUNIPv4Gateway   string        `json:"tun_ipv4_gateway"`
	TUNIPv6Gateway   string        `json:"tun_ipv6_gateway,omitempty"`
	DNSServers       []string      `json:"dns_servers"`
	ProxyEndpointIPs []string      `json:"proxy_endpoint_ips,omitempty"`
	IPv6Mode         IPv6Mode      `json:"ipv6_mode"`
	AdapterTimeout   time.Duration `json:"-"`
}

func (c *Config) withDefaults() {
	if c.AdapterName == "" {
		c.AdapterName = "Navo"
	}
	if c.TUNIPv4Gateway == "" {
		c.TUNIPv4Gateway = "172.19.0.2"
	}
	if c.WintunSHA256 == "" {
		c.WintunSHA256 = BundledWintunSHA256
	}
	if len(c.DNSServers) == 0 {
		c.DNSServers = []string{c.TUNIPv4Gateway}
	}
	if c.IPv6Mode == "" {
		c.IPv6Mode = IPv6Block
	}
	if c.AdapterTimeout <= 0 {
		c.AdapterTimeout = 15 * time.Second
	}
}

func (c Config) validate() error {
	if !adapterNamePattern.MatchString(c.AdapterName) {
		return fmt.Errorf("invalid adapter name %q", c.AdapterName)
	}
	if !sha256Pattern.MatchString(c.WintunSHA256) {
		return fmt.Errorf("invalid Wintun SHA-256")
	}
	if net.ParseIP(c.TUNIPv4Gateway) == nil || strings.Contains(c.TUNIPv4Gateway, ":") {
		return fmt.Errorf("invalid IPv4 TUN gateway %q", c.TUNIPv4Gateway)
	}
	for _, server := range c.DNSServers {
		ip := net.ParseIP(server)
		if ip == nil {
			return fmt.Errorf("invalid DNS server %q", server)
		}
		if c.IPv6Mode == IPv6Block && strings.Contains(server, ":") {
			return fmt.Errorf("IPv6 DNS server %q is incompatible with IPv6 block mode", server)
		}
	}
	for _, endpoint := range c.ProxyEndpointIPs {
		if ip := net.ParseIP(endpoint); ip == nil {
			return fmt.Errorf("invalid proxy endpoint IP %q", endpoint)
		}
		if c.IPv6Mode == IPv6Block && strings.Contains(endpoint, ":") {
			return fmt.Errorf("IPv6 proxy endpoint %q is incompatible with IPv6 block mode", endpoint)
		}
	}
	switch c.IPv6Mode {
	case IPv6Block, IPv6Tunnel, IPv6Passthrough:
	default:
		return fmt.Errorf("invalid IPv6 mode %q", c.IPv6Mode)
	}
	if c.IPv6Mode == IPv6Tunnel {
		if ip := net.ParseIP(c.TUNIPv6Gateway); ip == nil || !strings.Contains(c.TUNIPv6Gateway, ":") {
			return fmt.Errorf("IPv6 tunnel mode requires a valid IPv6 TUN gateway")
		}
	}
	return nil
}

// Command is an executable invocation. Arguments are kept separate to avoid shell quoting bugs.
type Command struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// Executor runs a privileged system command.
type Executor interface {
	Run(ctx context.Context, command Command) error
}

// Platform provides Windows-specific privilege, Wintun, and adapter checks.
type Platform interface {
	Preflight(ctx context.Context, cfg Config) error
	WaitForAdapter(ctx context.Context, name string, timeout time.Duration) error
}
