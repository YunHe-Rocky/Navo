package compiler

import (
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// ValidationError represents a single config validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (got: %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult holds all validation errors found.
type ValidationResult struct {
	Valid  bool               `json:"valid"`
	Errors []*ValidationError `json:"errors,omitempty"`
}

func (vr *ValidationResult) add(field, msg, value string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:   field,
		Message: msg,
		Value:   value,
	})
}

// Validate checks a Config model for semantic correctness.
func Validate(cfg *Config) *ValidationResult {
	vr := &ValidationResult{Valid: true}

	// Validate schema version
	if cfg.SchemaVersion != 1 {
		vr.add("schema_version", "unsupported schema version", fmt.Sprintf("%d", cfg.SchemaVersion))
	}

	inboundTags := make(map[string]bool)
	for i, in := range cfg.Inbounds {
		prefix := fmt.Sprintf("inbounds[%d]", i)
		validateInbound(vr, prefix, &in)
		if in.Tag != "" && inboundTags[in.Tag] {
			vr.add(prefix+".tag", "tag must be unique", in.Tag)
		}
		inboundTags[in.Tag] = true
	}

	// Validate outbounds
	outboundIDs := make(map[string]bool)
	for i, o := range cfg.Outbounds {
		prefix := fmt.Sprintf("outbounds[%d](%s)", i, o.ID)
		validateOutbound(vr, prefix, &o)
		if o.ID != "" && outboundIDs[o.ID] {
			vr.add(prefix+".id", "ID must be unique", o.ID)
		}
		outboundIDs[o.ID] = true
	}
	if cfg.FinalOutbound != "" && cfg.FinalOutbound != "direct" && cfg.FinalOutbound != "block" && !outboundIDs[cfg.FinalOutbound] {
		vr.add("final_outbound", "referenced outbound does not exist", cfg.FinalOutbound)
	}

	// Validate routing rules
	ruleIDs := make(map[string]bool)
	for i, r := range cfg.RoutingRules {
		prefix := fmt.Sprintf("routing_rules[%d](%s)", i, r.ID)
		validateRoutingRule(vr, prefix, &r, outboundIDs)
		if r.ID != "" && ruleIDs[r.ID] {
			vr.add(prefix+".id", "ID must be unique", r.ID)
		}
		ruleIDs[r.ID] = true
	}

	// Validate DNS
	if cfg.DNS != nil && cfg.DNS.Enabled {
		validateDNS(vr, cfg.DNS)
	}
	if cfg.TUN != nil && cfg.TUN.Enabled {
		validateTUN(vr, cfg.TUN)
	}

	return vr
}

func validateTUN(vr *ValidationResult, tun *TUNConfig) {
	if strings.TrimSpace(tun.InterfaceName) == "" {
		vr.add("tun.interface_name", "interface name is required", "")
	}
	if tun.MTU < 576 || tun.MTU > 9000 {
		vr.add("tun.mtu", "MTU must be 576-9000", strconv.Itoa(tun.MTU))
	}
	if len(tun.Address) == 0 {
		vr.add("tun.address", "at least one interface address is required", "")
	}
	for index, address := range tun.Address {
		ip, _, err := net.ParseCIDR(address)
		if err != nil {
			vr.add(fmt.Sprintf("tun.address[%d]", index), "invalid CIDR", address)
			continue
		}
		if ip.To4() == nil && !tun.IPv6Enabled {
			vr.add(fmt.Sprintf("tun.address[%d]", index), "IPv6 address requires IPv6 mode", address)
		}
	}
}

// ValidateOutbound validates one parser/import result before it can enter a
// persisted runtime candidate.
func ValidateOutbound(outbound *Outbound) *ValidationResult {
	result := &ValidationResult{Valid: true}
	if outbound == nil {
		result.add("outbound", "outbound is required", "")
		return result
	}
	validateOutbound(result, "outbound", outbound)
	return result
}

func validateInbound(vr *ValidationResult, prefix string, in *InboundConfig) {
	if in.Type == "" {
		vr.add(prefix+".type", "type is required", "")
		return
	}
	validTypes := map[string]bool{
		"mixed": true, "socks": true, "http": true,
		"tun": true, "redirect": true, "tproxy": true,
	}
	if !validTypes[in.Type] {
		vr.add(prefix+".type", "invalid inbound type", in.Type)
	}
	if in.Type != "tun" {
		if in.ListenPort <= 0 || in.ListenPort > 65535 {
			vr.add(prefix+".listen_port", "port must be 1-65535", fmt.Sprintf("%d", in.ListenPort))
		}
		if in.Listen == "" {
			vr.add(prefix+".listen", "listen address is required", "")
		}
	}
	if in.Tag == "" {
		vr.add(prefix+".tag", "tag is required", "")
	}
}

func validateOutbound(vr *ValidationResult, prefix string, o *Outbound) {
	if o.ID == "" {
		vr.add(prefix+".id", "ID is required", "")
	}
	if o.Type == "" {
		vr.add(prefix+".type", "type is required", "")
		return
	}

	// Direct needs no further validation
	if o.Type == OutboundDirect {
		return
	}

	if o.Server == "" {
		vr.add(prefix+".server", "server address is required", "")
	} else if !isValidAddress(o.Server) {
		vr.add(prefix+".server", "invalid server address", o.Server)
	}

	if o.Port <= 0 || o.Port > 65535 {
		vr.add(prefix+".port", "port must be 1-65535", fmt.Sprintf("%d", o.Port))
	}

	// Protocol-specific validation
	switch o.Type {
	case OutboundSOCKS, OutboundHTTP:
		// Username/password are optional
	case OutboundShadowsocks:
		if o.Method == "" {
			vr.add(prefix+".method", "encryption method is required for shadowsocks", "")
		}
		if o.Password == "" {
			vr.add(prefix+".password", "password is required for shadowsocks", "")
		}
	case OutboundVMess, OutboundVLESS:
		if _, err := uuid.Parse(o.UUID); err != nil {
			vr.add(prefix+".uuid", "UUID is required", "")
		}
	case OutboundTrojan:
		if o.Password == "" {
			vr.add(prefix+".password", "password is required for trojan", "")
		}
	case OutboundHysteria2:
		if o.Password == "" {
			vr.add(prefix+".password", "password is required for hysteria2", "")
		}
	case OutboundTUIC:
		if _, err := uuid.Parse(o.UUID); err != nil {
			vr.add(prefix+".uuid", "UUID is required for TUIC", "")
		}
		if o.Password == "" {
			vr.add(prefix+".password", "password is required for TUIC", "")
		}
	case OutboundWireGuard:
		vr.add(prefix+".type", "WireGuard is unsupported until the canonical model preserves all key, address, peer and route fields", string(o.Type))
	default:
		vr.add(prefix+".type", "unsupported outbound type", string(o.Type))
	}
	validateTransport(vr, prefix, o)
}

func validateRoutingRule(vr *ValidationResult, prefix string, r *RoutingRule, outboundIDs map[string]bool) {
	if r.ID == "" {
		vr.add(prefix+".id", "ID is required", "")
	}
	if len(r.Values) == 0 {
		vr.add(prefix+".values", "at least one value is required", "")
	}
	for index, value := range r.Values {
		validateRuleValue(vr, fmt.Sprintf("%s.values[%d]", prefix, index), r.RuleType, value)
	}

	// Check if outbound exists (direct is always available)
	if r.OutboundID != "direct" && r.OutboundID != "bypass" && r.OutboundID != "block" {
		if !outboundIDs[r.OutboundID] {
			vr.add(prefix+".outbound_id", "referenced outbound does not exist", r.OutboundID)
		}
	}
}

func validateDNS(vr *ValidationResult, dns *DNSConfig) {
	if len(dns.Servers) == 0 && dns.Final == "" {
		vr.add("dns.servers", "at least one DNS server or final is required", "")
	}

	tags := make(map[string]bool)
	for i, s := range dns.Servers {
		prefix := fmt.Sprintf("dns.servers[%d](%s)", i, s.Tag)
		if s.Tag == "" {
			vr.add(prefix+".tag", "tag is required", "")
		} else if tags[s.Tag] {
			vr.add(prefix+".tag", "tag must be unique", s.Tag)
		}
		tags[s.Tag] = true
		switch s.Type {
		case "":
			if s.Address == "" {
				vr.add(prefix+".address", "address is required", "")
			}
		case "local":
		case "udp", "tcp", "tls", "quic", "https", "h3":
			if !isValidAddress(s.Server) {
				vr.add(prefix+".server", "valid server is required", s.Server)
			}
			if s.ServerPort <= 0 || s.ServerPort > 65535 {
				vr.add(prefix+".server_port", "port must be 1-65535", fmt.Sprint(s.ServerPort))
			}
		default:
			vr.add(prefix+".type", "unsupported DNS server type", s.Type)
		}
	}
	if dns.Final != "" && !tags[dns.Final] {
		vr.add("dns.final", "referenced DNS server does not exist", dns.Final)
	}
	for i, rule := range dns.Rules {
		if !tags[rule.Server] {
			vr.add(fmt.Sprintf("dns.rules[%d].server", i), "referenced DNS server does not exist", rule.Server)
		}
	}
}

func isValidAddress(addr string) bool {
	// Could be IP, domain, or IP:port
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	// Try parsing as IP
	if net.ParseIP(addr) != nil {
		return true
	}
	return validDomain(addr)
}

func validDomain(value string) bool {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".")
	if value == "" || len(value) > 253 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func validateTransport(vr *ValidationResult, prefix string, outbound *Outbound) {
	network := strings.ToLower(strings.TrimSpace(outbound.Network))
	switch network {
	case "", "tcp", "udp":
	case "ws":
		if !strings.HasPrefix(outbound.TransportPath, "/") {
			vr.add(prefix+".transport_path", "WebSocket path must start with /", outbound.TransportPath)
		}
		if outbound.TransportHost != "" && !validDomain(outbound.TransportHost) {
			vr.add(prefix+".transport_host", "invalid WebSocket host", outbound.TransportHost)
		}
	case "grpc":
		if strings.TrimSpace(outbound.ServiceName) == "" {
			vr.add(prefix+".service_name", "gRPC service name is required", "")
		}
	default:
		vr.add(prefix+".network", "unsupported transport", outbound.Network)
	}
	if outbound.TLS && outbound.SNI != "" && !validDomain(outbound.SNI) {
		vr.add(prefix+".sni", "invalid TLS server name", outbound.SNI)
	}
	if outbound.RealityPublicKey != "" {
		if !validDomain(outbound.SNI) {
			vr.add(prefix+".sni", "Reality requires a valid server name", outbound.SNI)
		}
		shortID := strings.TrimSpace(outbound.RealityShortID)
		if len(shortID) == 0 || len(shortID) > 16 || len(shortID)%2 != 0 {
			vr.add(prefix+".reality_short_id", "Reality short ID must be 2-16 hexadecimal characters", shortID)
		} else if _, err := hex.DecodeString(shortID); err != nil {
			vr.add(prefix+".reality_short_id", "Reality short ID must be hexadecimal", shortID)
		}
	}
}

func validateRuleValue(vr *ValidationResult, field string, ruleType RuleType, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		vr.add(field, "value must not be empty", "")
		return
	}
	switch ruleType {
	case RuleIP:
		if _, _, err := net.ParseCIDR(value); err != nil {
			vr.add(field, "invalid IP CIDR", value)
		}
	case RulePort:
		parts := strings.Split(value, "-")
		if len(parts) > 2 {
			vr.add(field, "invalid port range", value)
			return
		}
		start, err := strconv.Atoi(parts[0])
		end := start
		if len(parts) == 2 {
			end, err = strconv.Atoi(parts[1])
		}
		if err != nil || start < 1 || end > 65535 || start > end {
			vr.add(field, "port or range must be within 1-65535", value)
		}
	case RuleDomain, RuleDomainSuffix:
		if !validDomain(strings.TrimPrefix(value, ".")) {
			vr.add(field, "invalid domain", value)
		}
	case RuleDomainRegex:
		if _, err := regexp.Compile(value); err != nil {
			vr.add(field, "invalid domain regular expression", "")
		}
	case RuleProcess:
		if strings.ContainsAny(value, `/\\`) || strings.Contains(value, "..") {
			vr.add(field, "process name must not contain path components", value)
		}
	case RuleDomainKeyword, RuleGeosite, RuleGeoip, RuleProtocol:
	default:
		vr.add(field, "unsupported rule type", string(ruleType))
	}
}
