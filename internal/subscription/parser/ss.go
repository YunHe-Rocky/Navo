package parser

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"navo/internal/compiler"
)

// SSParser handles Shadowsocks (ss://) subscriptions.
type SSParser struct{}

func NewSSParser() *SSParser { return &SSParser{} }

func (p *SSParser) Supports(raw []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "ss://")
}

func (p *SSParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "ss://") {
			continue
		}

		out, err := parseSS(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("ss parse error: %v", err))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	return result, nil
}

func parseSS(uri string) (*compiler.Outbound, error) {
	// Format: ss://BASE64(method:password)@server:port#name
	// Or:     ss://BASE64(method:password@server:port)#name
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid ss URI: %w", err)
	}

	userInfo := u.User.String()
	if userInfo == "" {
		// SIP002 format: userinfo is the base64 part, server:port is host
		encoded := u.Host
		if idx := strings.Index(uri, "@"); idx != -1 {
			encoded = uri[len("ss://"):idx]
		}
		decoded, err := base64URLDecode(encoded)
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(decoded, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid ss userinfo: %s", decoded)
		}
		method := parts[0]
		password := parts[1]

		port := 8388
		fmt.Sscanf(u.Port(), "%d", &port)

		name := u.Fragment
		if name == "" {
			name = u.Hostname()
		}

		return &compiler.Outbound{
			ID:       sanitizeID(name),
			Name:     name,
			Type:     compiler.OutboundShadowsocks,
			Server:   u.Hostname(),
			Port:     port,
			Method:   method,
			Password: password,
			Enabled:  true,
		}, nil
	}

	// Legacy format: ss://BASE64@server:port
	decoded, err := base64URLDecode(userInfo)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(decoded, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ss format: %s", decoded)
	}

	method := parts[0]
	password := parts[1]
	port := 8388
	fmt.Sscanf(u.Port(), "%d", &port)

	name := u.Fragment
	if name == "" {
		name = u.Hostname()
	}

	return &compiler.Outbound{
		ID:       sanitizeID(name),
		Name:     name,
		Type:     compiler.OutboundShadowsocks,
		Server:   u.Hostname(),
		Port:     port,
		Method:   method,
		Password: password,
		Enabled:  true,
	}, nil
}

func base64URLDecode(s string) (string, error) {
	// Trim #fragment if present
	if idx := strings.Index(s, "#"); idx != -1 {
		s = s[:idx]
	}

	// Add padding if needed
	s = strings.TrimRight(s, "=")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	// Try URL-safe first, then standard
	decoded, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(s)
	}
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(decoded), nil
}
