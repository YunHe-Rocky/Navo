package initialization

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resetForeignContext(dataDir string) error {
	exact := []string{
		"credentials.dpapi", "subscriptions.json", "upstream_proxies.json",
		"runtime.json", "runtime_state.json", "tun_network_journal.json",
		"sing-box.log", "ai-settings.json",
		filepath.Join("state", "repositories.json"),
		filepath.Join("state", "selfheal-state.json"),
		filepath.Join("agent", "capture_transition.json"),
		filepath.Join("agent", "proxy_backup.json"),
		filepath.Join("agent", "proxy_owner.json"),
	}
	var failures []error
	for _, relative := range exact {
		if err := removeControlled(dataDir, relative); err != nil {
			failures = append(failures, err)
		}
	}
	for _, pattern := range []string{"runtime.*.json", "*.tmp", "*.bak"} {
		matches, err := filepath.Glob(filepath.Join(dataDir, pattern))
		if err != nil {
			failures = append(failures, fmt.Errorf("expand privacy pattern %q: %w", pattern, err))
			continue
		}
		for _, match := range matches {
			if err := removeAbsoluteControlled(dataDir, match); err != nil {
				failures = append(failures, err)
			}
		}
	}
	for _, relative := range []string{"log"} {
		if err := removeControlled(dataDir, relative); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("privacy reset incomplete: %w", errors.Join(failures...))
	}
	return nil
}

func removeControlled(root, relative string) error {
	return removeAbsoluteControlled(root, filepath.Join(root, relative))
}

func removeAbsoluteControlled(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return fmt.Errorf("privacy target escapes data directory: %q", target)
	}
	if err := os.RemoveAll(targetAbs); err != nil {
		return fmt.Errorf("remove privacy target %q: %w", relative, err)
	}
	return nil
}
