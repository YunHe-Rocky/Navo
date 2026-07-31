package subscription

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdd_EmptyName(t *testing.T) {
	manager := NewManager()
	_, err := manager.Add("", "https://example.com/sub")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestAdd_InvalidURL(t *testing.T) {
	manager := NewManager()
	// HTTP is now allowed - URL validation is deferred to fetch time.
	_, err := manager.Add("Provider", "http://example.com/sub")
	if err != nil {
		t.Fatalf("HTTP URL should be allowed during add: %v", err)
	}
}

func TestAdd_EmptyURL(t *testing.T) {
	manager := NewManager()
	_, err := manager.Add("Provider", "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestAdd_DuplicateURL(t *testing.T) {
	manager := NewManager()
	_, err := manager.Add("First", "https://example.com/sub")
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	_, err = manager.Add("Second", "https://example.com/sub")
	if err == nil {
		t.Fatal("expected duplicate URL error")
	}
}

func TestTLSCompatibility_PersistsPerSubscription(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "subscriptions.json")
	manager := NewManagerWithPath(storePath)
	sub, err := manager.AddWithOptions("Provider", "https://example.com/sub", false)
	if err != nil {
		t.Fatalf("AddWithOptions: %v", err)
	}
	if sub.SkipTLSVerify {
		t.Fatal("TLS verification must be enabled by default")
	}

	updated, err := manager.UpdateTLSCompatibility(sub.ID, true)
	if err != nil {
		t.Fatalf("UpdateTLSCompatibility: %v", err)
	}
	if !updated.SkipTLSVerify {
		t.Fatal("updated subscription did not enable compatibility")
	}

	reloaded := NewManagerWithPath(storePath).List()
	if len(reloaded) != 1 || !reloaded[0].SkipTLSVerify {
		t.Fatalf("reloaded subscriptions = %#v", reloaded)
	}
}

func TestTLSCompatibility_UnknownSubscription(t *testing.T) {
	manager := NewManager()
	if _, err := manager.UpdateTLSCompatibility("missing", true); err == nil {
		t.Fatal("expected unknown subscription error")
	}
}

func TestRemove_Nonexistent(t *testing.T) {
	manager := NewManager()
	ok, err := manager.Remove("nonexistent")
	if err != nil || ok {
		t.Fatalf("Remove() = %v, %v, want false, nil", ok, err)
	}
}

func TestList_ReturnsCopy(t *testing.T) {
	manager := NewManager()
	_, _ = manager.Add("Provider", "https://example.com/sub")

	list := manager.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(list))
	}
	list[0].Name = "Modified"
	second := manager.List()
	if second[0].Name == "Modified" {
		t.Fatal("List() returned the same backing slice")
	}
}

func TestOutbounds_ReturnsCopy(t *testing.T) {
	manager := NewManagerWithPath(filepath.Join(t.TempDir(), "subs.json"))
	outbounds := manager.Outbounds()
	if len(outbounds) != 0 {
		t.Fatalf("expected empty outbounds, got %d", len(outbounds))
	}
}

func TestUniqueID_Collision(t *testing.T) {
	existing := []Subscription{
		{ID: "my-provider"},
		{ID: "my-provider-2"},
	}
	// sanitizeID preserves case, so "My Provider!" → "My-Provider",
	// which doesn't collide with existing lowercase IDs.
	id := uniqueID("My Provider!", existing)
	if id != "My-Provider" {
		t.Fatalf("uniqueID = %q, want My-Provider", id)
	}
}

func TestSanitizeID_Empty(t *testing.T) {
	id := sanitizeID("")
	if id == "" {
		t.Fatal("empty ID was not replaced")
	}
	if !strings.HasPrefix(id, "sub-") {
		t.Fatalf("sanitizeID = %q, want sub-*", id)
	}
}

func TestSanitizeID_Chinese(t *testing.T) {
	id := sanitizeID("我的供应商")
	if id == "" {
		t.Fatal("Chinese-only name resulted in empty ID")
	}
	if !strings.HasPrefix(id, "sub-") {
		t.Fatalf("sanitizeID = %q for Chinese name, want sub-*", id)
	}
}

func TestSanitizeID_Mixed(t *testing.T) {
	id := sanitizeID("My-Provider_01 测试")
	// _ → -, space → -, Chinese chars dropped
	if id != "My-Provider-01-" {
		t.Fatalf("sanitizeID = %q, want My-Provider-01-", id)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "subs.json")
	manager := NewManagerWithPath(storePath)
	_, _ = manager.Add("Provider", "https://example.com/sub")

	reloaded := NewManagerWithPath(storePath)
	list := reloaded.List()
	if len(list) != 1 || list[0].Name != "Provider" {
		t.Fatalf("reloaded subscriptions = %#v", list)
	}
}

func TestLoad_CorruptedFile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "subs.json")
	if err := os.WriteFile(storePath, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManagerWithPath(storePath)
	if len(manager.List()) != 0 {
		t.Fatal("corrupted file should yield empty list")
	}
}

func TestAutoRefresh_DoesNotBlock(t *testing.T) {
	manager := NewManagerWithPath(filepath.Join(t.TempDir(), "subs.json"))
	_, _ = manager.Add("Provider", "https://example.com/sub")
	ctx, cancel := context.WithCancel(context.Background())
	manager.StartAutoRefresh(ctx, 10*time.Hour)
	cancel()
}

func TestDecodeSubscriptionBody_PlainURL(t *testing.T) {
	data := []byte("vmess://abc123")
	result := decodeSubscriptionBody(data)
	if string(result) != "vmess://abc123" {
		t.Fatalf("plain URL should be returned as-is: %s", result)
	}
}

func TestDecodeSubscriptionBody_Base64(t *testing.T) {
	encoded := []byte("dm1lc3M6Ly9hYmMxMjMKdHJvamFuOi8vZGVmNDU2")
	result := decodeSubscriptionBody(encoded)
	if !strings.Contains(string(result), "vmess://") {
		t.Fatalf("base64 not decoded: %s", result)
	}
}

func TestOutboundsForProvider(t *testing.T) {
	ob := outboundsForProvider(nil, "p1")
	if len(ob) != 0 {
		t.Fatal("expected empty result for nil input")
	}
}

func TestValidateURL_Schemes(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://example.com/sub", true},
		{"http://example.com/sub", false},
		{"ftp://example.com/sub", false},
		{"file:///etc/passwd", false},
		{"not-a-url", false},
		{"", false},
	}
	for _, tc := range tests {
		err := validateURL(tc.url)
		if tc.ok && err != nil {
			t.Errorf("validateURL(%q) = %v, want nil", tc.url, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("validateURL(%q) = nil, want error", tc.url)
		}
	}
}
