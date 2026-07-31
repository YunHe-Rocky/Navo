package endpoint

import "testing"

func TestEndpointValidate(t *testing.T) {
	t.Parallel()

	valid := Endpoint{
		ID: "endpoint-1", ProviderID: "subscription-1", Name: "node",
		Protocol: ProtocolVLESS, Server: "proxy.example.com", Port: 443,
		Enabled: true, SpecVersion: 1,
		Spec: VLESSSpec{
			UUID:       "00000000-0000-0000-0000-000000000001",
			Encryption: "none",
			TLS:        TLSOptions{Enabled: true, ServerName: "proxy.example.com"},
			Transport:  TransportOptions{Type: TransportWebSocket, Path: "/ws"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid endpoint rejected: %v", err)
	}

	mismatch := valid
	mismatch.Protocol = ProtocolTrojan
	if err := mismatch.Validate(); err == nil {
		t.Fatal("protocol/spec mismatch accepted")
	}

	invalidReality := valid
	invalidReality.Spec = VLESSSpec{
		UUID: "id", Encryption: "none",
		TLS: TLSOptions{Reality: &RealityOptions{}},
	}
	if err := invalidReality.Validate(); err == nil {
		t.Fatal("invalid Reality options accepted")
	}
}

func TestUpstreamProxyValidate(t *testing.T) {
	t.Parallel()

	valid := UpstreamProxy{
		ID: "upstream-1", Name: "corp", Protocol: UpstreamSOCKS5,
		Server: "127.0.0.1", Port: 1080, UDPPolicy: UDPPolicyPrefer, Enabled: true,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid upstream rejected: %v", err)
	}

	httpWithUDP := valid
	httpWithUDP.Protocol = UpstreamHTTP
	if err := httpWithUDP.Validate(); err == nil {
		t.Fatal("HTTP upstream with UDP accepted")
	}

	httpsWithoutTLS := valid
	httpsWithoutTLS.Protocol = UpstreamHTTPS
	httpsWithoutTLS.UDPPolicy = UDPPolicyDisabled
	if err := httpsWithoutTLS.Validate(); err == nil {
		t.Fatal("HTTPS upstream without TLS accepted")
	}
}
