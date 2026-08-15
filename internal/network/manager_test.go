package network

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeExecutor struct {
	mu                  sync.Mutex
	commands            []Command
	inspectionState     string
	failContains        string
	failOnce            bool
	cancelOnFailure     context.CancelFunc
	rollbackContextLive bool
	orphanFirewallState string
}

func (f *fakeExecutor) Run(ctx context.Context, command Command) error {
	return f.execute(ctx, command, false)
}

func (f *fakeExecutor) RunOutput(ctx context.Context, command Command) (string, error) {
	err := f.execute(ctx, command, true)
	if err != nil {
		return "", err
	}
	script := command.Args[len(command.Args)-1]
	if strings.Contains(script, "NAVO_ORPHAN_FIREWALL_SCAN") {
		if f.orphanFirewallState == "" {
			return "CLEAN", nil
		}
		return f.orphanFirewallState, nil
	}
	state := f.inspectionState
	if state == "" {
		state = "MISSING"
	}
	return state, nil
}

func TestManagerRecoveryRemovesOnlyVerifiedOrphanedFirewallRules(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{orphanFirewallState: "ORPHANED"}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	removed := false
	for _, command := range executor.commands {
		script := command.Args[len(command.Args)-1]
		if strings.Contains(script, "Remove-NetFirewallRule") {
			removed = true
		}
	}
	if !removed {
		t.Fatal("verified Navo-owned orphan firewall rule was not removed")
	}
}

func TestManagerRecoveryFailsClosedOnOrphanedFirewallConflict(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{orphanFirewallState: "CONFLICT"}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err == nil {
		t.Fatal("ambiguous orphaned firewall state was accepted")
	}
	for _, command := range executor.commands {
		if strings.Contains(command.Args[len(command.Args)-1], "Remove-NetFirewallRule") {
			t.Fatal("ambiguous firewall rule was removed")
		}
	}
}

func (f *fakeExecutor) execute(ctx context.Context, command Command, inspection bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, command)
	script := command.Args[len(command.Args)-1]
	if strings.Contains(script, "Remove-") && ctx.Err() == nil {
		f.rollbackContextLive = true
	}
	if f.failContains != "" && strings.Contains(script, f.failContains) && !f.failOnce {
		f.failOnce = true
		if f.cancelOnFailure != nil {
			f.cancelOnFailure()
		}
		return errors.New("injected command failure")
	}
	if inspection && ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

type fakePlatform struct {
	preflightErr error
	waitErr      error
	verifyErr    error
	adapter      AdapterSnapshot
	adapters     []AdapterSnapshot
	waitCalls    *int
}

func (f fakePlatform) Preflight(context.Context, Config) error { return f.preflightErr }
func (f fakePlatform) WaitForAdapterReady(context.Context, string, string, int, time.Duration) (AdapterSnapshot, error) {
	if f.waitErr != nil {
		return AdapterSnapshot{}, f.waitErr
	}
	if len(f.adapters) > 0 && f.waitCalls != nil {
		index := *f.waitCalls
		*f.waitCalls++
		if index >= len(f.adapters) {
			index = len(f.adapters) - 1
		}
		return f.adapters[index], nil
	}
	if f.adapter.InterfaceIndex != 0 {
		return f.adapter, nil
	}
	return testAdapter(), nil
}

func TestManagerRebindReplacesAdapterBoundResources(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	waitCalls := 0
	first := testAdapter()
	second := testAdapter()
	second.InterfaceIndex = 49
	second.InterfaceGUID = "{NAVO-REBOUND}"
	manager, err := NewManager(cfg, executor, fakePlatform{
		adapters: []AdapterSnapshot{first, second}, waitCalls: &waitCalls,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	rebound, err := manager.Rebind(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rebound.InterfaceIndex != second.InterfaceIndex || rebound.InterfaceGUID != second.InterfaceGUID {
		t.Fatalf("rebound adapter = %#v, want %#v", rebound, second)
	}
	value, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if value.Adapter.InterfaceIndex != second.InterfaceIndex || value.Adapter.InterfaceGUID != second.InterfaceGUID {
		t.Fatalf("journal retained stale adapter: %#v", value.Adapter)
	}
}
func (f fakePlatform) VerifyControlPlane(context.Context, TUNActivationPlan, AdapterSnapshot) error {
	return f.verifyErr
}

func testAdapter() AdapterSnapshot {
	return AdapterSnapshot{Name: "Navo", InterfaceIndex: 27, InterfaceGUID: "{NAVO-GUID}", OperationalStatus: "Up", MTU: 1500, IPv4Addresses: []string{"172.19.0.1/30"}}
}

func testPlan() TUNActivationPlan {
	return TUNActivationPlan{SessionID: "test-session", CoreID: "sing-box", AdapterName: "Navo", TUNIPv4Address: "172.19.0.1/30", TUNIPv4Peer: "172.19.0.2", TUNDNSIPv4: "172.19.0.2", MTU: 1500, PhysicalRoute: EndpointRoutePlan{EndpointIP: "1.1.1.1", AddressFamily: "IPv4", InterfaceIndex: 8, InterfaceGUID: "{PHYSICAL}", InterfaceAlias: "Ethernet", NextHop: "192.0.2.1"}, IPv6Mode: IPv6Block, CreatedAt: time.Now().UTC()}
}

func TestOwnedTUNAdapterRejectsPhysicalEthernetIdentity(t *testing.T) {
	physical := AdapterSnapshot{
		Name: OwnedTUNAdapterName, InterfaceDescription: "Realtek Gaming 2.5GbE Family Controller",
		HardwareInterface: true, InterfaceIndex: 7, InterfaceGUID: "{ETHERNET}",
	}
	if isOwnedTUNAdapter(physical) {
		t.Fatal("renamed physical Ethernet adapter was treated as Navo-owned")
	}
	owned := AdapterSnapshot{
		Name: OwnedTUNAdapterName, InterfaceDescription: ownedTUNDescription,
		HardwareInterface: false, InterfaceIndex: 27, InterfaceGUID: "{WINTUN}",
	}
	if !isOwnedTUNAdapter(owned) {
		t.Fatal("valid Navo Wintun adapter was rejected")
	}
	owned.InterfaceDescription = ownedSingTUNDescription
	if !isOwnedTUNAdapter(owned) {
		t.Fatal("valid Navo sing-tun adapter was rejected")
	}
	owned.InterfaceDescription = ownedSingTUNDescription + " #2"
	if !isOwnedTUNAdapter(owned) {
		t.Fatal("valid numbered Navo sing-tun adapter was rejected")
	}
	owned.InterfaceDescription = ownedTUNDescription + " #12"
	if !isOwnedTUNAdapter(owned) {
		t.Fatal("valid numbered Navo Wintun adapter was rejected")
	}
	for _, description := range []string{
		ownedSingTUNDescription + " #",
		ownedSingTUNDescription + " #two",
		ownedSingTUNDescription + " backup #2",
	} {
		owned.InterfaceDescription = description
		if isOwnedTUNAdapter(owned) {
			t.Fatalf("invalid tunnel description %q was treated as Navo-owned", description)
		}
	}
	owned.InterfaceDescription = "arbitrary software tunnel"
	if isOwnedTUNAdapter(owned) {
		t.Fatal("unknown software tunnel identity was treated as Navo-owned")
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{Enabled: true, AdapterName: "Navo", WintunDLLPath: `C:\Navo\wintun.dll`, JournalPath: filepath.Join(t.TempDir(), "network.json"), TUNIPv4Gateway: "172.19.0.2", TUNIPv4Address: "172.19.0.1/30", TUNIPv4Peer: "172.19.0.2", TUNDNSIPv4: "172.19.0.2", MTU: 1500, DNSServers: []string{"172.19.0.2"}, IPv6Mode: IPv6Block, ActivationPlan: testPlan()}
}

func TestManagerActivateAndDeactivatePersistsV2Resources(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := manager.Activate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if adapter.InterfaceGUID != testAdapter().InterfaceGUID {
		t.Fatalf("adapter = %#v", adapter)
	}
	value, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if value.Version != 2 || len(value.Actions) != 4 {
		t.Fatalf("journal = %#v", value)
	}
	raw, err := os.ReadFile(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"undo"`) || strings.Contains(string(raw), "powershell") {
		t.Fatalf("V2 journal retained executable command authority: %s", raw)
	}
	for _, action := range value.Actions {
		if !action.Resource.CreatedByNavo {
			t.Fatalf("unsafe V2 action = %#v", action)
		}
	}
	if err := manager.Deactivate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal remains: %v", err)
	}
}

func TestManagerExactResourcesAreNotDeleted(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{inspectionState: "EXACT"}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if _, err := manager.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Deactivate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, command := range executor.commands {
		script := command.Args[len(command.Args)-1]
		if strings.Contains(script, "New-Net") || strings.Contains(script, "Add-Dns") || strings.Contains(script, "Remove-") {
			t.Fatalf("pre-existing exact resource was mutated: %s", script)
		}
	}
}

func TestManagerRollsBackEveryApplyFailure(t *testing.T) {
	for _, marker := range []string{"New-NetRoute", "Add-DnsClientNrptRule", "New-NetFirewallRule"} {
		t.Run(marker, func(t *testing.T) {
			cfg := testConfig(t)
			executor := &fakeExecutor{failContains: marker}
			manager, _ := NewManager(cfg, executor, fakePlatform{})
			if _, err := manager.Activate(context.Background()); err == nil {
				t.Fatal("activation unexpectedly succeeded")
			}
			if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("successful rollback retained journal: %v", err)
			}
		})
	}
}

func TestManagerFailurePointsRollbackEveryCommittedResource(t *testing.T) {
	for _, point := range []string{"after-first-split-route", "after-second-split-route", "after-nrpt", "after-ipv6"} {
		t.Run(point, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.FailurePoint = point
			manager, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Activate(context.Background()); err == nil {
				t.Fatal("failure injection unexpectedly succeeded")
			}
			if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rollback retained journal: %v", err)
			}
		})
	}
}

func TestManagerRecoversDurableJournalAfterCrashAtNRPT(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	cfg.CrashPoint = "after-nrpt"
	crashed := false
	cfg.CrashFn = func() {
		crashed = true
		panic("simulated process crash")
	}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("activation did not reach the crash boundary")
			}
		}()
		_, _ = manager.Activate(context.Background())
	}()
	if !crashed {
		t.Fatal("NRPT crash point was not reached")
	}
	if _, err := os.Stat(cfg.JournalPath); err != nil {
		t.Fatalf("durable crash journal is missing: %v", err)
	}

	freshManager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}
	if err := freshManager.Recover(context.Background()); err != nil {
		t.Fatalf("fresh manager recovery failed: %v", err)
	}
	removedNRPT := false
	for _, command := range executor.commands {
		script := command.Args[len(command.Args)-1]
		if strings.Contains(script, "Remove-DnsClientNrptRule") {
			removedNRPT = true
		}
	}
	if !removedNRPT {
		t.Fatal("fresh manager did not remove the owned NRPT rule")
	}
	if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovered crash journal remains: %v", err)
	}
}

func TestManagerRollbackUsesIndependentContextAfterCancellation(t *testing.T) {
	cfg := testConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	executor := &fakeExecutor{failContains: "Add-DnsClientNrptRule", cancelOnFailure: cancel}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if _, err := manager.Activate(ctx); err == nil {
		t.Fatal("activation unexpectedly succeeded")
	}
	if !executor.rollbackContextLive {
		t.Fatal("rollback reused the canceled activation context")
	}
}

func TestManagerKeepsFailedUndoInV2Journal(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if _, err := manager.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	executor.failContains = "Remove-NetFirewallRule"
	executor.failOnce = false
	if err := manager.Deactivate(context.Background()); err == nil {
		t.Fatal("rollback unexpectedly succeeded")
	}
	remaining, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining.Actions) != 1 || remaining.Actions[0].Resource.Kind != resourceFirewallRule {
		t.Fatalf("failed resource was not retained: %#v", remaining.Actions)
	}
}

func TestManagerControlPlaneFailureRollsBackWithoutCommit(t *testing.T) {
	cfg := testConfig(t)
	manager, _ := NewManager(cfg, &fakeExecutor{}, fakePlatform{verifyErr: errors.New("route escaped")})
	if _, err := manager.Activate(context.Background()); err == nil {
		t.Fatal("control-plane failure unexpectedly succeeded")
	}
	if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("control-plane rollback retained journal: %v", err)
	}
}

func TestManagerMigratesWhitelistedV1WithoutExecutingUndo(t *testing.T) {
	cfg := testConfig(t)
	legacy := &legacyJournal{Version: 1, AdapterName: "Navo", SessionID: "legacy", CreatedAt: time.Now(), Actions: []legacyJournalAction{{Name: "DNS leak protection", Status: actionApplied, Undo: powershell("Write-Output compromised")}}}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(cfg.JournalPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, command := range executor.commands {
		if strings.Contains(command.Args[len(command.Args)-1], "compromised") {
			t.Fatal("journal-supplied command was executed")
		}
	}
}

func TestManagerRefusesUnprovableLegacyEndpointRoute(t *testing.T) {
	cfg := testConfig(t)
	legacy := &legacyJournal{Version: 1, AdapterName: "Navo", SessionID: "legacy", CreatedAt: time.Now(), Actions: []legacyJournalAction{{Name: "proxy endpoint bypass 203.0.113.7", Status: actionApplied, Undo: powershell("Write-Output compromised")}}}
	data, _ := json.Marshal(legacy)
	_ = os.WriteFile(cfg.JournalPath, data, 0o600)
	executor := &fakeExecutor{}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err == nil {
		t.Fatal("unsafe legacy endpoint recovery unexpectedly succeeded")
	}
	if len(executor.commands) != 0 {
		t.Fatal("unsafe legacy endpoint caused a mutation")
	}
	if _, err := os.Stat(cfg.JournalPath); err != nil {
		t.Fatal("dirty evidence was removed")
	}
}

func TestManagerNeverExecutesUnknownJournalCommand(t *testing.T) {
	cfg := testConfig(t)
	legacy := &legacyJournal{Version: 1, AdapterName: "Navo", SessionID: "tampered", CreatedAt: time.Now(), Actions: []legacyJournalAction{{Name: "arbitrary command", Status: actionApplied, Undo: powershell("Write-Output compromised")}}}
	data, _ := json.Marshal(legacy)
	_ = os.WriteFile(cfg.JournalPath, data, 0o600)
	executor := &fakeExecutor{}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err == nil {
		t.Fatal("tampered journal unexpectedly recovered")
	}
	if len(executor.commands) != 0 {
		t.Fatal("tampered journal command was executed")
	}
}

func TestConfigValidationRejectsUnsafeInput(t *testing.T) {
	cfg := testConfig(t)
	cfg.AdapterName = "Navo'; Remove-Item C:\\ -Recurse; '"
	if _, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{}); err == nil {
		t.Fatal("unsafe adapter name was accepted")
	}
	cfg = testConfig(t)
	cfg.WintunSHA256 = "not-a-hash"
	if _, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{}); err == nil {
		t.Fatal("invalid Wintun digest was accepted")
	}
}

func TestDisabledManagerIsInert(t *testing.T) {
	manager, err := NewManager(Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
}
