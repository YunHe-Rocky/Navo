// Package tun manages the Wintun virtual network adapter for TUN-mode traffic interception.
// It provides interfaces for adapter lifecycle, route table management, and DNS configuration.
// All operations that modify the network stack require administrator privileges.
package tun

import "context"

// Manager controls a Wintun virtual network adapter.
type Manager interface {
	Create(ctx context.Context, name string) error
	Destroy(ctx context.Context) error
	Configure(ctx context.Context, cfg *Config) error
	Status() *Status
	IsInstalled() bool
	Cleanup(ctx context.Context) (*CleanupResult, error)
}

// RouteManager manages Windows route table entries for TUN traffic.
type RouteManager interface {
	AddRoutes(ctx context.Context, adapterName string, routes []Route) error
	RemoveRoutes(ctx context.Context, adapterName string) error
	ListTUNRoutes(ctx context.Context, adapterName string) ([]Route, error)
	CleanupAll(ctx context.Context) (int, error)
}

// DNSManager manages DNS server configuration on the TUN adapter.
type DNSManager interface {
	Set(ctx context.Context, adapterName string, servers []string) error
	Reset(ctx context.Context, adapterName string) error
	IsConfigured(ctx context.Context, adapterName string) bool
}

// ── Config types ──

// Config holds TUN adapter configuration parameters.
type Config struct {
	Name    string   `json:"name"`
	MTU     int      `json:"mtu"`
	Address []string `json:"address"`
	DNS     []string `json:"dns,omitempty"`
	Gateway string   `json:"gateway,omitempty"`
}

// Status represents the current state of the TUN adapter.
type Status struct {
	Name       string   `json:"name"`
	Identifier string   `json:"identifier,omitempty"`
	State      string   `json:"state"`
	Installed  bool     `json:"installed"`
	Created    bool     `json:"created"`
	Addresses  []string `json:"addresses"`
	MTU        int      `json:"mtu"`
	RouteCount int      `json:"route_count"`
}

// CleanupResult reports what was cleaned up during emergency recovery.
type CleanupResult struct {
	AdapterRemoved bool     `json:"adapter_removed"`
	RoutesCleaned  int      `json:"routes_cleaned"`
	DNSRestored    bool     `json:"dns_restored"`
	Errors         []string `json:"errors,omitempty"`
}

// Route represents a Windows route table entry.
type Route struct {
	Destination   string `json:"destination"`
	Gateway       string `json:"gateway,omitempty"`
	Metric        int    `json:"metric"`
	InterfaceName string `json:"interface_name"`
}

// ── Predefined error codes ──

const (
	ErrNet001 = "NET_001" // wintun.dll not found or failed to load
	ErrNet002 = "NET_002" // adapter creation failed
	ErrNet003 = "NET_003" // IP address configuration failed
	ErrNet004 = "NET_004" // route table modification failed
	ErrNet005 = "NET_005" // DNS configuration failed
	ErrNet006 = "NET_006" // adapter destruction failed
)
