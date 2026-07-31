package coremanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"navo/internal/domain/core"
)

type Manifest struct {
	SchemaVersion int            `json:"schema_version"`
	Cores         []Installation `json:"cores"`
}

type Installation struct {
	Type           core.Type `json:"type"`
	Version        string    `json:"version"`
	RelativePath   string    `json:"relative_path"`
	SHA256         string    `json:"sha256"`
	ConfigFormat   string    `json:"config_format"`
	VersionArgs    []string  `json:"version_args"`
	ValidationArgs []string  `json:"validation_args"`
	RunArgs        []string  `json:"run_args"`
}

func Load(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open core manifest: %w", err)
	}
	defer file.Close()

	var manifest Manifest
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode core manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported core manifest schema %d", m.SchemaVersion)
	}
	seen := make(map[core.Type]bool, len(m.Cores))
	for index, installation := range m.Cores {
		if !installation.Type.Valid() {
			return fmt.Errorf("core[%d] has invalid type %q", index, installation.Type)
		}
		if seen[installation.Type] {
			return fmt.Errorf("core manifest contains duplicate %s", installation.Type)
		}
		seen[installation.Type] = true
		if strings.TrimSpace(installation.Version) == "" {
			return fmt.Errorf("%s version is empty", installation.Type)
		}
		if filepath.IsAbs(installation.RelativePath) || strings.TrimSpace(installation.RelativePath) == "" {
			return fmt.Errorf("%s relative path is invalid", installation.Type)
		}
		if len(installation.SHA256) != 64 {
			return fmt.Errorf("%s SHA-256 is invalid", installation.Type)
		}
		switch installation.ConfigFormat {
		case "json":
		case "yaml":
			if installation.Type != core.TypeMihomo {
				return fmt.Errorf("%s cannot use YAML config", installation.Type)
			}
		default:
			return fmt.Errorf("%s config format is invalid", installation.Type)
		}
		if len(installation.VersionArgs) == 0 || len(installation.ValidationArgs) == 0 || len(installation.RunArgs) == 0 {
			return fmt.Errorf("%s command metadata is incomplete", installation.Type)
		}
	}
	for _, coreType := range core.All() {
		if !seen[coreType] {
			return fmt.Errorf("core manifest is missing %s", coreType)
		}
	}
	return nil
}

func (m Manifest) VerifyFiles(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve manifest root: %w", err)
	}
	for _, installation := range m.Cores {
		path, err := safeJoin(root, installation.RelativePath)
		if err != nil {
			return fmt.Errorf("%s: %w", installation.Type, err)
		}
		actual, err := fileHash(path)
		if err != nil {
			return fmt.Errorf("%s binary: %w", installation.Type, err)
		}
		if !strings.EqualFold(actual, installation.SHA256) {
			return fmt.Errorf("%s binary SHA-256 mismatch", installation.Type)
		}
	}
	return nil
}

func (m Manifest) Find(coreType core.Type) (Installation, bool) {
	for _, installation := range m.Cores {
		if installation.Type == coreType {
			return installation, true
		}
	}
	return Installation{}, false
}

func safeJoin(root, relative string) (string, error) {
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("binary path escapes the application root")
	}
	return path, nil
}

func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
