package systemproxy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnableFailsClosedWhenWinINetSnapshotCannotBeRead(t *testing.T) {
	manager := NewManagerWithDirectory(t.TempDir())
	manager.getProxy = func() (*ProxyConfig, error) {
		return nil, errors.New("registry unavailable")
	}
	setCalled := false
	manager.setProxy = func(string) error {
		setCalled = true
		return nil
	}
	if err := manager.Enable("127.0.0.1:12080"); err == nil {
		t.Fatal("enable succeeded without a complete WinINet snapshot")
	}
	if setCalled {
		t.Fatal("WinINet was mutated after snapshot failure")
	}
}

func TestEnableCommitsOwnershipAfterMutationAndNotification(t *testing.T) {
	manager := NewManagerWithDirectory(t.TempDir())
	manager.getProxy = func() (*ProxyConfig, error) { return &ProxyConfig{Enabled: false}, nil }
	manager.setProxy = func(string) error { return nil }
	manager.applyProxy = func(ProxyConfig) error { return nil }
	manager.notify = func() error { return nil }
	if err := manager.Enable("127.0.0.1:12080"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manager.ownerPath)
	if err != nil {
		t.Fatal(err)
	}
	var owner ownershipRecord
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.Phase != ownershipCommitted {
		t.Fatalf("ownership phase = %q", owner.Phase)
	}
}

func TestOwnershipRequiresMatchingRecord(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{ownerPath: filepath.Join(dir, "owner.json")}
	current := ProxyConfig{Enabled: true, ProxyServer: "127.0.0.1:10808"}

	if manager.owns(current) {
		t.Fatal("proxy without an ownership record must remain external")
	}
	data, err := json.Marshal(ownershipRecord{ProxyServer: "127.0.0.1:12080"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.ownerPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if manager.owns(current) {
		t.Fatal("mismatched proxy endpoint must remain external")
	}

	data, _ = json.Marshal(ownershipRecord{ProxyServer: current.ProxyServer})
	if err := os.WriteFile(manager.ownerPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if !manager.owns(current) {
		t.Fatal("matching ownership record should be recognized")
	}
}

func TestOwnershipDoesNotOverwriteNewerExternalProxy(t *testing.T) {
	owner := ownershipRecord{ProxyServer: "127.0.0.1:12080"}
	if ownershipMatchesCurrent(owner, ProxyConfig{
		Enabled: true, ProxyServer: "127.0.0.1:10808",
	}) {
		t.Fatal("a newer external proxy must not be treated as Navo-owned")
	}
	if ownershipMatchesCurrent(owner, ProxyConfig{
		Enabled: false, ProxyServer: owner.ProxyServer,
	}) {
		t.Fatal("a disabled proxy must not be treated as Navo-owned")
	}
	if !ownershipMatchesCurrent(owner, ProxyConfig{
		Enabled: true, ProxyServer: owner.ProxyServer,
	}) {
		t.Fatal("the exact enabled Navo endpoint should remain owned")
	}
}

func TestDisableRelinquishesWithoutOverwritingNewerExternalProxy(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithDirectory(dir)
	ownerData, _ := json.Marshal(ownershipRecord{ProxyServer: "127.0.0.1:12080"})
	if err := os.WriteFile(manager.ownerPath, ownerData, 0o600); err != nil {
		t.Fatal(err)
	}
	backupData, _ := json.Marshal(ProxyConfig{Enabled: false})
	if err := os.WriteFile(manager.backupPath, backupData, 0o600); err != nil {
		t.Fatal(err)
	}
	manager.getProxy = func() (*ProxyConfig, error) {
		return &ProxyConfig{Enabled: true, ProxyServer: "127.0.0.1:10808"}, nil
	}
	applied := false
	manager.applyProxy = func(ProxyConfig) error {
		applied = true
		return nil
	}
	if err := manager.Disable(); err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("Navo overwrote a newer external WinINet owner")
	}
	if _, err := os.Stat(manager.ownerPath); !os.IsNotExist(err) {
		t.Fatalf("stale ownership marker remains: %v", err)
	}
}

func TestDisableRestoresExactOwnedProxy(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithDirectory(dir)
	ownerData, _ := json.Marshal(ownershipRecord{ProxyServer: "127.0.0.1:12080"})
	if err := os.WriteFile(manager.ownerPath, ownerData, 0o600); err != nil {
		t.Fatal(err)
	}
	want := ProxyConfig{Enabled: true, ProxyServer: "127.0.0.1:10808"}
	backupData, _ := json.Marshal(want)
	if err := os.WriteFile(manager.backupPath, backupData, 0o600); err != nil {
		t.Fatal(err)
	}
	manager.getProxy = func() (*ProxyConfig, error) {
		return &ProxyConfig{Enabled: true, ProxyServer: "127.0.0.1:12080"}, nil
	}
	var restored ProxyConfig
	manager.applyProxy = func(value ProxyConfig) error {
		restored = value
		return nil
	}
	manager.notify = func() error { return nil }
	if err := manager.Disable(); err != nil {
		t.Fatal(err)
	}
	if restored != want {
		t.Fatalf("restored proxy = %#v, want %#v", restored, want)
	}
}
