// Package compiler transforms Navo domain models into sing-box configuration JSON.
// It is the bridge between the user's intent (rules, outbounds, DNS settings)
// and the proxy core's deployment format.
package compiler

import "time"

// ── Domain Models ──

// OutboundType enumerates the supported outbound protocol types.
type OutboundType string

const (
	OutboundDirect      OutboundType = "direct"
	OutboundSOCKS       OutboundType = "socks"
	OutboundHTTP        OutboundType = "http"
	OutboundShadowsocks OutboundType = "shadowsocks"
	OutboundVMess       OutboundType = "vmess"
	OutboundTrojan      OutboundType = "trojan"
	OutboundVLESS       OutboundType = "vless"
	OutboundHysteria2   OutboundType = "hysteria2"
	OutboundTUIC        OutboundType = "tuic"
	OutboundWireGuard   OutboundType = "wireguard"
)

// Outbound represents a network exit point.
type Outbound struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Type              OutboundType `json:"type"`
	Server            string       `json:"server"`
	Port              int          `json:"port"`
	Enabled           bool         `json:"enabled"`
	ProviderID        string       `json:"provider_id,omitempty"`
	Country           string       `json:"country,omitempty"`
	Username          string       `json:"username,omitempty"`
	Password          string       `json:"password,omitempty"`
	Method            string       `json:"method,omitempty"`      // shadowsocks method
	Password2         string       `json:"password2,omitempty"`   // VLESS flow
	UUID              string       `json:"uuid,omitempty"`        // VMess/VLESS UUID
	Security          string       `json:"security,omitempty"`    // Trojan/TUIC
	Fingerprint       string       `json:"fingerprint,omitempty"` // TLS fingerprint
	Network           string       `json:"network,omitempty"`     // tcp/udp/ws/grpc
	TLS               bool         `json:"tls,omitempty"`
	SNI               string       `json:"sni,omitempty"`  // TLS SNI
	ALPN              []string     `json:"alpn,omitempty"` // TLS ALPN
	SkipCertVerify    bool         `json:"skip_cert_verify,omitempty"`
	TransportPath     string       `json:"transport_path,omitempty"`
	TransportHost     string       `json:"transport_host,omitempty"`
	ServiceName       string       `json:"service_name,omitempty"`
	RealityPublicKey  string       `json:"reality_public_key,omitempty"`
	RealityShortID    string       `json:"reality_short_id,omitempty"`
	ObfsType          string       `json:"obfs_type,omitempty"`
	ObfsPassword      string       `json:"obfs_password,omitempty"`
	CongestionControl string       `json:"congestion_control,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// RuleType determines how a routing rule matches traffic.
type RuleType string

const (
	RuleDomain        RuleType = "domain"
	RuleDomainSuffix  RuleType = "domain_suffix"
	RuleDomainKeyword RuleType = "domain_keyword"
	RuleDomainRegex   RuleType = "domain_regex"
	RuleIP            RuleType = "ip_cidr"
	RuleProcess       RuleType = "process_name"
	RuleGeosite       RuleType = "geosite"
	RuleGeoip         RuleType = "geoip"
	RulePort          RuleType = "port"
	RuleProtocol      RuleType = "protocol"
)

// RoutingRule defines a traffic routing policy.
type RoutingRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Priority    int       `json:"priority"`
	RuleType    RuleType  `json:"rule_type"`
	Values      []string  `json:"values"`       // domain names, IP ranges, process names
	OutboundID  string    `json:"outbound_id"`  // target outbound
	OutboundTag string    `json:"outbound_tag"` // resolved at compile time
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DNSStrategy defines how DNS queries are handled.
type DNSStrategy string

const (
	DNSStrategyPreferIPv4 DNSStrategy = "prefer_ipv4"
	DNSStrategyPreferIPv6 DNSStrategy = "prefer_ipv6"
	DNSStrategyIPv4Only   DNSStrategy = "ipv4_only"
	DNSStrategyIPv6Only   DNSStrategy = "ipv6_only"
)

// DNSConfig represents DNS resolution configuration.
type DNSConfig struct {
	Enabled          bool        `json:"enabled"`
	IndependentCache bool        `json:"independent_cache"`
	Strategy         DNSStrategy `json:"strategy"`
	Servers          []DNSServer `json:"servers"`
	Rules            []DNSRule   `json:"rules,omitempty"`
	Final            string      `json:"final,omitempty"` // fallback server tag
}

// DNSServer represents a DNS resolver.
type DNSServer struct {
	Type            string `json:"type,omitempty"`
	Tag             string `json:"tag"`
	Address         string `json:"address"` // resolver address, e.g. "tls://1.1.1.1"
	Server          string `json:"server,omitempty"`
	ServerPort      int    `json:"server_port,omitempty"`
	Path            string `json:"path,omitempty"`
	TLSServerName   string `json:"tls_server_name,omitempty"`
	AddressResolver string `json:"address_resolver,omitempty"`
	AddressStrategy string `json:"address_strategy,omitempty"`
	Detour          string `json:"detour,omitempty"` // outbound tag to use
}

// DNSRule defines which DNS server handles which domains.
type DNSRule struct {
	RuleType     RuleType `json:"rule_type"`
	Values       []string `json:"values"`
	Server       string   `json:"server"` // target DNS server tag
	DisableCache bool     `json:"disable_cache,omitempty"`
}

// TUNConfig represents TUN mode settings.
type TUNConfig struct {
	Enabled           bool     `json:"enabled"`
	InterfaceName     string   `json:"interface_name"`
	MTU               int      `json:"mtu"`
	Address           []string `json:"address"` // IPv4/IPv6 addresses
	AutoRoute         bool     `json:"auto_route"`
	StrictRoute       bool     `json:"strict_route"`
	IPv6Enabled       bool     `json:"ipv6_enabled"`
	OutboundInterface string   `json:"outbound_interface,omitempty"`
	IncludePackage    []string `json:"include_package,omitempty"`
	ExcludePackage    []string `json:"exclude_package,omitempty"`
}

// InboundConfig represents local listening configuration.
type InboundConfig struct {
	Type       string `json:"type"` // "mixed", "socks", "http", "tun"
	Tag        string `json:"tag"`
	Listen     string `json:"listen"` // bind address
	ListenPort int    `json:"listen_port"`
	Sniff      bool   `json:"sniff,omitempty"`
}

// LogConfig represents logging settings.
type LogConfig struct {
	Disabled  bool   `json:"disabled"`
	Level     string `json:"level"` // "trace", "debug", "info", "warn", "error", "fatal", "panic"
	Output    string `json:"output,omitempty"`
	Timestamp bool   `json:"timestamp"`
}

type ControllerConfig struct {
	Listen string `json:"listen"`
	Port   int    `json:"port"`
	Secret string `json:"secret"`
}

// ── Config Model ──

// Config is the complete Navo internal configuration model.
// It serves as the source of truth, from which sing-box config is compiled.
type Config struct {
	SchemaVersion int             `json:"schema_version"`
	Log           LogConfig       `json:"log"`
	Inbounds      []InboundConfig `json:"inbounds"`
	Outbounds     []Outbound      `json:"outbounds"`
	RoutingRules  []RoutingRule   `json:"routing_rules"`
	DNS           *DNSConfig      `json:"dns,omitempty"`
	TUN           *TUNConfig      `json:"tun,omitempty"`
	FinalOutbound string          `json:"final_outbound,omitempty"`
	// OutboundInterface freezes core-originated DNS and direct sockets to the
	// physical egress selected by Windows instead of a virtual adapter.
	OutboundInterface string            `json:"outbound_interface,omitempty"`
	Controller        *ControllerConfig `json:"controller,omitempty"`
}

// ── Config Revision ──

// RevisionStatus tracks the lifecycle of a compiled config.
type RevisionStatus string

const (
	RevisionPending  RevisionStatus = "pending"
	RevisionActive   RevisionStatus = "active"
	RevisionRollback RevisionStatus = "rollback"
	RevisionRejected RevisionStatus = "rejected"
)

// Revision is a versioned compiled configuration.
type Revision struct {
	ID           string         `json:"id"`
	Version      int            `json:"version"`
	Status       RevisionStatus `json:"status"`
	ConfigHash   string         `json:"config_hash"`
	ConfigPath   string         `json:"config_path"`
	CreatedAt    time.Time      `json:"created_at"`
	ActivatedAt  *time.Time     `json:"activated_at,omitempty"`
	RollbackFrom int            `json:"rollback_from,omitempty"` // version this rolls back from
}
