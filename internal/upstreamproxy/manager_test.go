package upstreamproxy

import (
	"path/filepath"
	"testing"

	"navo/internal/domain/endpoint"
)

func TestManagerPersistsOnlyCredentialReferences(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "upstreams.json")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	usernameRef, passwordRef := "credential://user", "credential://password"
	value := endpoint.UpstreamProxy{
		ID: "proxy-1", Name: "Proxy", Protocol: endpoint.UpstreamSOCKS5,
		Server: "127.0.0.1", Port: 1080, UsernameRef: &usernameRef,
		PasswordRef: &passwordRef, UDPPolicy: endpoint.UDPPolicyDisabled, Enabled: true,
	}
	if err := manager.Add(value); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.Get(value.ID)
	if !ok || got.PasswordRef == nil || *got.PasswordRef != passwordRef {
		t.Fatalf("reloaded value = %+v", got)
	}
}
