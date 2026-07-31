package coreadapter

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/selection"
	"navo/internal/domain/source"
)

func TestBundledCoresValidateNativeCompiledConfig(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := map[core.Type]string{
		core.TypeSingBox: filepath.Join(root, "third_party", "sing-box", "sing-box.exe"),
		core.TypeMihomo:  filepath.Join(root, "third_party", "mihomo", "mihomo.exe"),
		core.TypeXray:    filepath.Join(root, "third_party", "xray", "xray.exe"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Skip("bundled core binaries are unavailable")
		}
	}

	subscriptionID, endpointID := "subscription-1", "endpoint-1"
	registry := NewDefaultRegistry()
	for _, coreType := range core.All() {
		coreType := coreType
		t.Run(coreType.String(), func(t *testing.T) {
			adapter, err := registry.Get(coreType)
			if err != nil {
				t.Fatal(err)
			}
			request := CompileRequest{
				Selection: selection.ActiveSelection{
					CoreType: coreType, SourceType: source.TypeAirportSubscription,
					CaptureMode: capture.ModeOff, SubscriptionID: &subscriptionID,
					EndpointID: &endpointID,
				},
				Config: &compiler.Config{
					SchemaVersion: 1,
					Log:           compiler.LogConfig{Level: "info", Timestamp: true},
					Inbounds: []compiler.InboundConfig{{
						Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080,
					}},
					Outbounds: []compiler.Outbound{{
						ID: "direct", Name: "Direct", Type: compiler.OutboundDirect, Enabled: true,
					}},
					FinalOutbound: "direct",
				},
				RuntimeDir: t.TempDir(), RevisionID: "native-validation",
			}
			compiled, err := adapter.Compile(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			versionCtx, cancelVersion := context.WithTimeout(context.Background(), 5*time.Second)
			version, err := adapter.DetectVersion(versionCtx, paths[coreType])
			cancelVersion()
			if err != nil || version.Raw == "" {
				t.Fatalf("version: %+v, %v", version, err)
			}
			validateCtx, cancelValidate := context.WithTimeout(context.Background(), 10*time.Second)
			result := adapter.Validate(validateCtx, CoreInstallation{
				Type: coreType, BinaryPath: paths[coreType], Version: version,
			}, compiled)
			cancelValidate()
			if !result.Valid {
				t.Fatalf("native validation failed: output=%q err=%v", result.Output, result.Err)
			}
		})
	}
}

func TestBundledSingBoxValidatesTUNDNSAndICMPConfig(t *testing.T) {
	root := filepath.Join("..", "..")
	binary := filepath.Join(root, "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("bundled sing-box binary is unavailable")
	}

	adapter := NewSingBoxAdapter()
	upstreamID := "upstream"
	request := CompileRequest{
		Selection: selection.ActiveSelection{
			CoreType: core.TypeSingBox, SourceType: source.TypeUpstreamProxy,
			CaptureMode: capture.ModeTUN, UpstreamProxyID: &upstreamID,
		},
		Config: &compiler.Config{
			SchemaVersion: 1,
			Log:           compiler.LogConfig{Level: "info", Timestamp: true},
			Inbounds: []compiler.InboundConfig{
				{Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080},
				{Type: "tun", Tag: "tun-in", Sniff: true},
			},
			Outbounds: []compiler.Outbound{
				{ID: "direct", Name: "Direct", Type: compiler.OutboundDirect, Enabled: true},
				{
					ID: "upstream", Name: "Upstream", Type: compiler.OutboundSOCKS,
					Server: "example.com", Port: 1080, Enabled: true,
				},
			},
			FinalOutbound: "upstream",
			TUN: &compiler.TUNConfig{
				Enabled: true, InterfaceName: "Navo",
				MTU: 1500, Address: []string{"172.19.0.1/30"},
				AutoRoute: true, StrictRoute: true,
			},
		},
		RuntimeDir: t.TempDir(), RevisionID: "tun-native-validation",
	}
	compiled, err := adapter.Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	versionCtx, cancelVersion := context.WithTimeout(context.Background(), 5*time.Second)
	version, err := adapter.DetectVersion(versionCtx, binary)
	cancelVersion()
	if err != nil {
		t.Fatal(err)
	}
	validateCtx, cancelValidate := context.WithTimeout(context.Background(), 10*time.Second)
	result := adapter.Validate(validateCtx, CoreInstallation{
		Type: core.TypeSingBox, BinaryPath: binary, Version: version,
	}, compiled)
	cancelValidate()
	if !result.Valid {
		t.Fatalf("native TUN validation failed: output=%q err=%v", result.Output, result.Err)
	}
}

func TestBundledSingBoxStartsModernDirectDNSService(t *testing.T) {
	root := filepath.Join("..", "..")
	binary := filepath.Join(root, "third_party", "sing-box", "sing-box.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("bundled sing-box binary is unavailable")
	}

	upstreamID := "upstream"
	adapter := NewSingBoxAdapter()
	compiled, err := adapter.Compile(context.Background(), CompileRequest{
		Selection: selection.ActiveSelection{
			CoreType: core.TypeSingBox, SourceType: source.TypeUpstreamProxy,
			CaptureMode: capture.ModeOff, UpstreamProxyID: &upstreamID,
		},
		Config: &compiler.Config{
			SchemaVersion: 1,
			Log:           compiler.LogConfig{Level: "warn"},
			Outbounds: []compiler.Outbound{{
				ID: "direct", Name: "Direct", Type: compiler.OutboundDirect, Enabled: true,
			}},
			FinalOutbound: "direct",
			DNS: &compiler.DNSConfig{
				Enabled: true, Strategy: compiler.DNSStrategyIPv4Only,
				Servers: []compiler.DNSServer{{
					Type: "udp", Tag: "dns-direct", Server: "223.5.5.5", ServerPort: 53,
				}},
				Final: "dns-direct",
			},
		},
		RuntimeDir: t.TempDir(), RevisionID: "dns-start-validation",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary, "run", "-c", compiled.MainConfigPath)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		cancel()
		t.Fatalf("sing-box exited during DNS service startup: %v; output=%q", err, output.String())
	case <-time.After(750 * time.Millisecond):
		cancel()
		<-done
	}
}
