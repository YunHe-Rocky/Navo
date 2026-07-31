package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"navo/internal/compiler"
)

// ClashParser handles Clash YAML-format subscriptions.
type ClashParser struct{}

func NewClashParser() *ClashParser { return &ClashParser{} }

func (p *ClashParser) Supports(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return strings.Contains(s, "proxies:") || strings.Contains(s, "Proxy:")
}

func (p *ClashParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}

	// Simple line-based parsing for common Clash YAML format
	// Looks for lines like: - {name: "Node1", type: ss, server: x.x.x.x, port: 8388, ...}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	var currentProxy map[string]string
	inProxies := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect proxies section
		if strings.HasPrefix(line, "proxies:") || strings.HasPrefix(line, "Proxy:") {
			inProxies = true
			continue
		}

		if !inProxies {
			continue
		}

		// End of proxies section
		if len(line) > 0 && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "#") && currentProxy == nil {
			inProxies = false
			continue
		}

		// Parse proxy entry
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "-{") {
			// Flush previous proxy
			if currentProxy != nil {
				if out := buildClashOutbound(currentProxy); out != nil {
					result.Outbounds = append(result.Outbounds, *out)
				}
			}
			currentProxy = make(map[string]string)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "{")
			line = strings.TrimSuffix(line, "}")
			if strings.Contains(line, ",") {
				parseInlineClashMap(line, currentProxy)
				continue
			}
		}

		if currentProxy != nil {
			parseClashKeyValue(line, currentProxy)
		}
	}

	// Flush last proxy
	if currentProxy != nil {
		if out := buildClashOutbound(currentProxy); out != nil {
			result.Outbounds = append(result.Outbounds, *out)
		}
	}

	if len(result.Outbounds) == 0 {
		result.Errors = append(result.Errors, "no proxies found in Clash config")
	}

	return result, nil
}

func parseInlineClashMap(line string, values map[string]string) {
	start := 0
	var quote rune
	depth := 0
	runes := []rune(line)
	for i, r := range runes {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			continue
		case r == '"' || r == '\'':
			quote = r
		case r == '[' || r == '{':
			depth++
		case r == ']' || r == '}':
			if depth > 0 {
				depth--
			}
		case r == ',' && depth == 0:
			parseClashKeyValue(string(runes[start:i]), values)
			start = i + 1
		}
	}
	if start < len(runes) {
		parseClashKeyValue(string(runes[start:]), values)
	}
}

func parseClashKeyValue(line string, m map[string]string) {
	// Parse "key: value" or "key:value" pairs, handling quoted values
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, `'`)
	m[key] = value
}

func buildClashOutbound(m map[string]string) *compiler.Outbound {
	typ := m["type"]
	if typ == "" {
		return nil
	}

	server := m["server"]
	port := 0
	fmt.Sscanf(m["port"], "%d", &port)

	if server == "" || port == 0 {
		return nil
	}

	name := m["name"]
	if name == "" {
		name = server
	}

	obType := mapClashType(typ)
	if obType == "" {
		return nil
	}
	out := &compiler.Outbound{
		ID:      sanitizeID(name),
		Name:    name,
		Type:    obType,
		Server:  server,
		Port:    port,
		Enabled: true,
	}

	// Protocol-specific fields
	switch typ {
	case "ss", "shadowsocks":
		out.Method = m["cipher"]
		out.Password = m["password"]
	case "vmess":
		out.UUID = m["uuid"]
		out.Security = m["cipher"]
		out.Username = m["uuid"]
		out.TLS = m["tls"] == "true"
	case "trojan":
		out.Password = m["password"]
		out.SNI = m["sni"]
		out.TLS = true
	case "vless":
		out.UUID = m["uuid"]
		out.Password2 = m["flow"]
		out.TLS = m["tls"] == "true"
	case "hysteria2", "hy2":
		out.Password = m["password"]
		out.TLS = true
		out.ObfsType = m["obfs"]
		out.ObfsPassword = m["obfs-password"]
	case "tuic":
		out.UUID = m["uuid"]
		out.Password = m["password"]
		out.TLS = true
		out.CongestionControl = m["congestion-controller"]
	case "socks5":
		out.Username = m["username"]
		out.Password = m["password"]
	}

	if m["sni"] != "" {
		out.SNI = m["sni"]
	} else if m["servername"] != "" {
		out.SNI = m["servername"]
	}
	out.Fingerprint = m["client-fingerprint"]
	if m["skip-cert-verify"] == "true" {
		out.SkipCertVerify = true
	}
	out.Network = m["network"]
	out.TransportPath = m["path"]
	out.TransportHost = m["host"]
	if out.TransportHost == "" {
		out.TransportHost = m["Host"]
	}
	out.ServiceName = m["grpc-service-name"]
	out.RealityPublicKey = m["public-key"]
	out.RealityShortID = m["short-id"]
	if out.RealityPublicKey != "" {
		out.TLS = true
	}

	return out
}

func mapClashType(t string) compiler.OutboundType {
	switch strings.ToLower(t) {
	case "ss", "shadowsocks":
		return compiler.OutboundShadowsocks
	case "vmess":
		return compiler.OutboundVMess
	case "vless":
		return compiler.OutboundVLESS
	case "trojan":
		return compiler.OutboundTrojan
	case "socks5", "socks":
		return compiler.OutboundSOCKS
	case "http":
		return compiler.OutboundHTTP
	case "hysteria2", "hy2":
		return compiler.OutboundHysteria2
	case "tuic":
		return compiler.OutboundTUIC
	default:
		return ""
	}
}

func sanitizeID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, id)
	return id
}
