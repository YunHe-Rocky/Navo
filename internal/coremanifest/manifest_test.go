package coremanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"navo/internal/domain/core"
)

func TestRepositoryManifestIsValid(t *testing.T) {
	path := filepath.Join("..", "..", "CORE_MANIFEST.json")
	manifest, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, coreType := range core.All() {
		if _, ok := manifest.Find(coreType); !ok {
			t.Fatalf("manifest missing %s", coreType)
		}
	}
}

func TestManifestRejectsEscapingPath(t *testing.T) {
	t.Parallel()
	manifest := validManifest()
	manifest.Cores[0].RelativePath = "../escape.exe"
	if err := manifest.VerifyFiles(t.TempDir()); err == nil {
		t.Fatal("escaping path accepted")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data, _ := json.Marshal(validManifest())
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	raw["unexpected"] = true
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func validManifest() Manifest {
	installations := make([]Installation, 0, 3)
	for _, coreType := range core.All() {
		format := "json"
		if coreType == core.TypeMihomo {
			format = "yaml"
		}
		installations = append(installations, Installation{
			Type: coreType, Version: "1.2.3", RelativePath: "cores/" + coreType.String() + ".exe",
			SHA256:       "0000000000000000000000000000000000000000000000000000000000000000",
			ConfigFormat: format, VersionArgs: []string{"version"},
			ValidationArgs: []string{"check"}, RunArgs: []string{"run"},
		})
	}
	return Manifest{SchemaVersion: 1, Cores: installations}
}
