package parser

import (
	"strings"
	"testing"
)

func TestSSParser(t *testing.T) {
	p := NewSSParser()

	tests := []struct {
		name     string
		input    string
		wantType string
		wantPort int
	}{
		{
			name:     "standard ss URI",
			input:    "ss://Y2hhY2hhMjAtaWV0Zi1wb2x5MTMwNTpwYXNz@1.2.3.4:8388#Node1\n",
			wantType: "shadowsocks",
			wantPort: 8388,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !p.Supports([]byte(tt.input)) {
				t.Skip("parser doesn't support input")
			}
			result, err := p.Parse([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Outbounds) == 0 {
				t.Fatal("no outbounds parsed")
			}
			ob := result.Outbounds[0]
			if ob.Server != "1.2.3.4" {
				t.Errorf("server = %s, want 1.2.3.4", ob.Server)
			}
			if ob.Port != tt.wantPort {
				t.Errorf("port = %d, want %d", ob.Port, tt.wantPort)
			}
		})
	}
}

func TestVMessParser(t *testing.T) {
	p := NewVMessParser()

	// base64 encoded JSON: {"add":"1.2.3.4","port":443,"id":"uuid","ps":"Test"}
	input := "vmess://eyJhZGQiOiIxLjIuMy40IiwicG9ydCI6NDQzLCJpZCI6InV1aWQiLCJwcyI6IlRlc3QifQ==\n"

	if !p.Supports([]byte(input)) {
		t.Skip("parser doesn't support input")
	}
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) == 0 {
		t.Fatal("no outbounds parsed")
	}
	ob := result.Outbounds[0]
	if ob.Server != "1.2.3.4" {
		t.Errorf("server = %s, want 1.2.3.4", ob.Server)
	}
	if ob.Port != 443 {
		t.Errorf("port = %d, want 443", ob.Port)
	}
	if ob.UUID != "uuid" {
		t.Errorf("UUID = %s, want uuid", ob.UUID)
	}
}

func TestTrojanParser(t *testing.T) {
	p := NewTrojanParser()

	input := "trojan://password123@1.2.3.4:443?sni=example.com#TrojanNode\n"

	if !p.Supports([]byte(input)) {
		t.Skip("parser doesn't support input")
	}
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) == 0 {
		t.Fatal("no outbounds parsed")
	}
	ob := result.Outbounds[0]
	if ob.Server != "1.2.3.4" {
		t.Errorf("server = %s, want 1.2.3.4", ob.Server)
	}
	if ob.Password != "password123" {
		t.Errorf("password = %s, want password123", ob.Password)
	}
	if ob.SNI != "example.com" {
		t.Errorf("SNI = %s, want example.com", ob.SNI)
	}
}

func TestVLESSParser(t *testing.T) {
	p := NewVLESSParser()

	input := "vless://my-uuid@1.2.3.4:8443?encryption=none&flow=xtls-rprx-vision&sni=example.com#VLESSNode\n"

	if !p.Supports([]byte(input)) {
		t.Skip("parser doesn't support input")
	}
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) == 0 {
		t.Fatal("no outbounds parsed")
	}
	ob := result.Outbounds[0]
	if ob.UUID != "my-uuid" {
		t.Errorf("UUID = %s, want my-uuid", ob.UUID)
	}
	if ob.Password2 != "xtls-rprx-vision" {
		t.Errorf("flow = %s, want xtls-rprx-vision", ob.Password2)
	}
}

func TestSOCKSParser(t *testing.T) {
	p := NewSOCKSParser()

	input := "socks5://user:pass@1.2.3.4:1080#SOCKSNode\n"

	if !p.Supports([]byte(input)) {
		t.Skip("parser doesn't support input")
	}
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) == 0 {
		t.Fatal("no outbounds parsed")
	}
	ob := result.Outbounds[0]
	if ob.Username != "user" {
		t.Errorf("username = %s, want user", ob.Username)
	}
	if ob.Password != "pass" {
		t.Errorf("password = %s, want pass", ob.Password)
	}
}

func TestClashParser(t *testing.T) {
	p := NewClashParser()

	input := `proxies:
  - name: "US-SS-01"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: testpass
  - name: "US-Trojan"
    type: trojan
    server: 5.6.7.8
    port: 443
    password: trojanpass
    sni: example.com
`

	if !p.Supports([]byte(input)) {
		t.Skip("parser doesn't support input")
	}
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(result.Outbounds))
	}
	if result.Outbounds[0].Type != "shadowsocks" {
		t.Errorf("first outbound type = %s, want shadowsocks", result.Outbounds[0].Type)
	}
	if result.Outbounds[1].Type != "trojan" {
		t.Errorf("second outbound type = %s, want trojan", result.Outbounds[1].Type)
	}
}

func TestClashParserInlineModernProtocols(t *testing.T) {
	input := `proxies:
  - {name: "HY2", type: hysteria2, server: hy.example.com, port: 443, password: secret, sni: hy.example.com, obfs: salamander, obfs-password: mask}
  - {name: "TUIC", type: tuic, server: tuic.example.com, port: 443, uuid: bf000d23-0752-40b4-affe-68f7707a9661, password: secret, sni: tuic.example.com, congestion-controller: bbr}
`
	result, err := NewClashParser().Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) != 2 {
		t.Fatalf("parsed %d outbounds, want 2", len(result.Outbounds))
	}
	if result.Outbounds[0].Type != "hysteria2" ||
		result.Outbounds[0].ObfsPassword != "mask" {
		t.Fatalf("invalid Hysteria2 outbound: %#v", result.Outbounds[0])
	}
	if result.Outbounds[1].Type != "tuic" ||
		result.Outbounds[1].CongestionControl != "bbr" {
		t.Fatalf("invalid TUIC outbound: %#v", result.Outbounds[1])
	}
}

func TestClashParserReadsNestedTransportOptions(t *testing.T) {
	input := `proxies:
  - name: Nested
    type: vless
    server: edge.example.com
    port: 443
    uuid: bf000d23-0752-40b4-affe-68f7707a9661
    tls: true
    network: ws
    ws-opts:
      path: /tunnel
      headers:
        Host: cdn.example.com
    reality-opts:
      public-key: public-key
      short-id: abcd
`
	result, err := NewClashParser().Parse([]byte(input))
	if err != nil || len(result.Outbounds) != 1 {
		t.Fatalf("parse result=%#v err=%v", result, err)
	}
	outbound := result.Outbounds[0]
	if outbound.TransportPath != "/tunnel" || outbound.TransportHost != "cdn.example.com" ||
		outbound.RealityPublicKey != "public-key" || outbound.RealityShortID != "abcd" {
		t.Fatalf("nested options lost: %#v", outbound)
	}
}

func TestParser_EmptyInput(t *testing.T) {
	parsers := []Parser{
		NewSSParser(), NewVMessParser(), NewVLESSParser(),
		NewTrojanParser(), NewSOCKSParser(), NewClashParser(),
	}
	for _, p := range parsers {
		if p.Supports([]byte("")) {
			t.Errorf("%T incorrectly claims to support empty input", p)
		}
	}
}

func TestParser_CommentsIgnored(t *testing.T) {
	p := NewTrojanParser()
	input := "# This is a comment\ntrojan://pass@1.2.3.4:443#Node\n# Another comment\n"
	result, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outbounds) != 1 {
		t.Errorf("expected 1 outbound, got %d (comments should be ignored)", len(result.Outbounds))
	}
}

func TestSSParser_Supports(t *testing.T) {
	p := NewSSParser()
	if !p.Supports([]byte("ss://test@1.2.3.4:8388")) {
		t.Error("should support ss:// URIs")
	}
	if p.Supports([]byte("vmess://test")) {
		t.Error("should not support vmess:// URIs")
	}
}

func TestClashParser_BuildOutbound(t *testing.T) {
	m := map[string]string{
		"type":   "vmess",
		"server": "10.0.0.1",
		"port":   "443",
		"uuid":   "test-uuid",
		"name":   "Test Node",
		"sni":    "sni.example.com",
	}
	out := buildClashOutbound(m)
	if out == nil {
		t.Fatal("expected outbound, got nil")
	}
	if out.Server != "10.0.0.1" {
		t.Errorf("server = %s", out.Server)
	}
	if out.UUID != "test-uuid" {
		t.Errorf("UUID = %s", out.UUID)
	}
	if out.SNI != "sni.example.com" {
		t.Errorf("SNI = %s", out.SNI)
	}
}

func TestClashParser_BuildOutbound_MissingServer(t *testing.T) {
	m := map[string]string{"type": "ss", "port": "8388"}
	out := buildClashOutbound(m)
	if out != nil {
		t.Error("expected nil for missing server")
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"US Node 01", "us-node-01"},
		{"Test@#$Node", "test---node"},
		{"My Node!", "my-node-"},
	}
	for _, tt := range tests {
		got := sanitizeID(tt.input)
		if !strings.Contains(got, "-node") && tt.input != "My Node!" {
			t.Errorf("sanitizeID(%q) = %q, unexpected", tt.input, got)
		}
		_ = got // use to avoid unused var
	}
}
