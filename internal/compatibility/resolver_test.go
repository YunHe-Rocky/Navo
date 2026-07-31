package compatibility

import (
	"testing"

	"navo/internal/coreadapter"
	"navo/internal/domain/capture"
	domaincompat "navo/internal/domain/compatibility"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
)

func TestResolverSixCoreSourceCombinations(t *testing.T) {
	t.Parallel()
	resolver := NewResolver(nil)
	version := coreadapter.Version{Raw: "pinned"}

	vless := validVLESS()
	socks := endpoint.Endpoint{
		ID: "socks", ProviderID: "provider", Name: "SOCKS",
		Protocol: endpoint.ProtocolSOCKS5, Server: "127.0.0.1", Port: 1080,
		Enabled: true, SpecVersion: 1, Spec: endpoint.SOCKS5ProxySpec{},
	}
	for _, coreType := range core.All() {
		for _, target := range []endpoint.Endpoint{vless, socks} {
			result := resolver.Resolve(coreType, version, target, capture.ModeSystemProxy)
			if !result.Supported {
				t.Errorf("%s + %s unexpectedly unsupported: %+v", coreType, target.Protocol, result.Reasons)
			}
		}
	}
}

func TestResolverRejectsUnsupportedProtocolAndCapture(t *testing.T) {
	t.Parallel()
	resolver := NewResolver(nil)
	version := coreadapter.Version{Raw: "26.3.27"}
	hy2 := endpoint.Endpoint{
		ID: "hy2", ProviderID: "provider", Name: "HY2",
		Protocol: endpoint.ProtocolHysteria2, Server: "example.com", Port: 443,
		Enabled: true, SpecVersion: 1,
		Spec: endpoint.Hysteria2Spec{
			PasswordRef: "credential://hy2",
			TLS:         endpoint.TLSOptions{Enabled: true, ServerName: "example.com"},
		},
	}
	result := resolver.Resolve(core.TypeXray, version, hy2, capture.ModeTUN)
	if result.Supported || result.Level != domaincompat.LevelUnsupported || len(result.Reasons) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResolverReportsInsecureTLSAsLimitation(t *testing.T) {
	t.Parallel()
	target := validVLESS()
	spec := target.Spec.(endpoint.VLESSSpec)
	spec.TLS.Insecure = true
	target.Spec = spec
	result := NewResolver(nil).Resolve(
		core.TypeSingBox, coreadapter.Version{Raw: "1.13.14"}, target, capture.ModeOff,
	)
	if result.Level != domaincompat.LevelSupportedWithLimitations || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func validVLESS() endpoint.Endpoint {
	return endpoint.Endpoint{
		ID: "vless", ProviderID: "provider", Name: "VLESS",
		Protocol: endpoint.ProtocolVLESS, Server: "example.com", Port: 443,
		Enabled: true, SpecVersion: 1,
		Spec: endpoint.VLESSSpec{
			UUID: "00000000-0000-0000-0000-000000000001", Encryption: "none",
			TLS:       endpoint.TLSOptions{Enabled: true, ServerName: "example.com"},
			Transport: endpoint.TransportOptions{Type: endpoint.TransportTCP},
		},
	}
}
