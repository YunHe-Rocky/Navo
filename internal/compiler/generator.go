package compiler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// singBoxConfig is the intermediate representation of a sing-box configuration file.
type singBoxConfig struct {
	Log          *singBoxLog          `json:"log,omitempty"`
	Inbounds     []singBoxInbound     `json:"inbounds,omitempty"`
	Outbounds    []singBoxOutbound    `json:"outbounds,omitempty"`
	Route        *singBoxRoute        `json:"route,omitempty"`
	DNS          *singBoxDNS          `json:"dns,omitempty"`
	Experimental *singBoxExperimental `json:"experimental,omitempty"`
}

type singBoxExperimental struct {
	ClashAPI *singBoxClashAPI `json:"clash_api,omitempty"`
}

type singBoxClashAPI struct {
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret,omitempty"`
}

type singBoxLog struct {
	Disabled  bool   `json:"disabled"`
	Level     string `json:"level"`
	Output    string `json:"output,omitempty"`
	Timestamp bool   `json:"timestamp"`
}

type singBoxInbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	// TUN-specific
	MTU            int      `json:"mtu,omitempty"`
	InterfaceName  string   `json:"interface_name,omitempty"`
	Address        []string `json:"address,omitempty"`
	AutoRoute      bool     `json:"auto_route,omitempty"`
	StrictRoute    bool     `json:"strict_route,omitempty"`
	IncludePackage []string `json:"include_package,omitempty"`
	ExcludePackage []string `json:"exclude_package,omitempty"`
	Stack          string   `json:"stack,omitempty"`
}

type singBoxOutbound struct {
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	Server string `json:"server,omitempty"`
	Port   int    `json:"server_port,omitempty"`

	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"`

	UUID     string `json:"uuid,omitempty"`
	Security string `json:"security,omitempty"`
	Flow     string `json:"flow,omitempty"`

	TLS *singBoxTLS `json:"tls,omitempty"`

	Transport *singBoxTransport `json:"transport,omitempty"`

	Multiplex *singBoxMultiplex `json:"multiplex,omitempty"`

	OverrideAddress string `json:"override_address,omitempty"`
	OverridePort    int    `json:"override_port,omitempty"`

	Obfs              *singBoxObfs `json:"obfs,omitempty"`
	CongestionControl string       `json:"congestion_control,omitempty"`
}

type singBoxObfs struct {
	Type     string `json:"type"`
	Password string `json:"password"`
}

type singBoxTLS struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"server_name,omitempty"`
	Insecure   bool            `json:"insecure,omitempty"`
	ALPN       []string        `json:"alpn,omitempty"`
	UTLS       *singBoxUTLS    `json:"utls,omitempty"`
	Reality    *singBoxReality `json:"reality,omitempty"`
}

type singBoxUTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type singBoxReality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id,omitempty"`
}

type singBoxTransport struct {
	Type        string                 `json:"type"`
	Path        string                 `json:"path,omitempty"`
	Host        interface{}            `json:"host,omitempty"`
	Headers     map[string]interface{} `json:"headers,omitempty"`
	ServiceName string                 `json:"service_name,omitempty"`
}

type singBoxMultiplex struct {
	Enabled        bool   `json:"enabled"`
	Protocol       string `json:"protocol,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
}

type singBoxRoute struct {
	Rules                 []singBoxRouteRule `json:"rules"`
	AutoDetectInterface   bool               `json:"auto_detect_interface"`
	DefaultDomainResolver string             `json:"default_domain_resolver,omitempty"`
	Final                 string             `json:"final,omitempty"`
}

type singBoxRouteRule struct {
	Inbound       []string `json:"inbound,omitempty"`
	Domain        []string `json:"domain,omitempty"`
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	DomainRegex   []string `json:"domain_regex,omitempty"`
	IPCIDR        []string `json:"ip_cidr,omitempty"`
	ProcessName   []string `json:"process_name,omitempty"`
	Geosite       []string `json:"geosite,omitempty"`
	Geoip         []string `json:"geoip,omitempty"`
	Port          []int    `json:"port,omitempty"`
	Protocol      []string `json:"protocol,omitempty"`
	Network       []string `json:"network,omitempty"`
	Action        string   `json:"action"`
	Outbound      string   `json:"outbound,omitempty"`
}

type singBoxDNS struct {
	Servers          []singBoxDNSServer `json:"servers"`
	Rules            []singBoxDNSRule   `json:"rules,omitempty"`
	IndependentCache bool               `json:"independent_cache,omitempty"`
	Final            string             `json:"final,omitempty"`
	Strategy         string             `json:"strategy,omitempty"`
	DisableCache     bool               `json:"disable_cache,omitempty"`
}

type singBoxDNSServer struct {
	Type            string `json:"type,omitempty"`
	Tag             string `json:"tag"`
	Address         string `json:"address,omitempty"`
	Server          string `json:"server,omitempty"`
	ServerPort      int    `json:"server_port,omitempty"`
	AddressResolver string `json:"address_resolver,omitempty"`
	AddressStrategy string `json:"address_strategy,omitempty"`
	Detour          string `json:"detour,omitempty"`
}

type singBoxDNSRule struct {
	Domain        []string `json:"domain,omitempty"`
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	DomainRegex   []string `json:"domain_regex,omitempty"`
	Geosite       []string `json:"geosite,omitempty"`
	Server        string   `json:"server"`
	DisableCache  bool     `json:"disable_cache,omitempty"`
}

// Generate compiles a Config model into sing-box JSON.
func Generate(cfg *Config) ([]byte, error) {
	sb := &singBoxConfig{}

	sb.Log = &singBoxLog{
		Disabled:  cfg.Log.Disabled,
		Level:     cfg.Log.Level,
		Output:    cfg.Log.Output,
		Timestamp: cfg.Log.Timestamp,
	}
	if cfg.Controller != nil {
		sb.Experimental = &singBoxExperimental{
			ClashAPI: &singBoxClashAPI{
				ExternalController: fmt.Sprintf("%s:%d", cfg.Controller.Listen, cfg.Controller.Port),
				Secret:             cfg.Controller.Secret,
			},
		}
	}

	for _, in := range cfg.Inbounds {
		sbIn := singBoxInbound{
			Type:       in.Type,
			Tag:        in.Tag,
			Listen:     in.Listen,
			ListenPort: in.ListenPort,
		}
		if in.Type == "tun" && cfg.TUN != nil {
			sbIn.MTU = cfg.TUN.MTU
			sbIn.InterfaceName = cfg.TUN.InterfaceName
			sbIn.Address = cfg.TUN.Address
			sbIn.AutoRoute = cfg.TUN.AutoRoute
			sbIn.StrictRoute = cfg.TUN.StrictRoute
			sbIn.IncludePackage = cfg.TUN.IncludePackage
			sbIn.ExcludePackage = cfg.TUN.ExcludePackage
			if cfg.TUN.IPv6Enabled {
				sbIn.Stack = "mixed"
			} else {
				sbIn.Stack = "system"
			}
		}
		sb.Inbounds = append(sb.Inbounds, sbIn)
	}

	for _, o := range cfg.Outbounds {
		sbOut := buildSingBoxOutbound(&o)
		sb.Outbounds = append(sb.Outbounds, sbOut)
	}

	sb.Route = &singBoxRoute{
		AutoDetectInterface: true,
		Final:               cfg.FinalOutbound,
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Sniff {
			sb.Route.Rules = append(
				sb.Route.Rules,
				singBoxRouteRule{Action: "sniff"},
			)
			break
		}
	}
	tunEnabled := cfg.TUN != nil && cfg.TUN.Enabled
	if tunEnabled {
		sb.Route.Rules = append(
			sb.Route.Rules,
			singBoxRouteRule{
				Protocol: []string{"dns"},
				Action:   "hijack-dns",
			},
		)
		if directTag := directOutboundTag(cfg.Outbounds); directTag != "" {
			sb.Route.Rules = append(
				sb.Route.Rules,
				singBoxRouteRule{
					Network:  []string{"icmp"},
					Action:   "route",
					Outbound: directTag,
				},
			)
		}
	}

	for _, r := range cfg.RoutingRules {
		if !r.Enabled {
			continue
		}
		sbRule := singBoxRouteRule{
			Action:   "route",
			Outbound: r.OutboundTag,
		}
		switch r.RuleType {
		case RuleDomain:
			sbRule.Domain = r.Values
		case RuleDomainSuffix:
			sbRule.DomainSuffix = r.Values
		case RuleDomainKeyword:
			sbRule.DomainKeyword = r.Values
		case RuleDomainRegex:
			sbRule.DomainRegex = r.Values
		case RuleIP:
			sbRule.IPCIDR = r.Values
		case RuleProcess:
			sbRule.ProcessName = r.Values
		case RuleGeosite:
			sbRule.Geosite = r.Values
		case RuleGeoip:
			sbRule.Geoip = r.Values
		case RulePort:
			var ports []int
			for _, v := range r.Values {
				var p int
				fmt.Sscanf(v, "%d", &p)
				ports = append(ports, p)
			}
			sbRule.Port = ports
		}
		sb.Route.Rules = append(sb.Route.Rules, sbRule)
	}

	effectiveDNS := cfg.DNS
	if tunEnabled && (effectiveDNS == nil || !effectiveDNS.Enabled || len(effectiveDNS.Servers) == 0) {
		effectiveDNS = defaultTUNDNSConfig()
	}
	if effectiveDNS != nil && effectiveDNS.Enabled {
		sb.DNS = buildSingBoxDNS(effectiveDNS)
		if effectiveDNS.Final != "" {
			sb.Route.DefaultDomainResolver = effectiveDNS.Final
		} else if len(effectiveDNS.Servers) > 0 {
			sb.Route.DefaultDomainResolver = effectiveDNS.Servers[0].Tag
		}
	}

	return marshalJSON(sb)
}

func directOutboundTag(outbounds []Outbound) string {
	for _, outbound := range outbounds {
		if outbound.Enabled && outbound.Type == OutboundDirect {
			return outbound.ID
		}
	}
	return ""
}

func defaultTUNDNSConfig() *DNSConfig {
	return &DNSConfig{
		Enabled:  true,
		Strategy: DNSStrategyIPv4Only,
		Servers: []DNSServer{{
			Type:       "udp",
			Tag:        "dns-direct",
			Server:     "223.5.5.5",
			ServerPort: 53,
		}},
		Final: "dns-direct",
	}
}

func buildSingBoxOutbound(o *Outbound) singBoxOutbound {
	sb := singBoxOutbound{
		Tag:  o.ID,
		Type: mapOutboundType(o.Type),
	}

	switch o.Type {
	case OutboundDirect:
	case OutboundSOCKS, OutboundHTTP:
		sb.Server = o.Server
		sb.Port = o.Port
		if o.Username != "" {
			sb.Username = o.Username
			sb.Password = o.Password
		}
	case OutboundShadowsocks:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.Method = o.Method
		sb.Password = o.Password
	case OutboundVMess:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.UUID = o.UUID
		sb.Security = o.Security
	case OutboundVLESS:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.UUID = o.UUID
		sb.Flow = o.Password2
	case OutboundTrojan:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.Password = o.Password
	case OutboundHysteria2:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.Password = o.Password
		if o.ObfsType != "" {
			sb.Obfs = &singBoxObfs{
				Type: o.ObfsType, Password: o.ObfsPassword,
			}
		}
	case OutboundTUIC:
		sb.Server = o.Server
		sb.Port = o.Port
		sb.UUID = o.UUID
		sb.Password = o.Password
		sb.CongestionControl = o.CongestionControl
	case OutboundWireGuard:
		sb.Server = o.Server
		sb.Port = o.Port
	}

	if o.Network != "" && o.Network != "tcp" {
		transport := &singBoxTransport{
			Type:        o.Network,
			Path:        o.TransportPath,
			ServiceName: o.ServiceName,
		}
		switch o.Network {
		case "ws":
			if o.TransportHost != "" {
				transport.Headers = map[string]interface{}{"Host": o.TransportHost}
			}
		case "http":
			if o.TransportHost != "" {
				transport.Host = []string{o.TransportHost}
			}
		case "httpupgrade":
			transport.Host = o.TransportHost
		}
		sb.Transport = transport
	}

	if o.TLS || o.SNI != "" || o.RealityPublicKey != "" {
		sb.TLS = &singBoxTLS{
			Enabled:    true,
			ServerName: o.SNI,
			Insecure:   o.SkipCertVerify,
			ALPN:       o.ALPN,
		}
		if o.Fingerprint != "" {
			sb.TLS.UTLS = &singBoxUTLS{
				Enabled: true, Fingerprint: o.Fingerprint,
			}
		}
		if o.RealityPublicKey != "" {
			sb.TLS.Reality = &singBoxReality{
				Enabled: true, PublicKey: o.RealityPublicKey,
				ShortID: o.RealityShortID,
			}
		}
	}

	return sb
}

func buildSingBoxDNS(dns *DNSConfig) *singBoxDNS {
	sb := &singBoxDNS{
		IndependentCache: dns.IndependentCache,
		Final:            dns.Final,
		Strategy:         string(dns.Strategy),
	}

	for _, s := range dns.Servers {
		sb.Servers = append(sb.Servers, singBoxDNSServer{
			Type:            s.Type,
			Tag:             s.Tag,
			Address:         s.Address,
			Server:          s.Server,
			ServerPort:      s.ServerPort,
			AddressResolver: s.AddressResolver,
			AddressStrategy: s.AddressStrategy,
			Detour:          s.Detour,
		})
	}

	for _, r := range dns.Rules {
		sbRule := singBoxDNSRule{
			Server:       r.Server,
			DisableCache: r.DisableCache,
		}
		switch r.RuleType {
		case RuleDomain:
			sbRule.Domain = r.Values
		case RuleDomainSuffix:
			sbRule.DomainSuffix = r.Values
		case RuleDomainKeyword:
			sbRule.DomainKeyword = r.Values
		case RuleDomainRegex:
			sbRule.DomainRegex = r.Values
		case RuleGeosite:
			sbRule.Geosite = r.Values
		}
		sb.Rules = append(sb.Rules, sbRule)
	}

	return sb
}

func mapOutboundType(t OutboundType) string {
	switch t {
	case OutboundSOCKS:
		return "socks"
	case OutboundHTTP:
		return "http"
	case OutboundShadowsocks:
		return "shadowsocks"
	case OutboundVMess:
		return "vmess"
	case OutboundTrojan:
		return "trojan"
	case OutboundVLESS:
		return "vless"
	case OutboundHysteria2:
		return "hysteria2"
	case OutboundTUIC:
		return "tuic"
	case OutboundWireGuard:
		return "wireguard"
	default:
		return "direct"
	}
}

func marshalJSON(sb *singBoxConfig) ([]byte, error) {
	raw, err := json.MarshalIndent(sb, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}
	return raw, nil
}

func GenerateString(cfg *Config) (string, error) {
	data, err := Generate(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ResolveOutboundTags(cfg *Config) error {
	outboundMap := make(map[string]string, len(cfg.Outbounds))
	for _, o := range cfg.Outbounds {
		outboundMap[o.ID] = o.ID
		if o.Name != "" {
			tag := strings.ReplaceAll(strings.ToLower(o.Name), " ", "-")
			outboundMap[o.Name] = tag
		}
	}

	for i, r := range cfg.RoutingRules {
		if tag, ok := outboundMap[r.OutboundID]; ok {
			cfg.RoutingRules[i].OutboundTag = tag
		} else if r.OutboundID == "direct" {
			cfg.RoutingRules[i].OutboundTag = "direct"
		} else if r.OutboundID == "bypass" || r.OutboundID == "block" {
			cfg.RoutingRules[i].OutboundTag = r.OutboundID
		} else {
			cfg.RoutingRules[i].OutboundTag = r.OutboundID
		}
	}
	return nil
}
