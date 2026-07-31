package parser

import (
	"fmt"
	"net/url"
	"strings"

	"navo/internal/compiler"
)

// SOCKSParser handles SOCKS5 (socks5://) subscriptions.
type SOCKSParser struct{}

func NewSOCKSParser() *SOCKSParser { return &SOCKSParser{} }

func (p *SOCKSParser) Supports(raw []byte) bool {
	line := strings.TrimSpace(string(raw))
	return strings.HasPrefix(line, "socks5://")
}

func (p *SOCKSParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "socks5://") {
			continue
		}

		out, err := parseSocks5(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("socks5 parse error: %v", err))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	return result, nil
}

func parseSocks5(uri string) (*compiler.Outbound, error) {
	// Format: socks5://user:pass@server:port#name
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid socks5 URI: %w", err)
	}

	port := 1080
	fmt.Sscanf(u.Port(), "%d", &port)

	name := u.Fragment
	if name == "" {
		name = u.Hostname()
	}

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	return &compiler.Outbound{
		ID:       sanitizeID(name),
		Name:     name,
		Type:     compiler.OutboundSOCKS,
		Server:   u.Hostname(),
		Port:     port,
		Username: username,
		Password: password,
		Enabled:  true,
	}, nil
}
