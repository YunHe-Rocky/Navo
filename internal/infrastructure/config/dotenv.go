package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const maxDotEnvBytes = 1 << 20

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// LoadDotEnv loads the first available .env file. Existing process variables
// always win, which keeps service-manager and CI configuration authoritative.
func LoadDotEnv(executableDir string) (string, error) {
	if explicit, ok := os.LookupEnv("NAVO_ENV_FILE"); ok {
		path := strings.TrimSpace(explicit)
		if path == "" {
			return "", fmt.Errorf("NAVO_ENV_FILE is empty")
		}
		if err := loadDotEnvFile(path); err != nil {
			return "", fmt.Errorf("load NAVO_ENV_FILE: %w", err)
		}
		return path, nil
	}

	candidates := make([]string, 0, 2)
	if executableDir != "" {
		candidates = append(candidates, filepath.Join(executableDir, ".env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		path := filepath.Join(cwd, ".env")
		if len(candidates) == 0 || !samePath(candidates[0], path) {
			candidates = append(candidates, path)
		}
	}

	for _, path := range candidates {
		err := loadDotEnvFile(path)
		switch {
		case err == nil:
			return path, nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return "", fmt.Errorf("load %s: %w", path, err)
		}
	}
	return "", nil
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := io.LimitReader(file, maxDotEnvBytes+1)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxDotEnvBytes+1)

	total := 0
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		total += len(line) + 1
		if total > maxDotEnvBytes {
			return fmt.Errorf("file exceeds %d bytes", maxDotEnvBytes)
		}
		key, value, ok, err := parseDotEnvLine(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return nil
}

func parseDotEnvLine(line string) (key, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, raw, found := strings.Cut(line, "=")
	if !found {
		return "", "", false, fmt.Errorf("missing =")
	}
	key = strings.TrimSpace(key)
	if !envKeyPattern.MatchString(key) {
		return "", "", false, fmt.Errorf("invalid variable name %q", key)
	}
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		value, err = strconv.Unquote(raw)
		if err != nil {
			return "", "", false, fmt.Errorf("invalid quoted value: %w", err)
		}
		return key, value, true, nil
	}
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return key, raw[1 : len(raw)-1], true, nil
	}
	if comment := strings.Index(raw, " #"); comment >= 0 {
		raw = strings.TrimSpace(raw[:comment])
	}
	return key, raw, true, nil
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
