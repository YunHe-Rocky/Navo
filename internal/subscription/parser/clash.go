package parser

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

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
	var document struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
		Legacy  []map[string]interface{} `yaml:"Proxy"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse Clash YAML: %w", err)
	}
	proxies := append(document.Proxies, document.Legacy...)
	result := &Result{Outbounds: make([]compiler.Outbound, 0, len(proxies))}
	for index, proxy := range proxies {
		values := flattenClashProxy(proxy)
		out := buildClashOutbound(values)
		if out == nil {
			name := values["name"]
			if name == "" {
				name = fmt.Sprintf("index %d", index)
			}
			result.Errors = append(result.Errors, fmt.Sprintf("proxy %q has unsupported type or invalid server/port", name))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	if len(result.Outbounds) == 0 {
		result.Errors = append(result.Errors, "no proxies found in Clash config")
	}

	return result, nil
}

func flattenClashProxy(proxy map[string]interface{}) map[string]string {
	result := make(map[string]string, len(proxy)+8)
	for key, value := range proxy {
		switch scalar := value.(type) {
		case string:
			result[key] = scalar
		case int:
			result[key] = strconv.Itoa(scalar)
		case uint64:
			result[key] = strconv.FormatUint(scalar, 10)
		case bool:
			result[key] = strconv.FormatBool(scalar)
		}
	}
	copyNested := func(section, target, source string) {
		if nested, ok := proxy[section].(map[string]interface{}); ok {
			if value, exists := nested[source]; exists {
				result[target] = fmt.Sprint(value)
			}
		}
	}
	copyNested("ws-opts", "path", "path")
	if ws, ok := proxy["ws-opts"].(map[string]interface{}); ok {
		if headers, ok := ws["headers"].(map[string]interface{}); ok {
			if host, exists := headers["Host"]; exists {
				result["host"] = fmt.Sprint(host)
			} else if host, exists := headers["host"]; exists {
				result["host"] = fmt.Sprint(host)
			}
		}
	}
	copyNested("grpc-opts", "grpc-service-name", "grpc-service-name")
	if result["grpc-service-name"] == "" {
		copyNested("grpc-opts", "grpc-service-name", "service-name")
	}
	copyNested("reality-opts", "public-key", "public-key")
	copyNested("reality-opts", "short-id", "short-id")
	return result
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
