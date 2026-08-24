//go:build windows

package startup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/fsatomic"
	"navo/internal/winprocess"
)

type persistedSettings struct {
	Version        int       `json:"version"`
	Enabled        bool      `json:"enabled"`
	Mode           string    `json:"mode"`
	TaskName       string    `json:"task_name,omitempty"`
	ExecutablePath string    `json:"executable_path,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type manager struct {
	statePath      string
	executablePath string
	run            commandRunner
}

func New(statePath, executablePath string) Controller {
	return &manager{
		statePath:      statePath,
		executablePath: filepath.Clean(executablePath),
		run:            runHidden,
	}
}

func runHidden(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	winprocess.ConfigureHidden(command)
	return command.CombinedOutput()
}

func (m *manager) Status(ctx context.Context) (Settings, error) {
	stored, err := m.load()
	if err != nil {
		return Settings{}, err
	}
	status := Settings{Supported: true, Enabled: stored.Enabled, Mode: stored.Mode}
	if !stored.Enabled || stored.TaskName == "" {
		return status, nil
	}
	if err := validateOwnedTask(stored); err != nil {
		status.LastError = err.Error()
		return status, nil
	}
	output, queryErr := m.run(ctx, "schtasks.exe", "/Query", "/TN", stored.TaskName)
	if queryErr != nil {
		status.LastError = boundedTaskError("query login task", output, queryErr)
		return status, nil
	}
	status.Registered = true
	return status, nil
}

func (m *manager) Configure(ctx context.Context, enabled bool, mode string) (Settings, error) {
	mode = strings.TrimSpace(mode)
	if enabled && !ValidMode(mode) {
		return Settings{}, fmt.Errorf("startup capture mode must be system_proxy or tun")
	}
	stored, err := m.load()
	if err != nil {
		return Settings{}, err
	}
	if stored.TaskName != "" {
		if err := validateOwnedTask(stored); err != nil {
			return Settings{}, err
		}
	}
	if !enabled {
		if stored.TaskName != "" {
			output, deleteErr := m.run(ctx, "schtasks.exe", "/Delete", "/TN", stored.TaskName, "/F")
			if deleteErr != nil {
				return Settings{}, fmt.Errorf("%s", boundedTaskError("delete login task", output, deleteErr))
			}
		}
		stored = persistedSettings{Version: 1, Mode: defaultMode(mode), UpdatedAt: time.Now().UTC()}
		if err := m.save(stored); err != nil {
			return Settings{}, err
		}
		return Settings{Supported: true, Mode: stored.Mode}, nil
	}

	taskName := taskNameForPath(m.executablePath)
	if stored.TaskName != "" && stored.TaskName != taskName {
		output, deleteErr := m.run(ctx, "schtasks.exe", "/Delete", "/TN", stored.TaskName, "/F")
		if deleteErr != nil {
			return Settings{}, fmt.Errorf("%s", boundedTaskError("replace previous login task", output, deleteErr))
		}
	}
	quotedExecutable, err := quoteTaskExecutable(m.executablePath)
	if err != nil {
		return Settings{}, err
	}
	action := quotedExecutable + " --silent --startup"
	output, createErr := m.run(
		ctx, "schtasks.exe", "/Create", "/TN", taskName, "/TR", action,
		"/SC", "ONLOGON", "/DELAY", "0000:15", "/RL", "HIGHEST", "/F",
	)
	if createErr != nil {
		return Settings{}, fmt.Errorf("%s", boundedTaskError("create login task", output, createErr))
	}
	stored = persistedSettings{
		Version: 1, Enabled: true, Mode: mode, TaskName: taskName,
		ExecutablePath: m.executablePath, UpdatedAt: time.Now().UTC(),
	}
	if err := m.save(stored); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = m.run(cleanupCtx, "schtasks.exe", "/Delete", "/TN", taskName, "/F")
		cleanupCancel()
		return Settings{}, err
	}
	return Settings{Supported: true, Enabled: true, Mode: mode, Registered: true}, nil
}

func (m *manager) load() (persistedSettings, error) {
	settings := persistedSettings{Version: 1, Mode: ModeSystemProxy}
	data, err := os.ReadFile(m.statePath)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("read startup settings: %w", err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return persistedSettings{}, fmt.Errorf("decode startup settings: %w", err)
	}
	settings.Mode = defaultMode(settings.Mode)
	return settings, nil
}

func (m *manager) save(settings persistedSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode startup settings: %w", err)
	}
	if err := fsatomic.WriteFile(m.statePath, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("save startup settings: %w", err)
	}
	return nil
}

func defaultMode(mode string) string {
	if ValidMode(mode) {
		return mode
	}
	return ModeSystemProxy
}

func taskNameForPath(executablePath string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(executablePath))))
	return "Navo Startup " + hex.EncodeToString(digest[:6])
}

func quoteTaskExecutable(executablePath string) (string, error) {
	if strings.ContainsRune(executablePath, '"') {
		return "", fmt.Errorf("launcher path contains an invalid quote")
	}
	return `"` + executablePath + `"`, nil
}

func validateOwnedTask(settings persistedSettings) error {
	if settings.TaskName == "" || settings.ExecutablePath == "" ||
		settings.TaskName != taskNameForPath(settings.ExecutablePath) {
		return fmt.Errorf("refuse unowned startup task metadata")
	}
	return nil
}

func boundedTaskError(action string, output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if len(message) > 512 {
		message = message[:512]
	}
	if message == "" {
		return fmt.Sprintf("%s: %v", action, err)
	}
	return fmt.Sprintf("%s: %v: %s", action, err, message)
}
