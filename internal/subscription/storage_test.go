package subscription

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"navo/internal/compiler"
	"navo/internal/credential"
)

func TestManagerPersistsOutbounds(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "subscriptions.json")
	credentials := credential.NewMemoryStore()
	manager, err := newManagerWithProtector(storePath, credentials, identityProtector, identityProtector)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := manager.Add("Provider", "https://example.com/sub")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	manager.mu.Lock()
	manager.outbounds = []compiler.Outbound{{
		ID: "node", Name: "Node", Type: compiler.OutboundShadowsocks,
		Server: "example.com", Port: 443, ProviderID: sub.ID, Enabled: true,
		Password: "must-not-be-plaintext",
	}}
	err = manager.saveLocked()
	manager.mu.Unlock()
	if err != nil {
		t.Fatalf("saveLocked() error: %v", err)
	}

	reloaded, err := newManagerWithProtector(storePath, credentials, identityProtector, identityProtector)
	if err != nil {
		t.Fatal(err)
	}
	outbounds := reloaded.Outbounds()
	if len(outbounds) != 1 || outbounds[0].ProviderID != sub.ID {
		t.Fatalf("persisted outbounds = %#v", outbounds)
	}
	metadata, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metadata), "must-not-be-plaintext") ||
		strings.Contains(string(metadata), `"outbounds"`) {
		t.Fatal("subscription metadata exposed endpoint credentials")
	}
	if _, err := os.Stat(storePath + ".endpoints.dpapi"); err != nil {
		t.Fatalf("encrypted endpoint cache was not created: %v", err)
	}
}

func TestManagerLoadsLegacySubscriptionArray(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "subscriptions.json")
	if err := os.WriteFile(
		storePath,
		[]byte(`[{"id":"legacy","name":"Legacy","url":"https://example.com/sub","enabled":true}]`),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	manager := NewManagerWithPath(storePath)
	subs := manager.List()
	if len(subs) != 1 || subs[0].ID != "legacy" {
		t.Fatalf("legacy subscriptions = %#v", subs)
	}
}

func TestRemoveDeletesProviderOutbounds(t *testing.T) {
	manager, err := newManagerWithProtector(
		filepath.Join(t.TempDir(), "subscriptions.json"), credential.NewMemoryStore(),
		identityProtector, identityProtector,
	)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := manager.Add("Provider", "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.outbounds = []compiler.Outbound{
		{ID: "owned", ProviderID: sub.ID},
		{ID: "other", ProviderID: "other-provider"},
	}
	manager.mu.Unlock()

	removed, err := manager.Remove(sub.ID)
	if err != nil || !removed {
		t.Fatalf("Remove() = %v, %v", removed, err)
	}
	outbounds := manager.Outbounds()
	if len(outbounds) != 1 || outbounds[0].ID != "other" {
		t.Fatalf("remaining outbounds = %#v", outbounds)
	}
}

func identityProtector(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}

func TestNormalizerCreatesUniqueStableIDs(t *testing.T) {
	normalizer := NewNormalizer()
	first := normalizer.Merge(nil, []compiler.Outbound{
		{Name: "Same", Server: "one.example", Port: 443, Type: compiler.OutboundTrojan},
		{Name: "Same", Server: "two.example", Port: 443, Type: compiler.OutboundTrojan},
	})
	if first[0].ID == first[1].ID {
		t.Fatalf("duplicate outbound ID %q", first[0].ID)
	}

	reordered := normalizer.Merge(first, []compiler.Outbound{
		{Name: "Same", Server: "two.example", Port: 443, Type: compiler.OutboundTrojan},
		{Name: "Same", Server: "one.example", Port: 443, Type: compiler.OutboundTrojan},
	})
	ids := map[string]string{}
	for _, outbound := range reordered {
		ids[outbound.Server] = outbound.ID
	}
	for _, outbound := range first {
		if ids[outbound.Server] != outbound.ID {
			t.Fatalf(
				"ID for %s changed from %q to %q",
				outbound.Server,
				outbound.ID,
				ids[outbound.Server],
			)
		}
	}
}

func TestNormalizerKeepsCredentialDistinctEndpoints(t *testing.T) {
	normalizer := NewNormalizer()
	result := normalizer.Normalize([]compiler.Outbound{
		{Name: "A", Server: "edge.example", Port: 443, Type: compiler.OutboundTrojan, Password: "first"},
		{Name: "B", Server: "edge.example", Port: 443, Type: compiler.OutboundTrojan, Password: "second"},
	})
	if len(result) != 2 {
		t.Fatalf("credential-distinct nodes were collapsed: %#v", result)
	}
}

func TestParseBase64MixedSubscription(t *testing.T) {
	body := strings.Join([]string{
		"ss://Y2hhY2hhMjAtaWV0Zi1wb2x5MTMwNTpwYXNz@1.2.3.4:8388#SS",
		"trojan://password@5.6.7.8:443?sni=example.com#Trojan",
	}, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(body))

	outbounds := NewManager().parseContent([]byte(encoded))
	if len(outbounds) != 2 {
		t.Fatalf("parsed %d outbounds, want 2: %#v", len(outbounds), outbounds)
	}
	var trojan *compiler.Outbound
	for i := range outbounds {
		if outbounds[i].Type == compiler.OutboundTrojan {
			trojan = &outbounds[i]
			break
		}
	}
	if trojan == nil || !trojan.TLS {
		t.Fatalf("Trojan outbound was not parsed with TLS: %#v", outbounds)
	}
}
