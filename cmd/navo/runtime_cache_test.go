package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanRuntimeCachePreservesRuntimeState(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"runtime.123.json":   `{"generated":true}`,
		"runtime_state.json": `{"core_id":"mihomo"}`,
		"subscriptions.json": `[]`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cleanRuntimeCache(dir)

	if _, err := os.Stat(filepath.Join(dir, "runtime.123.json")); !os.IsNotExist(err) {
		t.Fatalf("generated runtime config still exists: %v", err)
	}
	for _, name := range []string{"runtime_state.json", "subscriptions.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
}
