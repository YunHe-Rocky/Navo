package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"navo/internal/compiler"
)

// VMessParser handles VMess (vmess://) subscriptions.
type VMessParser struct{}

func NewVMessParser() *VMessParser { return &VMessParser{} }

func (p *VMessParser) Supports(raw []byte) bool {
	line := strings.TrimSpace(string(raw))
	return strings.HasPrefix(line, "vmess://")
}

func (p *VMessParser) Parse(raw []byte) (*Result, error) {
	result := &Result{}
	lines := strings.Split(string(raw), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "vmess://") {
			continue
		}

		out, err := parseVMess(line)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("vmess parse error: %v", err))
			continue
		}
		result.Outbounds = append(result.Outbounds, *out)
	}

	return result, nil
}

func parseVMess(uri string) (*compiler.Outbound, error) {
	// Format: vmess://BASE64(JSON)
	encoded := strings.TrimPrefix(uri, "vmess://")

	// Remove fragment
	if idx := strings.Index(encoded, "#"); idx != -1 {
		encoded = encoded[:idx]
	}

	decoded, err := base64URLDecode(encoded)
	if err != nil {
		return nil, fmt.Errorf("vmess base64 decode: %w", err)
	}

	var vm struct {
		Add  string      `json:"add"`  // server
		Port interface{} `json:"port"` // can be string or number
		ID   string      `json:"id"`   // UUID
		Aid  interface{} `json:"aid"`
		Net  string      `json:"net"`  // network type
		Type string      `json:"type"` // security
		Scy  string      `json:"scy"`  // encryption method
		Host string      `json:"host"`
		Path string      `json:"path"`
		TLS  string      `json:"tls"`
		SNI  string      `json:"sni"`
		PS   string      `json:"ps"` // name / remark
	}

	if err := json.Unmarshal([]byte(decoded), &vm); err != nil {
		return nil, fmt.Errorf("vmess JSON parse: %w", err)
	}

	port := 0
	switch v := vm.Port.(type) {
	case float64:
		port = int(v)
	case string:
		fmt.Sscanf(v, "%d", &port)
	}
	if vm.Add == "" || port == 0 {
		return nil, fmt.Errorf("missing server or port in vmess config")
	}

	name := vm.PS
	if name == "" {
		name = vm.Add
	}

	out := &compiler.Outbound{
		ID:            sanitizeID(name),
		Name:          name,
		Type:          compiler.OutboundVMess,
		Server:        vm.Add,
		Port:          port,
		UUID:          vm.ID,
		Security:      vm.Scy,
		Network:       vm.Net,
		TransportPath: vm.Path,
		TransportHost: vm.Host,
		Enabled:       true,
	}
	if out.Security == "" {
		out.Security = "auto"
	}

	if vm.TLS == "tls" {
		out.TLS = true
		out.SNI = vm.SNI
		if vm.SNI == "" {
			out.SNI = vm.Host
		}
	}

	return out, nil
}
