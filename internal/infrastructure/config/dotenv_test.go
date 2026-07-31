package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvPreservesProcessEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NAVO_TEST_EXISTING=file\nNAVO_TEST_QUOTED=\"hello world\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAVO_TEST_EXISTING", "process")
	t.Setenv("NAVO_ENV_FILE", path)

	loaded, err := LoadDotEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path {
		t.Fatalf("loaded path = %q, want %q", loaded, path)
	}
	if got := os.Getenv("NAVO_TEST_EXISTING"); got != "process" {
		t.Fatalf("existing value overwritten: %q", got)
	}
	if got := os.Getenv("NAVO_TEST_QUOTED"); got != "hello world" {
		t.Fatalf("quoted value = %q", got)
	}
}

func TestParseDotEnvLineRejectsInvalidKey(t *testing.T) {
	t.Parallel()
	if _, _, _, err := parseDotEnvLine("NOT-VALID=value"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
