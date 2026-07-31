package compiler

import (
	"fmt"
	"net"
	"strings"
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

	// Validate inbounds
	for i, in := range cfg.Inbounds {
		prefix := fmt.Sprintf("inbounds[%d]", i)
		validateInbound(vr, prefix, &in)
	}

	// Validate outbounds
	outboundIDs := make(map[string]bool)
	for i, o := range cfg.Outbounds {
		prefix := fmt.Sprintf("outbounds[%d](%s)", i, o.ID)
		validateOutbound(vr, prefix, &o)
		outboundIDs[o.ID] = true
	}

	// Validate routing rules
	for i, r := range cfg.RoutingRules {
		prefix := fmt.Sprintf("routing_rules[%d](%s)", i, r.ID)
		validateRoutingRule(vr, prefix, &r, outboundIDs)
	}

	// Validate DNS
	if cfg.DNS != nil && cfg.DNS.Enabled {
		validateDNS(vr, cfg.DNS)
	}

	return vr
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
		if o.UUID == "" {
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
		if o.UUID == "" {
			vr.add(prefix+".uuid", "UUID is required for TUIC", "")
		}
		if o.Password == "" {
			vr.add(prefix+".password", "password is required for TUIC", "")
		}
	case OutboundWireGuard:
		// WireGuard-specific validation is handled by sing-box preflight.
	default:
		vr.add(prefix+".type", "unsupported outbound type", string(o.Type))
	}
}

func validateRoutingRule(vr *ValidationResult, prefix string, r *RoutingRule, outboundIDs map[string]bool) {
	if r.ID == "" {
		vr.add(prefix+".id", "ID is required", "")
	}
	if len(r.Values) == 0 {
		vr.add(prefix+".values", "at least one value is required", "")
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

	for i, s := range dns.Servers {
		prefix := fmt.Sprintf("dns.servers[%d](%s)", i, s.Tag)
		if s.Tag == "" {
			vr.add(prefix+".tag", "tag is required", "")
		}
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
			if s.ServerPort < 0 || s.ServerPort > 65535 {
				vr.add(prefix+".server_port", "port must be between 1 and 65535", fmt.Sprint(s.ServerPort))
			}
		default:
			vr.add(prefix+".type", "unsupported DNS server type", s.Type)
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
	// Simple domain check
	if strings.Contains(addr, ".") && !strings.Contains(addr, " ") {
		return true
	}
	return false
}
