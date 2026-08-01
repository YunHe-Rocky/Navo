package coreadapter

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"navo/internal/compiler"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
	"navo/internal/domain/selection"
	"navo/internal/domain/source"
)

func TestDefaultRegistryHasIndependentAdapters(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistry()
	for _, coreType := range core.All() {
		adapter, err := registry.Get(coreType)
		if err != nil {
			t.Fatal(err)
		}
		if adapter.Type() != coreType || adapter.BinaryName() == "" {
			t.Fatalf("invalid adapter for %s", coreType)
		}
	}
}

func TestAdaptersCompileNativeFormats(t *testing.T) {
	t.Parallel()

	subscriptionID, endpointID := "subscription-1", "endpoint-1"
	request := CompileRequest{
		Selection: selection.ActiveSelection{
			CoreType: core.TypeSingBox, SourceType: source.TypeAirportSubscription,
			CaptureMode:    capture.ModeSystemProxy,
			SubscriptionID: &subscriptionID, EndpointID: &endpointID,
		},
		Config: &compiler.Config{
			SchemaVersion: 1,
			Log:           compiler.LogConfig{Level: "info"},
			Inbounds: []compiler.InboundConfig{{
				Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: 12080,
			}},
			Outbounds: []compiler.Outbound{
				{ID: "direct", Type: compiler.OutboundDirect, Enabled: true},
				{ID: "node", Type: compiler.OutboundVLESS, Server: "example.com", Port: 443, UUID: "bf000d23-0752-40b4-affe-68f7707a9661", Enabled: true},
			},
			FinalOutbound: "node",
		},
		RuntimeDir: t.TempDir(),
	}

	registry := NewDefaultRegistry()
	for _, coreType := range core.All() {
		adapter, err := registry.Get(coreType)
		if err != nil {
			t.Fatal(err)
		}
		current := request
		current.Selection.CoreType = coreType
		current.RevisionID = coreType.String() + "-revision"
		compiled, err := adapter.Compile(context.Background(), current)
		if err != nil {
			t.Fatalf("%s compile: %v", coreType, err)
		}
		if compiled.CoreType != coreType || len(compiled.ContentHash) != 64 {
			t.Fatalf("%s returned invalid metadata", coreType)
		}
		info, err := os.Stat(compiled.MainConfigPath)
		if err != nil {
			t.Fatalf("%s config stat failed: %v", coreType, err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			t.Fatalf("%s config permissions are not private: info=%v err=%v", coreType, info, err)
		}
		wantExtension := ".json"
		if coreType == core.TypeMihomo {
			wantExtension = ".yaml"
		}
		if filepath.Ext(compiled.MainConfigPath) != wantExtension {
			t.Fatalf("%s config extension = %s", coreType, filepath.Ext(compiled.MainConfigPath))
		}
	}
}

func TestHealthProbeRejectsMissingReadinessPorts(t *testing.T) {
	t.Parallel()
	result := NewSingBoxAdapter().HealthProbe(context.Background(), RuntimeInfo{
		PID: 42, ProcessRunning: true,
	})
	if result.Healthy || result.Error == "" {
		t.Fatal("health probe accepted a process without readiness ports")
	}
}

func TestCapabilitiesDoNotAdvertiseIncompleteWireGuard(t *testing.T) {
	for _, adapter := range []CoreAdapter{NewSingBoxAdapter(), NewMihomoAdapter(), NewXrayAdapter()} {
		if adapter.Capabilities(Version{}).Protocols[endpoint.ProtocolWireGuard] {
			t.Fatalf("%s advertised incomplete WireGuard support", adapter.Type())
		}
	}
}
