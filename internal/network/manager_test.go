package network

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeExecutor struct {
	mu        sync.Mutex
	commands  []Command
	failAt    int
	callCount int
}

func (f *fakeExecutor) Run(_ context.Context, command Command) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	f.commands = append(f.commands, command)
	if f.failAt > 0 && f.callCount == f.failAt {
		return errors.New("injected command failure")
	}
	return nil
}

type fakePlatform struct {
	preflightErr error
	waitErr      error
}

func (f fakePlatform) Preflight(context.Context, Config) error {
	return f.preflightErr
}

func (f fakePlatform) WaitForAdapter(context.Context, string, time.Duration) error {
	return f.waitErr
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Enabled:        true,
		AdapterName:    "Navo",
		WintunDLLPath:  `C:\Navo\wintun.dll`,
		JournalPath:    filepath.Join(t.TempDir(), "network.json"),
		TUNIPv4Gateway: "172.19.0.2",
		DNSServers:     []string{"172.19.0.2"},
		IPv6Mode:       IPv6Block,
	}
}

func TestManagerActivateAndDeactivate(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := manager.Preflight(context.Background()); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if err := manager.Activate(context.Background()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := os.Stat(cfg.JournalPath); err != nil {
		t.Fatalf("journal was not persisted: %v", err)
	}
	applyCount := len(executor.commands)
	if applyCount != 4 {
		t.Fatalf("apply command count = %d, want 4", applyCount)
	}
	ipv6Rule := executor.commands[applyCount-1].Args[len(executor.commands[applyCount-1].Args)-1]
	if !strings.Contains(ipv6Rule, "'::/1'") || !strings.Contains(ipv6Rule, "'8000::/1'") {
		t.Fatalf("IPv6 block rule does not cover both address-space halves: %s", ipv6Rule)
	}
	if strings.Contains(ipv6Rule, "'::/0'") {
		t.Fatalf("IPv6 block rule uses the Windows-incompatible ::/0 range: %s", ipv6Rule)
	}

	if err := manager.Deactivate(context.Background()); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal still exists after rollback: %v", err)
	}
	if len(executor.commands) != applyCount*2 {
		t.Fatalf("total command count = %d, want %d", len(executor.commands), applyCount*2)
	}
	if script := executor.commands[applyCount].Args[len(executor.commands[applyCount].Args)-1]; !strings.Contains(script, "Remove-NetFirewallRule") {
		t.Fatalf("rollback was not executed in reverse order: %s", script)
	}
}

func TestAllNetworkUndoCommandsAreIdempotent(t *testing.T) {
	cfg := testConfig(t)
	cfg.ProxyEndpointIPs = []string{"203.0.113.7"}
	manager, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range manager.operations("rollback") {
		script := operation.undo.Args[len(operation.undo.Args)-1]
		if !strings.HasSuffix(script, "exit 0") {
			t.Fatalf("undo %q can fail when its resource is already missing: %s", operation.name, script)
		}
	}
}

func TestEndpointBypassSelectsOneTypedPhysicalRoute(t *testing.T) {
	for _, endpoint := range []string{"203.0.113.7", "2001:db8::7"} {
		family := IPv6Tunnel
		cfg := testConfig(t)
		cfg.IPv6Mode = family
		cfg.TUNIPv6Gateway = "fd00::2"
		cfg.ProxyEndpointIPs = []string{endpoint}
		manager, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{})
		if err != nil {
			t.Fatal(err)
		}
		script := manager.operations("scalar")[0].apply.Args[len(manager.operations("scalar")[0].apply.Args)-1]
		for _, required := range []string{
			"Select-Object -First 1",
			"[uint32]$r.InterfaceIndex",
			"[string]$r.NextHop",
			"InterfaceAlias -ne 'Navo'",
			"$interfaceIndex",
			"$nextHop",
		} {
			if !strings.Contains(script, required) {
				t.Fatalf("%s route script missing %q: %s", endpoint, required, script)
			}
		}
		if strings.Contains(script, "-InterfaceIndex $r.InterfaceIndex") || strings.Contains(script, "-NextHop $r.NextHop") {
			t.Fatalf("%s route script forwards an unbounded result: %s", endpoint, script)
		}
	}
}

func TestPowerShellCommandsForceUTF8Output(t *testing.T) {
	command := powershell("Write-Output '测试'")
	script := command.Args[len(command.Args)-1]
	if !strings.Contains(script, "[Console]::OutputEncoding") || !strings.Contains(script, "$OutputEncoding") {
		t.Fatalf("PowerShell encoding prelude missing: %s", script)
	}
}

func TestManagerRollsBackFailedActivation(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{failAt: 3}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := manager.Activate(context.Background()); err == nil {
		t.Fatal("Activate unexpectedly succeeded")
	}
	// Three apply attempts followed by undo for pending DNS and both applied routes.
	if len(executor.commands) != 6 {
		t.Fatalf("command count = %d, want 6", len(executor.commands))
	}
	if _, err := os.Stat(cfg.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal should be removed after successful rollback: %v", err)
	}
}

func TestManagerRecoversCrashJournal(t *testing.T) {
	cfg := testConfig(t)
	value := &journal{
		Version:     1,
		AdapterName: cfg.AdapterName,
		SessionID:   "crashed",
		CreatedAt:   time.Now(),
		Actions: []journalAction{
			{Name: "IPv4 route 0.0.0.0/1", Undo: powershell("untrusted-route"), Status: actionApplied},
			{Name: "DNS leak protection", Undo: powershell("untrusted-dns"), Status: actionPending},
		},
	}
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatalf("writeJournal: %v", err)
	}

	executor := &fakeExecutor{}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(executor.commands) != 2 {
		t.Fatalf("undo count = %d, want 2", len(executor.commands))
	}
	if got := executor.commands[0].Args[len(executor.commands[0].Args)-1]; !strings.Contains(got, "Get-DnsClientNrptRule") {
		t.Fatalf("first undo was not rebuilt from the trusted DNS action: %q", got)
	} else if strings.Contains(got, "untrusted-dns") {
		t.Fatalf("journal-supplied command was executed: %q", got)
	}
}

func TestManagerNormalizesLegacyIPv6FirewallUndo(t *testing.T) {
	cfg := testConfig(t)
	value := &journal{
		Version:     1,
		AdapterName: cfg.AdapterName,
		SessionID:   "legacy",
		CreatedAt:   time.Now(),
		Actions: []journalAction{{
			Name:   "IPv6 leak protection",
			Undo:   powershell("Remove-NetFirewallRule -DisplayName 'Navo TUN IPv6 Block legacy' -ErrorAction SilentlyContinue"),
			Status: actionPending,
		}},
	}
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatalf("writeJournal: %v", err)
	}

	executor := &fakeExecutor{}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(executor.commands) != 1 {
		t.Fatalf("undo count = %d, want 1", len(executor.commands))
	}
	script := executor.commands[0].Args[len(executor.commands[0].Args)-1]
	if !strings.Contains(script, "if($rule)") {
		t.Fatalf("legacy firewall undo was not normalized: %s", script)
	}
	if !strings.HasSuffix(script, "exit 0") {
		t.Fatalf("idempotent firewall undo does not force a successful exit: %s", script)
	}
}

func TestManagerKeepsFailedUndoInJournal(t *testing.T) {
	cfg := testConfig(t)
	value := &journal{
		Version:     1,
		AdapterName: cfg.AdapterName,
		SessionID:   "crashed",
		CreatedAt:   time.Now(),
		Actions: []journalAction{
			{Name: "IPv4 route 0.0.0.0/1", Undo: powershell("untrusted-first"), Status: actionApplied},
			{Name: "DNS leak protection", Undo: powershell("untrusted-second"), Status: actionApplied},
		},
	}
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatalf("writeJournal: %v", err)
	}
	executor := &fakeExecutor{failAt: 1}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err == nil {
		t.Fatal("Recover unexpectedly succeeded")
	}
	remaining, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatalf("readJournal: %v", err)
	}
	if len(remaining.Actions) != 1 || remaining.Actions[0].Name != "DNS leak protection" {
		t.Fatalf("failed undo was not retained: %+v", remaining.Actions)
	}
}

func TestManagerNeverExecutesUnknownJournalCommand(t *testing.T) {
	cfg := testConfig(t)
	value := &journal{
		Version:     1,
		AdapterName: cfg.AdapterName,
		SessionID:   "tampered",
		CreatedAt:   time.Now(),
		Actions: []journalAction{{
			Name: "arbitrary command", Undo: powershell("Write-Output compromised"),
			Status: actionApplied,
		}},
	}
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{}
	manager, _ := NewManager(cfg, executor, fakePlatform{})
	if err := manager.Recover(context.Background()); err == nil {
		t.Fatal("tampered journal unexpectedly recovered")
	}
	if len(executor.commands) != 0 {
		t.Fatalf("tampered journal command was executed: %+v", executor.commands)
	}
	if _, err := os.Stat(cfg.JournalPath); err != nil {
		t.Fatalf("tampered journal evidence was not retained: %v", err)
	}
}

func TestConfigValidationRejectsUnsafeInput(t *testing.T) {
	cfg := testConfig(t)
	cfg.AdapterName = "Navo'; Remove-Item C:\\ -Recurse; '"
	if _, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{}); err == nil {
		t.Fatal("unsafe adapter name was accepted")
	}

	cfg = testConfig(t)
	cfg.IPv6Mode = IPv6Tunnel
	if _, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{}); err == nil {
		t.Fatal("IPv6 tunnel without gateway was accepted")
	}

	cfg = testConfig(t)
	cfg.ProxyEndpointIPs = []string{"2001:db8::1"}
	if _, err := NewManager(cfg, &fakeExecutor{}, fakePlatform{}); err == nil {
		t.Fatal("IPv6 endpoint was accepted in block mode")
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
		t.Fatalf("NewManager: %v", err)
	}
	if err := manager.Recover(context.Background()); err != nil {
		t.Fatalf("disabled Recover: %v", err)
	}
}
