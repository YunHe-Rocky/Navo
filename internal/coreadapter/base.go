package coreadapter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
	"navo/internal/winprocess"
)

var semanticVersionPattern = regexp.MustCompile(`(?i)\bv?(\d+)\.(\d+)\.(\d+)(?:[-+][0-9A-Za-z.-]+)?\b`)

type adapterSpec struct {
	coreType        core.Type
	binaryName      string
	versionArgs     []string
	extension       string
	compile         func(*compiler.Config) ([]byte, error)
	validateArgs    func(string) []string
	runArgs         func(string) []string
	capabilities    CapabilitySet
	controllerProbe bool
}

func detectVersion(ctx context.Context, binaryPath string, args []string) (Version, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return Version{}, fmt.Errorf("binary path is required")
	}
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	winprocess.ConfigureHidden(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Version{}, fmt.Errorf("detect version: %s: %w", strings.TrimSpace(string(output)), err)
	}
	match := semanticVersionPattern.FindStringSubmatch(string(output))
	if len(match) != 4 {
		return Version{}, fmt.Errorf("version not found in output")
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return Version{Raw: match[0], Major: major, Minor: minor, Patch: patch}, nil
}

func compileNative(ctx context.Context, spec adapterSpec, request CompileRequest) (CompiledConfig, error) {
	if err := ctx.Err(); err != nil {
		return CompiledConfig{}, err
	}
	if !request.Disconnected {
		if err := request.Selection.Validate(); err != nil {
			return CompiledConfig{}, fmt.Errorf("invalid active selection: %w", err)
		}
	} else if request.Config == nil || request.Config.FinalOutbound != "direct" {
		return CompiledConfig{}, fmt.Errorf("disconnected compilation must route directly")
	}
	if request.Config == nil {
		return CompiledConfig{}, fmt.Errorf("compiler config is required")
	}
	if strings.TrimSpace(request.RuntimeDir) == "" || strings.TrimSpace(request.RevisionID) == "" {
		return CompiledConfig{}, fmt.Errorf("runtime directory and revision ID are required")
	}
	if validation := compiler.Validate(request.Config); !validation.Valid {
		return CompiledConfig{}, fmt.Errorf("canonical config validation failed: %v", validation.Errors)
	}

	config := request.Config
	if spec.coreType == core.TypeMihomo || spec.coreType == core.TypeSingBox {
		controllerPort := request.PortPlan.ControllerPort
		if controllerPort == 0 {
			for _, inbound := range config.Inbounds {
				if inbound.ListenPort > 0 {
					controllerPort = inbound.ListenPort + 1000
					break
				}
			}
		}
		if controllerPort < 1 || controllerPort > 65535 {
			if spec.coreType == core.TypeMihomo {
				return CompiledConfig{}, fmt.Errorf("valid metrics controller port is required")
			}
		} else {
			secret, err := randomSecret()
			if err != nil {
				return CompiledConfig{}, fmt.Errorf("generate metrics controller secret: %w", err)
			}
			copy := *config
			copy.Controller = &compiler.ControllerConfig{
				Listen: "127.0.0.1", Port: controllerPort, Secret: secret,
			}
			config = &copy
		}
	}
	content, err := spec.compile(config)
	if err != nil {
		return CompiledConfig{}, fmt.Errorf("compile %s config: %w", spec.coreType, err)
	}
	if err := os.MkdirAll(request.RuntimeDir, 0700); err != nil {
		return CompiledConfig{}, fmt.Errorf("create runtime directory: %w", err)
	}
	configPath := filepath.Join(request.RuntimeDir, request.RevisionID+spec.extension)
	if err := writeFileAtomically(configPath, content, 0600); err != nil {
		return CompiledConfig{}, fmt.Errorf("write compiled config: %w", err)
	}
	hash := sha256.Sum256(content)
	result := CompiledConfig{
		CoreType: spec.coreType, RevisionID: request.RevisionID,
		MainConfigPath: configPath, WorkingDir: request.RuntimeDir,
		ContentHash:    hex.EncodeToString(hash[:]),
		RedactedView:   []byte(`{"redacted":true}`),
		SensitiveFiles: []string{configPath},
	}
	if config.Controller != nil {
		result.ControllerURL = fmt.Sprintf(
			"http://%s:%d",
			config.Controller.Listen,
			config.Controller.Port,
		)
		result.ControllerSecret = config.Controller.Secret
	}
	return result, nil
}

func validateNative(
	ctx context.Context,
	spec adapterSpec,
	installation CoreInstallation,
	config CompiledConfig,
) ValidationResult {
	if installation.Type != spec.coreType || config.CoreType != spec.coreType {
		err := fmt.Errorf("adapter/core type mismatch")
		return ValidationResult{Err: err, Output: err.Error()}
	}
	cmd := exec.CommandContext(ctx, installation.BinaryPath, spec.validateArgs(config.MainConfigPath)...)
	cmd.Dir = defaultWorkingDir(installation)
	winprocess.ConfigureHidden(cmd)
	output, err := cmd.CombinedOutput()
	return ValidationResult{
		Valid: err == nil, Output: truncateOutput(strings.TrimSpace(string(output)), 8192), Err: err,
	}
}

func buildLaunchSpec(
	spec adapterSpec,
	installation CoreInstallation,
	config CompiledConfig,
) (LaunchSpec, error) {
	if installation.Type != spec.coreType || config.CoreType != spec.coreType {
		return LaunchSpec{}, fmt.Errorf("adapter/core type mismatch")
	}
	if installation.BinaryPath == "" || config.MainConfigPath == "" {
		return LaunchSpec{}, fmt.Errorf("binary path and main config path are required")
	}
	return LaunchSpec{
		BinaryPath: installation.BinaryPath,
		Args:       spec.runArgs(config.MainConfigPath),
		WorkingDir: defaultWorkingDir(installation),
	}, nil
}

func probeRuntime(ctx context.Context, runtime RuntimeInfo, requireController bool) HealthResult {
	result := HealthResult{CheckedAt: time.Now().UTC(), ProcessOK: runtime.ProcessRunning}
	if !runtime.ProcessRunning || runtime.PID <= 0 {
		result.Error = "selected core process is not running"
		return result
	}
	if len(runtime.ListenPorts) == 0 {
		result.Error = "no readiness ports were declared"
		return result
	}
	started := time.Now()
	for _, port := range runtime.ListenPorts {
		if port < 1 || port > 65535 {
			result.Error = fmt.Sprintf("invalid readiness port %d", port)
			return result
		}
		dialer := net.Dialer{Timeout: 2 * time.Second}
		connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			result.Error = fmt.Sprintf("readiness port %d is not listening: %v", port, err)
			return result
		}
		connection.Close()
	}
	result.PortsOK = true
	if requireController {
		if strings.TrimSpace(runtime.ControllerURL) == "" {
			result.Error = "controller URL is required"
			return result
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(runtime.ControllerURL, "/")+"/version", nil)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if runtime.ControllerSecret != "" {
			request.Header.Set("Authorization", "Bearer "+runtime.ControllerSecret)
		}
		response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
		if err != nil {
			result.Error = fmt.Sprintf("controller is not ready: %v", err)
			return result
		}
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			result.Error = fmt.Sprintf("controller returned HTTP %d", response.StatusCode)
			return result
		}
		result.ControllerOK = true
	} else {
		result.ControllerOK = true
	}
	result.Latency = time.Since(started)
	result.Healthy = result.ProcessOK && result.PortsOK && result.ControllerOK
	return result
}

func allCapabilities(protocols ...endpoint.Protocol) map[endpoint.Protocol]bool {
	result := make(map[endpoint.Protocol]bool, len(protocols))
	for _, protocol := range protocols {
		result[protocol] = true
	}
	return result
}

func captureCapabilities(tun bool) map[capture.Mode]bool {
	return map[capture.Mode]bool{
		capture.ModeOff: true, capture.ModeSystemProxy: true, capture.ModeTUN: tun,
	}
}

func randomSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func defaultWorkingDir(installation CoreInstallation) string {
	if installation.WorkingDir != "" {
		return installation.WorkingDir
	}
	return filepath.Dir(installation.BinaryPath)
}

func truncateOutput(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...[truncated]"
}

func writeFileAtomically(path string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".navo-config-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
