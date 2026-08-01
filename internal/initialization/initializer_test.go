package initialization

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func identity(data []byte) ([]byte, error) { return append([]byte(nil), data...), nil }

func TestRunFirstAndSameContext(t *testing.T) {
	dir := t.TempDir()
	options := Options{Protect: identity, Unprotect: identity}
	first, err := RunWithOptions(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	if !first.FirstRun || !first.Ready || first.PrivacyReset {
		t.Fatalf("first result = %#v", first)
	}
	second, err := RunWithOptions(dir, options)
	if err != nil {
		t.Fatal(err)
	}
	if second.FirstRun || second.ForeignContext || !second.Ready {
		t.Fatalf("second result = %#v", second)
	}
}

func TestForeignContextClearsSensitiveStateAndOldJournal(t *testing.T) {
	dir := t.TempDir()
	if _, err := RunWithOptions(dir, Options{Protect: identity, Unprotect: identity}); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		"subscriptions.json", "credentials.dpapi", "tun_network_journal.json",
		filepath.Join("state", "repositories.json"),
		filepath.Join("agent", "proxy_owner.json"),
	} {
		path := filepath.Join(dir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("sensitive"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := RunWithOptions(dir, Options{
		Protect:   identity,
		Unprotect: func([]byte) ([]byte, error) { return nil, errors.New("different user") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ForeignContext || !result.PrivacyReset || !result.Ready {
		t.Fatalf("result = %#v", result)
	}
	for _, relative := range []string{"subscriptions.json", "tun_network_journal.json", filepath.Join("agent", "proxy_owner.json")} {
		if _, err := os.Stat(filepath.Join(dir, relative)); !os.IsNotExist(err) {
			t.Fatalf("privacy target still exists: %s", relative)
		}
	}
}

func TestTamperedDeviceStateBlocksStartupWithoutCleanup(t *testing.T) {
	dir := t.TempDir()
	options := Options{Protect: identity, Unprotect: identity}
	if _, err := RunWithOptions(dir, options); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "device-state.dat")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state deviceState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state.Checksum = "00"
	data, _ = json.Marshal(state)
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	sensitive := filepath.Join(dir, "subscriptions.json")
	if err := os.WriteFile(sensitive, []byte("keep-unread"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := RunWithOptions(dir, options)
	if err == nil || result.ErrorCode != ErrorDeviceStateInvalid || result.Ready {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if _, err := os.Stat(sensitive); err != nil {
		t.Fatalf("tamper path unexpectedly cleaned: %v", err)
	}
}

func TestLegacyEnvironmentCleanupPreservesUnrelatedKeys(t *testing.T) {
	dir := t.TempDir()
	content := "NAVO_MYSQL_HOST=old\nNAVO_AI_API_KEY=old\nNAVO_PROXY_PORT=12080\n# keep\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWithOptions(dir, Options{Protect: identity, Unprotect: identity}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "NAVO_PROXY_PORT=12080\r\n# keep\r\n" {
		t.Fatalf("cleaned environment = %q", data)
	}
}
