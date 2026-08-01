package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CoreSingBox = "sing-box"
	CoreMihomo  = "mihomo"
	CoreXray    = "xray"
)

// GenerateForCore compiles the canonical Navo model for a specific proxy core.
// GenerateForCore is retained for legacy callers. New runtime code must select
// one concrete coreadapter.CoreAdapter instead of adding branches here.
func GenerateForCore(coreID string, cfg *Config) ([]byte, error) {
	switch coreID {
	case "", CoreSingBox:
		return Generate(cfg)
	case CoreMihomo:
		return GenerateMihomo(cfg)
	case CoreXray:
		return GenerateXray(cfg)
	default:
		return nil, fmt.Errorf("unsupported core %q", coreID)
	}
}

// Compatible reports whether a core can faithfully represent an outbound.
func Compatible(coreID string, outbound Outbound) bool {
	switch coreID {
	case CoreSingBox:
		return outbound.Type != OutboundWireGuard
	case CoreMihomo:
		return outbound.Type != OutboundWireGuard
	case CoreXray:
		switch outbound.Type {
		case OutboundDirect, OutboundSOCKS, OutboundHTTP, OutboundShadowsocks,
			OutboundVMess, OutboundVLESS, OutboundTrojan:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// GenerateMihomo emits a native Mihomo YAML document.
func GenerateMihomo(cfg *Config) ([]byte, error) {
	root := map[string]any{
		"mixed-port": 0,
		"allow-lan":  false,
		"mode":       "rule",
		"log-level":  cfg.Log.Level,
		"ipv6":       cfg.TUN != nil && cfg.TUN.IPv6Enabled,
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Type == "mixed" {
			root["mixed-port"] = inbound.ListenPort
			root["bind-address"] = inbound.Listen
		}
	}
	if cfg.Controller != nil {
		root["external-controller"] = fmt.Sprintf("%s:%d", cfg.Controller.Listen, cfg.Controller.Port)
		root["secret"] = cfg.Controller.Secret
	}
	if cfg.TUN != nil && cfg.TUN.Enabled {
		tunConfig := map[string]any{
			"enable": true, "stack": "mixed", "device": cfg.TUN.InterfaceName,
			"auto-route": cfg.TUN.AutoRoute, "strict-route": cfg.TUN.StrictRoute,
			"auto-detect-interface": true,
			"dns-hijack":            []string{"any:53"},
			"mtu":                   cfg.TUN.MTU,
		}
		if len(cfg.TUN.Address) > 0 {
			// Keep Mihomo's Wintun address aligned with the external route
			// transaction. Without this, routes target 172.19.0.2 while Mihomo
			// creates an adapter in a different subnet.
			tunConfig["inet4-address"] = cfg.TUN.Address
		}
		root["tun"] = tunConfig
		root["dns"] = map[string]any{
			"enable":        true,
			"listen":        "0.0.0.0:1053",
			"enhanced-mode": "fake-ip",
			"nameserver":    []string{"https://1.1.1.1/dns-query"},
		}
	}
	proxies := make([]map[string]any, 0, len(cfg.Outbounds))
	for _, outbound := range cfg.Outbounds {
		if outbound.Type == OutboundDirect {
			continue
		}
		proxy, err := mihomoProxy(outbound)
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, proxy)
	}
	root["proxies"] = proxies
	target := cfg.FinalOutbound
	if target == "" {
		target = "DIRECT"
	}
	root["proxy-groups"] = []map[string]any{{
		"name": "NAVO", "type": "select",
		"proxies": []string{mihomoName(target)},
	}}
	rules := make([]string, 0, len(cfg.RoutingRules)+1)
	for _, rule := range cfg.RoutingRules {
		if !rule.Enabled {
			continue
		}
		targetID := rule.OutboundTag
		if targetID == "" {
			targetID = rule.OutboundID
		}
		target := mihomoName(targetID)
		for _, value := range rule.Values {
			switch rule.RuleType {
			case RuleDomain:
				rules = append(rules, "DOMAIN,"+value+","+target)
			case RuleDomainSuffix:
				rules = append(rules, "DOMAIN-SUFFIX,"+value+","+target)
			case RuleDomainKeyword:
				rules = append(rules, "DOMAIN-KEYWORD,"+value+","+target)
			case RuleDomainRegex:
				rules = append(rules, "DOMAIN-REGEX,"+value+","+target)
			case RuleIP:
				rules = append(rules, "IP-CIDR,"+value+","+target+",no-resolve")
			case RuleProcess:
				rules = append(rules, "PROCESS-NAME,"+value+","+target)
			case RulePort:
				rules = append(rules, "DST-PORT,"+value+","+target)
			case RuleProtocol:
				rules = append(rules, "NETWORK,"+value+","+target)
			case RuleGeosite:
				rules = append(rules, "GEOSITE,"+value+","+target)
			case RuleGeoip:
				rules = append(rules, "GEOIP,"+value+","+target+",no-resolve")
			default:
				return nil, fmt.Errorf("mihomo does not support routing rule type %q", rule.RuleType)
			}
		}
	}
	rules = append(rules, "MATCH,NAVO")
	root["rules"] = rules
	return yaml.Marshal(root)
}

func mihomoName(id string) string {
	if id == "" || id == "direct" {
		return "DIRECT"
	}
	return id
}

func mihomoProxy(o Outbound) (map[string]any, error) {
	p := map[string]any{"name": o.ID, "server": o.Server, "port": o.Port}
	switch o.Type {
	case OutboundSOCKS:
		p["type"], p["username"], p["password"] = "socks5", o.Username, o.Password
	case OutboundHTTP:
		p["type"], p["username"], p["password"] = "http", o.Username, o.Password
	case OutboundShadowsocks:
		p["type"], p["cipher"], p["password"] = "ss", o.Method, o.Password
	case OutboundVMess:
		p["type"], p["uuid"], p["cipher"] = "vmess", o.UUID, defaultString(o.Security, "auto")
	case OutboundVLESS:
		p["type"], p["uuid"], p["flow"] = "vless", o.UUID, o.Password2
	case OutboundTrojan:
		p["type"], p["password"] = "trojan", o.Password
	case OutboundHysteria2:
		p["type"], p["password"] = "hysteria2", o.Password
	case OutboundTUIC:
		p["type"], p["uuid"], p["password"] = "tuic", o.UUID, o.Password
	case OutboundWireGuard:
		return nil, fmt.Errorf("mihomo WireGuard is unsupported until all key, peer, address and route fields are preserved")
	default:
		return nil, fmt.Errorf("mihomo does not support outbound %q", o.Type)
	}
	if o.TLS || o.SNI != "" {
		p["tls"], p["servername"], p["skip-cert-verify"] = true, o.SNI, o.SkipCertVerify
	}
	switch o.Network {
	case "", "tcp":
	case "ws":
		p["network"] = "ws"
		p["ws-opts"] = map[string]any{
			"path": o.TransportPath, "headers": map[string]string{"Host": o.TransportHost},
		}
	case "grpc":
		p["network"] = "grpc"
		p["grpc-opts"] = map[string]any{"grpc-service-name": o.ServiceName}
	default:
		return nil, fmt.Errorf("mihomo does not support transport %q", o.Network)
	}
	if o.RealityPublicKey != "" {
		p["reality-opts"] = map[string]any{
			"public-key": o.RealityPublicKey, "short-id": o.RealityShortID,
		}
	}
	return p, nil
}

// GenerateXray emits a native Xray JSON document.
func GenerateXray(cfg *Config) ([]byte, error) {
	if cfg.TUN != nil && cfg.TUN.Enabled {
		return nil, fmt.Errorf("Xray TUN is unsupported by the bundled adapter version")
	}
	root := map[string]any{
		"log": map[string]any{"loglevel": cfg.Log.Level},
	}
	inbounds := make([]map[string]any, 0, len(cfg.Inbounds))
	for _, inbound := range cfg.Inbounds {
		if inbound.Type != "mixed" {
			continue
		}
		inbounds = append(inbounds,
			map[string]any{"tag": "http-in", "listen": inbound.Listen, "port": inbound.ListenPort, "protocol": "http"},
			map[string]any{"tag": "socks-in", "listen": inbound.Listen, "port": inbound.ListenPort + 1, "protocol": "socks", "settings": map[string]any{"udp": true}},
		)
	}
	root["inbounds"] = inbounds
	outbounds := make([]map[string]any, 0, len(cfg.Outbounds))
	for _, outbound := range cfg.Outbounds {
		item, err := xrayOutbound(outbound)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, item)
	}
	root["outbounds"] = outbounds
	hasBlackhole := false
	for _, outbound := range outbounds {
		if outbound["tag"] == "block" {
			hasBlackhole = true
			break
		}
	}
	if !hasBlackhole {
		outbounds = append(outbounds, map[string]any{
			"tag": "block", "protocol": "blackhole", "settings": map[string]any{},
		})
		root["outbounds"] = outbounds
	}
	final := cfg.FinalOutbound
	if final == "" {
		final = "direct"
	}
	rules := make([]map[string]any, 0, len(cfg.RoutingRules)+1)
	for _, rule := range cfg.RoutingRules {
		if !rule.Enabled {
			continue
		}
		compiled, err := xrayRule(rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, compiled)
	}
	rules = append(rules, map[string]any{
		"type": "field", "network": "tcp,udp", "outboundTag": final,
	})
	root["routing"] = map[string]any{"domainStrategy": "AsIs", "rules": rules}
	root["policy"] = map[string]any{
		"system": map[string]any{
			"statsOutboundUplink": true, "statsOutboundDownlink": true,
		},
	}
	root["stats"] = map[string]any{}
	return json.MarshalIndent(root, "", "  ")
}

func xrayOutbound(o Outbound) (map[string]any, error) {
	result := map[string]any{"tag": o.ID}
	switch o.Type {
	case OutboundDirect:
		result["protocol"], result["settings"] = "freedom", map[string]any{}
		return result, nil
	case OutboundSOCKS, OutboundHTTP, OutboundShadowsocks:
		protocol := string(o.Type)
		if o.Type == OutboundShadowsocks {
			protocol = "shadowsocks"
		}
		result["protocol"] = protocol
		server := map[string]any{"address": o.Server, "port": o.Port}
		if o.Username != "" {
			server["users"] = []map[string]any{{"user": o.Username, "pass": o.Password}}
		} else if o.Type == OutboundShadowsocks {
			server["method"], server["password"] = o.Method, o.Password
		}
		result["settings"] = map[string]any{"servers": []map[string]any{server}}
	case OutboundVMess, OutboundVLESS:
		user := map[string]any{"id": o.UUID}
		if o.Type == OutboundVMess {
			user["security"] = defaultString(o.Security, "auto")
		} else if o.Password2 != "" {
			user["flow"] = o.Password2
		}
		result["protocol"] = string(o.Type)
		result["settings"] = map[string]any{"vnext": []map[string]any{{
			"address": o.Server, "port": o.Port, "users": []map[string]any{user},
		}}}
	case OutboundTrojan:
		result["protocol"] = "trojan"
		result["settings"] = map[string]any{"servers": []map[string]any{{
			"address": o.Server, "port": o.Port, "password": o.Password,
		}}}
	default:
		return nil, fmt.Errorf("xray does not support outbound %q", o.Type)
	}
	network := defaultString(o.Network, "tcp")
	stream := map[string]any{"network": network}
	if o.TLS || o.SNI != "" {
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{
			"serverName": o.SNI, "allowInsecure": o.SkipCertVerify,
			"fingerprint": o.Fingerprint,
		}
	}
	if o.RealityPublicKey != "" {
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]any{
			"serverName": o.SNI, "fingerprint": defaultString(o.Fingerprint, "chrome"),
			"publicKey": o.RealityPublicKey, "shortId": o.RealityShortID,
		}
	}
	if network == "ws" {
		stream["wsSettings"] = map[string]any{
			"path": o.TransportPath, "headers": map[string]string{"Host": o.TransportHost},
		}
	} else if network == "grpc" {
		stream["grpcSettings"] = map[string]any{"serviceName": o.ServiceName}
	}
	result["streamSettings"] = stream
	return result, nil
}

func xrayRule(rule RoutingRule) (map[string]any, error) {
	target := rule.OutboundTag
	if target == "" {
		target = rule.OutboundID
	}
	result := map[string]any{"type": "field", "outboundTag": target}
	switch rule.RuleType {
	case RuleDomain:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "full:" + value
		}
		result["domain"] = values
	case RuleDomainSuffix:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "domain:" + value
		}
		result["domain"] = values
	case RuleDomainKeyword:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "keyword:" + value
		}
		result["domain"] = values
	case RuleDomainRegex:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "regexp:" + value
		}
		result["domain"] = values
	case RuleIP:
		result["ip"] = rule.Values
	case RulePort:
		result["port"] = strings.Join(rule.Values, ",")
	case RuleProtocol:
		result["protocol"] = rule.Values
	case RuleGeosite:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "geosite:" + value
		}
		result["domain"] = values
	case RuleGeoip:
		values := make([]string, len(rule.Values))
		for i, value := range rule.Values {
			values[i] = "geoip:" + value
		}
		result["ip"] = values
	default:
		return nil, fmt.Errorf("xray does not support routing rule type %q", rule.RuleType)
	}
	return result, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
