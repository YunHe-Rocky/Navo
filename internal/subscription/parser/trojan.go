package parser

import (
	"fmt"
	"net/url"
	"strings"

	"navo/internal/compiler"
)

// TrojanParser handles Trojan (trojan://) subscriptions.
type TrojanParser struct{}

func NewTrojanParser() *TrojanParser { return &TrojanParser{} }

func (p *TrojanParser) Supports(raw []byte) bool {
	line := strings.TrimSpace(string(raw))
	return strings.HasPrefix(line, "trojan://")
}

func (p *TrojanParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "trojan://") {
			continue
		}

		out, err := parseTrojan(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("trojan parse error: %v", err))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	return result, nil
}

func parseTrojan(uri string) (*compiler.Outbound, error) {
	// Format: trojan://password@server:port?params#name
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid trojan URI: %w", err)
	}

	password := u.User.String()
	if password == "" {
		return nil, fmt.Errorf("missing password in trojan URI")
	}

	port := 443
	fmt.Sscanf(u.Port(), "%d", &port)

	name := u.Fragment
	if name == "" {
		name = u.Hostname()
	}

	params := u.Query()

	out := &compiler.Outbound{
		ID:       sanitizeID(name),
		Name:     name,
		Type:     compiler.OutboundTrojan,
		Server:   u.Hostname(),
		Port:     port,
		Password: password,
		TLS:      true,
		Enabled:  true,
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

	return out, nil
}
