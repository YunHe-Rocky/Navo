// Package network manages the privileged Windows networking changes required by TUN mode.
package network

import (
	"context"
	"errors"
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
	// OwnedTUNAdapterName is the only Windows adapter Navo may create,
	// configure, inspect for recovery, or bind split routes to.
	OwnedTUNAdapterName     = "Navo"
	ownedTUNDescription     = "Wintun Userspace Tunnel"
	ownedSingTUNDescription = "sing-tun Tunnel"
	// BundledWintunSHA256 is the official Wintun 0.14.1 AMD64 DLL digest.
	BundledWintunSHA256 = "e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce"
)

var adapterNamePattern = regexp.MustCompile(`^[\pL\pN_. -]{1,128}$`)
var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// AdapterSnapshot is the observed Windows identity and configuration of one
// adapter. Production decisions never rely on the mutable display name alone.
type AdapterSnapshot struct {
	Name                 string   `json:"name"`
	InterfaceDescription string   `json:"interface_description"`
	HardwareInterface    bool     `json:"hardware_interface"`
	InterfaceIndex       uint32   `json:"interface_index"`
	InterfaceGUID        string   `json:"interface_guid"`
	InterfaceLUID        uint64   `json:"interface_luid,omitempty"`
	OperationalStatus    string   `json:"operational_status"`
	MTU                  int      `json:"mtu"`
	IPv4Addresses        []string `json:"ipv4_addresses"`
	IPv6Addresses        []string `json:"ipv6_addresses,omitempty"`
}

func isOwnedTUNAdapter(snapshot AdapterSnapshot) bool {
	description := strings.TrimSpace(snapshot.InterfaceDescription)
	descriptionOwned := isOwnedTUNDescription(description, ownedTUNDescription) ||
		isOwnedTUNDescription(description, ownedSingTUNDescription)
	return strings.EqualFold(strings.TrimSpace(snapshot.Name), OwnedTUNAdapterName) &&
		descriptionOwned &&
		!snapshot.HardwareInterface && snapshot.InterfaceIndex > 0 && snapshot.InterfaceGUID != ""
}

func isOwnedTUNDescription(description, base string) bool {
	if strings.EqualFold(description, base) {
		return true
	}
	prefix := base + " #"
	if len(description) <= len(prefix) || !strings.EqualFold(description[:len(prefix)], prefix) {
		return false
	}
	for _, char := range description[len(prefix):] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// EndpointRoutePlan freezes the physical route used by the selected proxy
// endpoint before the TUN adapter or split-default routes exist.
type EndpointRoutePlan struct {
	EndpointHost    string `json:"endpoint_host"`
	EndpointIP      string `json:"endpoint_ip"`
	AddressFamily   string `json:"address_family"`
	InterfaceIndex  uint32 `json:"interface_index"`
	InterfaceGUID   string `json:"interface_guid"`
	InterfaceAlias  string `json:"interface_alias"`
	NextHop         string `json:"next_hop"`
	RouteMetric     int    `json:"route_metric"`
	InterfaceMetric int    `json:"interface_metric"`
}

// TUNActivationPlan is immutable for one activation attempt.
type TUNActivationPlan struct {
	SessionID          string              `json:"session_id"`
	CoreID             string              `json:"core_id"`
	AdapterName        string              `json:"adapter_name"`
	TUNIPv4Address     string              `json:"tun_ipv4_address"`
	TUNIPv4Peer        string              `json:"tun_ipv4_peer"`
	TUNDNSIPv4         string              `json:"tun_dns_ipv4"`
	MTU                int                 `json:"mtu"`
	SelectedOutboundID string              `json:"selected_outbound_id,omitempty"`
	OriginalServerHost string              `json:"original_server_host,omitempty"`
	PinnedServerIP     string              `json:"pinned_server_ip,omitempty"`
	PhysicalRoute      EndpointRoutePlan   `json:"physical_route"`
	EndpointRoutes     []EndpointRoutePlan `json:"endpoint_routes,omitempty"`
	IPv6Mode           IPv6Mode            `json:"ipv6_mode"`
	CreatedAt          time.Time           `json:"created_at"`
}

func (p TUNActivationPlan) validate() error {
	if p.SessionID == "" || len(p.SessionID) > 64 {
		return fmt.Errorf("invalid TUN session id")
	}
	if !adapterNamePattern.MatchString(p.AdapterName) {
		return fmt.Errorf("invalid adapter name %q", p.AdapterName)
	}
	if !strings.EqualFold(strings.TrimSpace(p.AdapterName), OwnedTUNAdapterName) {
		return fmt.Errorf("adapter %q is outside Navo ownership", p.AdapterName)
	}
	if p.MTU < 1280 || p.MTU > 9000 {
		return fmt.Errorf("invalid TUN MTU %d", p.MTU)
	}
	if _, _, err := net.ParseCIDR(p.TUNIPv4Address); err != nil || strings.Contains(p.TUNIPv4Address, ":") {
		return fmt.Errorf("invalid TUN IPv4 address %q", p.TUNIPv4Address)
	}
	for field, value := range map[string]string{"peer": p.TUNIPv4Peer, "DNS": p.TUNDNSIPv4} {
		if ip := net.ParseIP(value); ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid TUN IPv4 %s %q", field, value)
		}
	}
	if p.OriginalServerHost != "" {
		endpointIP := net.ParseIP(p.PinnedServerIP)
		if endpointIP == nil {
			return fmt.Errorf("proxy endpoint is not pinned")
		}
		if endpointIP.IsLoopback() {
			if endpointIP.To4() == nil || len(p.EndpointRoutes) != 0 {
				return fmt.Errorf("local proxy endpoint has an invalid bypass route")
			}
		} else if len(p.EndpointRoutes) == 0 {
			return fmt.Errorf("proxy endpoint is not pinned to a physical route")
		}
	}
	if p.PhysicalRoute.InterfaceIndex == 0 || p.PhysicalRoute.InterfaceGUID == "" || p.PhysicalRoute.InterfaceAlias == "" || p.PhysicalRoute.NextHop == "" {
		return fmt.Errorf("physical outbound interface is not frozen")
	}
	if strings.EqualFold(p.PhysicalRoute.InterfaceAlias, p.AdapterName) {
		return fmt.Errorf("physical outbound interface cannot be the TUN adapter")
	}
	for _, route := range p.EndpointRoutes {
		if net.ParseIP(route.EndpointIP) == nil || route.InterfaceIndex == 0 || route.InterfaceGUID == "" || route.NextHop == "" {
			return fmt.Errorf("invalid frozen endpoint route for %q", route.EndpointIP)
		}
		if strings.EqualFold(route.InterfaceAlias, p.AdapterName) {
			return fmt.Errorf("endpoint route cannot use the TUN adapter")
		}
	}
	return nil
}

type TUNStage string

const (
	TUNStagePreflight            TUNStage = "PREFLIGHT"
	TUNStageBaselineCaptured     TUNStage = "BASELINE_CAPTURED"
	TUNStageConfigCompiled       TUNStage = "CONFIG_COMPILED"
	TUNStageCoreStarted          TUNStage = "CORE_STARTED"
	TUNStageAdapterReady         TUNStage = "ADAPTER_READY"
	TUNStageNetworkApplied       TUNStage = "NETWORK_APPLIED"
	TUNStageControlPlaneVerified TUNStage = "CONTROL_PLANE_VERIFIED"
	TUNStageDataPlaneVerified    TUNStage = "DATA_PLANE_VERIFIED"
	TUNStageHealthCommitted      TUNStage = "HEALTH_COMMITTED"
)

const (
	ErrTUNPreflightFailed        = "TUN_PREFLIGHT_FAILED"
	ErrTUNEndpointResolveFailed  = "TUN_ENDPOINT_RESOLVE_FAILED"
	ErrTUNPhysicalRouteNotFound  = "TUN_PHYSICAL_ROUTE_NOT_FOUND"
	ErrTUNEndpointPinFailed      = "TUN_ENDPOINT_PIN_FAILED"
	ErrTUNCoreStartFailed        = "TUN_CORE_START_FAILED"
	ErrTUNAdapterNotReady        = "TUN_ADAPTER_NOT_READY"
	ErrTUNAdapterConflict        = "TUN_ADAPTER_CONFLICT"
	ErrTUNEndpointBypassFailed   = "TUN_ENDPOINT_BYPASS_FAILED"
	ErrTUNSplitRouteFailed       = "TUN_SPLIT_ROUTE_FAILED"
	ErrTUNNRPTFailed             = "TUN_NRPT_FAILED"
	ErrTUNIPv6PolicyFailed       = "TUN_IPV6_POLICY_FAILED"
	ErrTUNEndpointLoopDetected   = "TUN_ENDPOINT_LOOP_DETECTED"
	ErrTUNPublicRouteNotCaptured = "TUN_PUBLIC_ROUTE_NOT_CAPTURED"
	ErrTUNDNSVerifyFailed        = "TUN_DNS_VERIFY_FAILED"
	ErrTUNTCPVerifyFailed        = "TUN_TCP_VERIFY_FAILED"
	ErrTUNHTTPSVerifyFailed      = "TUN_HTTPS_VERIFY_FAILED"
	ErrTUNExitIPMismatch         = "TUN_EXIT_IP_MISMATCH"
	ErrTUNUDPVerifyFailed        = "TUN_UDP_VERIFY_FAILED"
	ErrTUNRollbackFailed         = "TUN_ROLLBACK_FAILED"
	ErrTUNRecoveryDirty          = "TUN_RECOVERY_DIRTY"
)

// TUNError carries a stable code and enough structured context to diagnose a
// failed transition without logging node credentials.
type TUNError struct {
	Code           string   `json:"code"`
	Stage          TUNStage `json:"stage"`
	Resource       string   `json:"resource,omitempty"`
	Expected       string   `json:"expected,omitempty"`
	Actual         string   `json:"actual,omitempty"`
	RollbackStatus string   `json:"rollback_status,omitempty"`
	Cause          error    `json:"-"`
}

func (e *TUNError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("%s: stage=%s", e.Code, e.Stage)
	if e.Resource != "" {
		message += " resource=" + e.Resource
	}
	if e.Expected != "" {
		message += " expected=" + e.Expected
	}
	if e.Actual != "" {
		message += " actual=" + e.Actual
	}
	if e.RollbackStatus != "" {
		message += " rollback=" + e.RollbackStatus
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *TUNError) Unwrap() error { return e.Cause }

func asTUNError(err error) *TUNError {
	var target *TUNError
	if errors.As(err, &target) {
		return target
	}
	return nil
}

// Config describes the system networking policy surrounding a sing-box TUN inbound.
type Config struct {
	Enabled          bool              `json:"enabled"`
	AdapterName      string            `json:"adapter_name"`
	WintunDLLPath    string            `json:"wintun_dll_path"`
	WintunSHA256     string            `json:"wintun_sha256"`
	JournalPath      string            `json:"journal_path"`
	TUNIPv4Gateway   string            `json:"tun_ipv4_gateway"`
	TUNIPv4Address   string            `json:"tun_ipv4_address"`
	TUNIPv4Peer      string            `json:"tun_ipv4_peer"`
	TUNDNSIPv4       string            `json:"tun_dns_ipv4"`
	MTU              int               `json:"mtu"`
	TUNIPv6Gateway   string            `json:"tun_ipv6_gateway,omitempty"`
	DNSServers       []string          `json:"dns_servers"`
	ProxyEndpointIPs []string          `json:"proxy_endpoint_ips,omitempty"`
	IPv6Mode         IPv6Mode          `json:"ipv6_mode"`
	AdapterTimeout   time.Duration     `json:"-"`
	ActivationPlan   TUNActivationPlan `json:"activation_plan"`
	StageFn          func(TUNStage)    `json:"-"`
	FailurePoint     string            `json:"-"`
	CrashPoint       string            `json:"-"`
	CrashFn          func()            `json:"-"`
}

func (c *Config) withDefaults() {
	if c.AdapterName == "" {
		c.AdapterName = "Navo"
	}
	if c.TUNIPv4Gateway == "" {
		c.TUNIPv4Gateway = "172.19.0.2"
	}
	if c.TUNIPv4Address == "" {
		c.TUNIPv4Address = "172.19.0.1/30"
	}
	if c.TUNIPv4Peer == "" {
		c.TUNIPv4Peer = c.TUNIPv4Gateway
	}
	if c.TUNDNSIPv4 == "" {
		c.TUNDNSIPv4 = c.TUNIPv4Gateway
	}
	if c.MTU <= 0 {
		c.MTU = 1500
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
	if !strings.EqualFold(strings.TrimSpace(c.AdapterName), OwnedTUNAdapterName) {
		return fmt.Errorf("adapter %q is outside Navo ownership", c.AdapterName)
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
	if c.ActivationPlan.SessionID != "" {
		if err := c.ActivationPlan.validate(); err != nil {
			return err
		}
	}
	if c.FailurePoint != "" || c.CrashPoint != "" {
		allowed := map[string]bool{
			"after-endpoint-bypass": true, "after-first-split-route": true,
			"after-second-split-route": true, "after-nrpt": true, "after-ipv6": true,
			"during-dataplane": true,
		}
		if c.FailurePoint != "" && !allowed[c.FailurePoint] {
			return fmt.Errorf("invalid TUN failure point %q", c.FailurePoint)
		}
		if c.CrashPoint != "" && (!allowed[c.CrashPoint] || c.CrashFn == nil) {
			return fmt.Errorf("invalid TUN crash point %q", c.CrashPoint)
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
	RunOutput(ctx context.Context, command Command) (string, error)
}

// Platform provides Windows-specific privilege, Wintun, and adapter checks.
type Platform interface {
	Preflight(ctx context.Context, cfg Config) error
	WaitForAdapterReady(ctx context.Context, expectedName, expectedAddress string, expectedMTU int, timeout time.Duration) (AdapterSnapshot, error)
	VerifyControlPlane(ctx context.Context, plan TUNActivationPlan, adapter AdapterSnapshot) error
}
