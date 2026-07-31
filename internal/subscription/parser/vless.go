package parser

import (
	"fmt"
	"net/url"
	"strings"

	"navo/internal/compiler"
)

// VLESSParser handles VLESS (vless://) subscriptions.
type VLESSParser struct{}

func NewVLESSParser() *VLESSParser { return &VLESSParser{} }

func (p *VLESSParser) Supports(raw []byte) bool {
	line := strings.TrimSpace(string(raw))
	return strings.HasPrefix(line, "vless://")
}

func (p *VLESSParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "vless://") {
			continue
		}

		out, err := parseVLESS(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("vless parse error: %v", err))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	return result, nil
}

func parseVLESS(uri string) (*compiler.Outbound, error) {
	// Format: vless://UUID@server:port?params#name
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid vless URI: %w", err)
	}

	uuid := u.User.String()
	if uuid == "" {
		return nil, fmt.Errorf("missing UUID in vless URI")
	}

	port := 443
	fmt.Sscanf(u.Port(), "%d", &port)

	name := u.Fragment
	if name == "" {
		name = u.Hostname()
	}

	params := u.Query()

	out := &compiler.Outbound{
		ID:      sanitizeID(name),
		Name:    name,
		Type:    compiler.OutboundVLESS,
		Server:  u.Hostname(),
		Port:    port,
		UUID:    uuid,
		Enabled: true,
	}

	// Query parameters
	if enc := params.Get("encryption"); enc != "" {
		out.Security = enc
	}
	if flow := params.Get("flow"); flow != "" {
		out.Password2 = flow
	}
	security := params.Get("security")
	if security == "tls" || security == "reality" {
		out.TLS = true
	}
	out.Network = params.Get("type")
	out.TransportPath = params.Get("path")
	out.TransportHost = params.Get("host")
	out.ServiceName = params.Get("serviceName")
	if sni := params.Get("sni"); sni != "" {
		out.SNI = sni
	}
	if fp := params.Get("fp"); fp != "" {
		out.Fingerprint = fp
	}
	if alpn := params.Get("alpn"); alpn != "" {
		out.ALPN = strings.Split(alpn, ",")
	}
	if insecure := params.Get("allowInsecure"); insecure == "1" || insecure == "true" {
		out.SkipCertVerify = true
	}
	out.RealityPublicKey = params.Get("pbk")
	out.RealityShortID = params.Get("sid")

	return out, nil
}
