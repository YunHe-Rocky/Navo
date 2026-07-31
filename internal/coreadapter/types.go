package coreadapter

import (
	"context"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
	"navo/internal/domain/selection"
)

type Version struct {
	Raw   string `json:"raw"`
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	Patch int    `json:"patch"`
}

type CapabilitySet struct {
	Protocols    map[endpoint.Protocol]bool `json:"protocols"`
	CaptureModes map[capture.Mode]bool      `json:"captureModes"`
	HotReload    bool                       `json:"hotReload"`
	Controller   bool                       `json:"controller"`
	Metrics      bool                       `json:"metrics"`
}

type PortPlan struct {
	MixedPort      int `json:"mixedPort"`
	HTTPPort       int `json:"httpPort"`
	SOCKSPort      int `json:"socksPort"`
	ControllerPort int `json:"controllerPort"`
}

type CompileRequest struct {
	Selection    selection.ActiveSelection
	Config       *compiler.Config
	PortPlan     PortPlan
	RuntimeDir   string
	RevisionID   string
	Disconnected bool
}

type CompiledConfig struct {
	CoreType         core.Type `json:"coreType"`
	RevisionID       string    `json:"revisionId"`
	MainConfigPath   string    `json:"mainConfigPath"`
	WorkingDir       string    `json:"workingDir"`
	ContentHash      string    `json:"contentHash"`
	RedactedView     []byte    `json:"redactedView"`
	SensitiveFiles   []string  `json:"sensitiveFiles"`
	ControllerURL    string    `json:"-"`
	ControllerSecret string    `json:"-"`
}

type CoreInstallation struct {
	Type       core.Type
	BinaryPath string
	WorkingDir string
	Version    Version
}

type ValidationResult struct {
	Valid  bool
	Output string
	Err    error
}

type LaunchSpec struct {
	BinaryPath string
	Args       []string
	WorkingDir string
}

type RuntimeInfo struct {
	PID              int
	ProcessRunning   bool
	ListenPorts      []int
	ControllerURL    string
	ControllerSecret string
	StartedAt        time.Time
}

type HealthResult struct {
	Healthy      bool
	ProcessOK    bool
	PortsOK      bool
	ControllerOK bool
	Latency      time.Duration
	CheckedAt    time.Time
	Error        string
}

type Metrics struct {
	UploadBytes   uint64
	DownloadBytes uint64
	Connections   int
}

type MetricsReader interface {
	Read(ctx context.Context) (Metrics, error)
}

type CoreAdapter interface {
	Type() core.Type
	BinaryName() string
	DetectVersion(ctx context.Context, binaryPath string) (Version, error)
	Capabilities(version Version) CapabilitySet
	Compile(ctx context.Context, request CompileRequest) (CompiledConfig, error)
	Validate(ctx context.Context, installation CoreInstallation, config CompiledConfig) ValidationResult
	BuildLaunchSpec(installation CoreInstallation, config CompiledConfig) (LaunchSpec, error)
	HealthProbe(ctx context.Context, runtime RuntimeInfo) HealthResult
	MetricsReader(runtime RuntimeInfo) MetricsReader
}
