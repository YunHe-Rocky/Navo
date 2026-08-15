package network

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"navo/internal/host"
	"navo/internal/network/tun"
)

// ── Mock types implementing tun interfaces ──

type mockTun struct {
	created   bool
	installed bool
	cleanRes  *tun.CleanupResult
	cleanErr  error
}

func (m *mockTun) Create(ctx context.Context, name string) error        { m.created = true; return nil }
func (m *mockTun) Destroy(ctx context.Context) error                    { m.created = false; return nil }
func (m *mockTun) Configure(ctx context.Context, cfg *tun.Config) error { return nil }
func (m *mockTun) Status() *tun.Status {
	return &tun.Status{Name: "Navo-TUN", Installed: m.installed, Created: m.created}
}
func (m *mockTun) IsInstalled() bool { return m.installed }
func (m *mockTun) Cleanup(ctx context.Context) (*tun.CleanupResult, error) {
	if m.cleanErr != nil {
		return nil, m.cleanErr
	}
	m.created = false
	r := m.cleanRes
	if r == nil {
		r = &tun.CleanupResult{AdapterRemoved: true, RoutesCleaned: 2, DNSRestored: true}
	}
	return r, nil
}

type mockRoute struct {
	routes       []tun.Route
	cleanErr     error
	cleanedN     int
	cleanupCalls int
}

func (r *mockRoute) AddRoutes(ctx context.Context, n string, routes []tun.Route) error {
	r.routes = append(r.routes, routes...)
	return nil
}
func (r *mockRoute) RemoveRoutes(ctx context.Context, n string) error {
	r.routes = nil
	return nil
}
func (r *mockRoute) ListTUNRoutes(ctx context.Context, n string) ([]tun.Route, error) {
	return r.routes, nil
}
func (r *mockRoute) CleanupAll(ctx context.Context) (int, error) {
	r.cleanupCalls++
	if r.cleanErr != nil {
		return 0, r.cleanErr
	}
	n := r.cleanedN
	r.routes = nil
	return n, nil
}

type mockDNS struct {
	servers    []string
	configured bool
}

func (d *mockDNS) Set(ctx context.Context, n string, servers []string) error {
	d.servers = servers
	d.configured = true
	return nil
}
func (d *mockDNS) Reset(ctx context.Context, n string) error {
	d.servers = nil
	d.configured = false
	return nil
}
func (d *mockDNS) IsConfigured(ctx context.Context, n string) bool {
	return d.configured
}

// ── Tests ──

func TestReconciler_NormalShutdown(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	// Write normal state
	r.MarkNormalExit()

	result, err := r.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryNormal {
		t.Errorf("expected Normal, got %s", result.RecoveryState)
	}
}

func TestReconciler_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "nonexistent.json")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	result, err := r.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryNormal {
		t.Errorf("expected Normal when no state file, got %s", result.RecoveryState)
	}
}

func TestReconciler_DirtyShutdown_PortCheck(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	// Mark dirty shutdown
	port := unusedTestPort(t)
	r.MarkDirtyShutdown(12345, port, "", nil)

	result, err := r.Reconcile(context.Background(), &ReconcileConfig{ListenPort: port})
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("expected Ready after reconcile, got %s", result.RecoveryState)
	}
	if len(result.IssuesFixed) == 0 {
		t.Error("expected fixed issues")
	}
}

func TestReconciler_DirtyShutdown_TUNCleanup(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	tunMgr := &mockTun{installed: true, created: true}
	routeMgr := &mockRoute{cleanedN: 1}
	dnsMgr := &mockDNS{configured: true}

	r := NewReconciler(tunMgr, routeMgr, dnsMgr)
	r.SetStateFilePath(stateFile)

	r.MarkDirtyShutdown(12345, unusedTestPort(t), "Navo-TUN", []string{"1.1.1.1"})

	result, err := r.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("expected Ready, got %s", result.RecoveryState)
	}
	if len(result.IssuesFound) == 0 {
		t.Error("expected issues found for dirty shutdown with TUN")
	}
	// Verify TUN was cleaned
	if tunMgr.created {
		t.Error("expected TUN adapter to be cleaned up")
	}
}

func TestReconciler_DirtyShutdown_DoesNotGuessRouteOwnership(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	routeMgr := &mockRoute{
		routes:   []tun.Route{{Destination: "0.0.0.0/1", Metric: 1}},
		cleanedN: 1,
	}

	r := NewReconciler(&mockTun{installed: true}, routeMgr, &mockDNS{})
	r.SetStateFilePath(stateFile)

	r.MarkDirtyShutdown(1, 0, "", nil)

	result, err := r.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("expected Ready, got %s", result.RecoveryState)
	}
	if routeMgr.cleanupCalls != 0 || len(routeMgr.routes) != 1 {
		t.Fatal("Reconciler invoked the forbidden bulk route cleanup path")
	}
}

func TestReconciler_DirtyShutdown_DoesNotGuessDNSOwnership(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	dnsMgr := &mockDNS{configured: true}

	r := NewReconciler(&mockTun{installed: true}, &mockRoute{}, dnsMgr)
	r.SetStateFilePath(stateFile)

	r.MarkDirtyShutdown(12345, unusedTestPort(t), "Navo-TUN", []string{"8.8.8.8"})

	result, err := r.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryState != host.RecoveryReady {
		t.Errorf("expected Ready, got %s", result.RecoveryState)
	}
	if !dnsMgr.configured {
		t.Error("Reconciler modified DNS without Journal V2 ownership")
	}
}

func TestReconciler_CleanupFailure(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	tunMgr := &mockTun{
		installed: true,
		created:   true,
		cleanErr:  errors.New("cleanup failed"),
	}

	r := NewReconciler(tunMgr, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	r.MarkDirtyShutdown(12345, 0, "Navo-TUN", nil)

	result, err := r.Reconcile(context.Background(), nil)
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	if result.RecoveryState != host.RecoveryDirty {
		t.Errorf("expected Dirty after cleanup failure, got %s", result.RecoveryState)
	}
	if len(result.IssuesUnfixed) == 0 {
		t.Error("expected unfixed issues from cleanup failure")
	}
}

func TestReconciler_CorruptedStateFailsClosed(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")
	if err := os.WriteFile(stateFile, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)
	result, err := r.Reconcile(context.Background(), nil)
	if err == nil {
		t.Fatal("expected corrupted state error")
	}
	if result.RecoveryState != host.RecoveryDirty {
		t.Fatalf("expected Dirty for corrupted state, got %s", result.RecoveryState)
	}
}

func TestReconciler_EnvPathOverride(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom_recovery.json")

	os.Setenv("NAVO_RECOVERY_STATE_PATH", customPath)
	defer os.Unsetenv("NAVO_RECOVERY_STATE_PATH")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	// Should use env var path for state file
	r.MarkNormalExit()
	r.MarkDirtyShutdown(1, 0, "", nil)

	// Verify file was written to custom path
	if _, err := os.Stat(customPath); err != nil {
		t.Errorf("state file should exist at custom path: %v", err)
	}
}

func TestReconciler_MarkNormalExit(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	if err := r.MarkNormalExit(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(data, "NORMAL") {
		t.Error("expected NORMAL state in file")
	}
}

func TestReconciler_MarkDirtyShutdown(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "recovery.json")

	r := NewReconciler(&mockTun{}, &mockRoute{}, &mockDNS{})
	r.SetStateFilePath(stateFile)

	err := r.MarkDirtyShutdown(9999, 8080, "Navo-TUN", []string{"1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(data, "DIRTY_SHUTDOWN") {
		t.Error("expected DIRTY_SHUTDOWN state in file")
	}
	if !contains(data, "Navo-TUN") {
		t.Error("expected TUN adapter name in state file")
	}
}

func contains(data []byte, substr string) bool {
	return strings.Contains(string(data), substr)
}

func unusedTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
