package initialization

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"navo/internal/fsatomic"
)

func cleanupLegacyEnvironment(dataDir string) error {
	path := filepath.Join(dataDir, ".env")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy environment: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		key := environmentKey(line)
		if isLegacyEnvironmentKey(key) {
			changed = true
			continue
		}
		kept = append(kept, line)
	}
	if !changed {
		return nil
	}
	return fsatomic.WriteFile(path, []byte(strings.Join(kept, "\r\n")), 0600)
}

func environmentKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	key, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(key))
}

func isLegacyEnvironmentKey(key string) bool {
	return strings.HasPrefix(key, "NAVO_MYSQL_") ||
		strings.HasPrefix(key, "NAVO_AI_") ||
		key == "DATABASE_URL" || key == "DSN"
}
