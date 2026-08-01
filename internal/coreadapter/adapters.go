package coreadapter

import (
	"context"
	"fmt"

	"navo/internal/compiler"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
)

type SingBoxAdapter struct{}
type MihomoAdapter struct{}
type XrayAdapter struct{}

func NewSingBoxAdapter() *SingBoxAdapter { return &SingBoxAdapter{} }
func NewMihomoAdapter() *MihomoAdapter   { return &MihomoAdapter{} }
func NewXrayAdapter() *XrayAdapter       { return &XrayAdapter{} }

func (a *SingBoxAdapter) Type() core.Type    { return core.TypeSingBox }
func (a *SingBoxAdapter) BinaryName() string { return "sing-box.exe" }
func (a *SingBoxAdapter) DetectVersion(ctx context.Context, path string) (Version, error) {
	return detectVersion(ctx, path, []string{"version"})
}
func (a *SingBoxAdapter) Capabilities(Version) CapabilitySet { return singBoxSpec().capabilities }
func (a *SingBoxAdapter) Compile(ctx context.Context, request CompileRequest) (CompiledConfig, error) {
	return compileNative(ctx, singBoxSpec(), request)
}
func (a *SingBoxAdapter) Validate(ctx context.Context, i CoreInstallation, c CompiledConfig) ValidationResult {
	return validateNative(ctx, singBoxSpec(), i, c)
}
func (a *SingBoxAdapter) BuildLaunchSpec(i CoreInstallation, c CompiledConfig) (LaunchSpec, error) {
	return buildLaunchSpec(singBoxSpec(), i, c)
}
func (a *SingBoxAdapter) HealthProbe(ctx context.Context, runtime RuntimeInfo) HealthResult {
	return probeRuntime(ctx, runtime, false)
}
func (a *SingBoxAdapter) MetricsReader(runtime RuntimeInfo) MetricsReader {
	return newClashMetricsReader(runtime)
}

func (a *MihomoAdapter) Type() core.Type    { return core.TypeMihomo }
func (a *MihomoAdapter) BinaryName() string { return "mihomo.exe" }
func (a *MihomoAdapter) DetectVersion(ctx context.Context, path string) (Version, error) {
	return detectVersion(ctx, path, []string{"-v"})
}
func (a *MihomoAdapter) Capabilities(Version) CapabilitySet { return mihomoSpec().capabilities }
func (a *MihomoAdapter) Compile(ctx context.Context, request CompileRequest) (CompiledConfig, error) {
	return compileNative(ctx, mihomoSpec(), request)
}
func (a *MihomoAdapter) Validate(ctx context.Context, i CoreInstallation, c CompiledConfig) ValidationResult {
	return validateNative(ctx, mihomoSpec(), i, c)
}
func (a *MihomoAdapter) BuildLaunchSpec(i CoreInstallation, c CompiledConfig) (LaunchSpec, error) {
	return buildLaunchSpec(mihomoSpec(), i, c)
}
func (a *MihomoAdapter) HealthProbe(ctx context.Context, runtime RuntimeInfo) HealthResult {
	return probeRuntime(ctx, runtime, true)
}
func (a *MihomoAdapter) MetricsReader(runtime RuntimeInfo) MetricsReader {
	return newClashMetricsReader(runtime)
}

func (a *XrayAdapter) Type() core.Type    { return core.TypeXray }
func (a *XrayAdapter) BinaryName() string { return "xray.exe" }
func (a *XrayAdapter) DetectVersion(ctx context.Context, path string) (Version, error) {
	return detectVersion(ctx, path, []string{"version"})
}
func (a *XrayAdapter) Capabilities(Version) CapabilitySet { return xraySpec().capabilities }
func (a *XrayAdapter) Compile(ctx context.Context, request CompileRequest) (CompiledConfig, error) {
	return compileNative(ctx, xraySpec(), request)
}
func (a *XrayAdapter) Validate(ctx context.Context, i CoreInstallation, c CompiledConfig) ValidationResult {
	return validateNative(ctx, xraySpec(), i, c)
}
func (a *XrayAdapter) BuildLaunchSpec(i CoreInstallation, c CompiledConfig) (LaunchSpec, error) {
	return buildLaunchSpec(xraySpec(), i, c)
}
func (a *XrayAdapter) HealthProbe(ctx context.Context, runtime RuntimeInfo) HealthResult {
	return probeRuntime(ctx, runtime, false)
}
func (a *XrayAdapter) MetricsReader(RuntimeInfo) MetricsReader { return nil }

func singBoxSpec() adapterSpec {
	return adapterSpec{
		coreType: core.TypeSingBox, binaryName: "sing-box.exe",
		versionArgs: []string{"version"}, extension: ".json",
		compile:      compiler.Generate,
		validateArgs: func(path string) []string { return []string{"check", "-c", path} },
		runArgs:      func(path string) []string { return []string{"run", "-c", path} },
		capabilities: CapabilitySet{
			Protocols: allCapabilities(
				endpoint.ProtocolVLESS, endpoint.ProtocolVMess, endpoint.ProtocolTrojan,
				endpoint.ProtocolShadowsocks, endpoint.ProtocolHysteria2, endpoint.ProtocolTUIC,
				endpoint.ProtocolHTTP, endpoint.ProtocolHTTPS,
				endpoint.ProtocolSOCKS5,
			),
			CaptureModes: captureCapabilities(true), Controller: true, Metrics: true,
		},
	}
}

func mihomoSpec() adapterSpec {
	return adapterSpec{
		coreType: core.TypeMihomo, binaryName: "mihomo.exe",
		versionArgs: []string{"-v"}, extension: ".yaml",
		compile:      compiler.GenerateMihomo,
		validateArgs: func(path string) []string { return []string{"-t", "-f", path} },
		runArgs:      func(path string) []string { return []string{"-f", path} },
		capabilities: CapabilitySet{
			Protocols: allCapabilities(
				endpoint.ProtocolVLESS, endpoint.ProtocolVMess, endpoint.ProtocolTrojan,
				endpoint.ProtocolShadowsocks, endpoint.ProtocolHysteria2, endpoint.ProtocolTUIC,
				endpoint.ProtocolHTTP, endpoint.ProtocolHTTPS,
				endpoint.ProtocolSOCKS5,
			),
			CaptureModes: captureCapabilities(true), Controller: true, Metrics: true, HotReload: true,
		},
		controllerProbe: true,
	}
}

func xraySpec() adapterSpec {
	return adapterSpec{
		coreType: core.TypeXray, binaryName: "xray.exe",
		versionArgs: []string{"version"}, extension: ".json",
		compile:      compiler.GenerateXray,
		validateArgs: func(path string) []string { return []string{"run", "-test", "-c", path} },
		runArgs:      func(path string) []string { return []string{"run", "-c", path} },
		capabilities: CapabilitySet{
			Protocols: allCapabilities(
				endpoint.ProtocolVLESS, endpoint.ProtocolVMess, endpoint.ProtocolTrojan,
				endpoint.ProtocolShadowsocks,
				endpoint.ProtocolHTTP, endpoint.ProtocolHTTPS, endpoint.ProtocolSOCKS5,
			),
			CaptureModes: captureCapabilities(false),
		},
	}
}

type Registry struct {
	adapters map[core.Type]CoreAdapter
}

func NewRegistry(adapters ...CoreAdapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[core.Type]CoreAdapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil || !adapter.Type().Valid() {
			return nil, fmt.Errorf("invalid core adapter")
		}
		if _, exists := registry.adapters[adapter.Type()]; exists {
			return nil, fmt.Errorf("duplicate adapter for %s", adapter.Type())
		}
		registry.adapters[adapter.Type()] = adapter
	}
	return registry, nil
}

func NewDefaultRegistry() *Registry {
	registry, _ := NewRegistry(NewSingBoxAdapter(), NewMihomoAdapter(), NewXrayAdapter())
	return registry
}

func (r *Registry) Get(coreType core.Type) (CoreAdapter, error) {
	adapter, ok := r.adapters[coreType]
	if !ok {
		return nil, fmt.Errorf("core adapter %q is not registered", coreType)
	}
	return adapter, nil
}
