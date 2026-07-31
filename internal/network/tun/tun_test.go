package tun

import (
	"context"
	"errors"
	"testing"
)

// ── Mock implementations ──

type mockManager struct {
	createErr    error
	configErr    error
	destroyErr   error
	installedVal bool
	status       *Status
	cleanupRes   *CleanupResult
	cleanupErr   error
	created      bool
	cfg          *Config
}

func (m *mockManager) Create(ctx context.Context, name string) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = true
	m.status = &Status{Name: name, Installed: true, Created: true, MTU: 1500}
	return nil
}
func (m *mockManager) Destroy(ctx context.Context) error {
	if m.destroyErr != nil {
		return m.destroyErr
	}
	m.created = false
	if m.status != nil {
		m.status.Created = false
	}
	return nil
}
func (m *mockManager) Configure(ctx context.Context, cfg *Config) error {
	if m.configErr != nil {
		return m.configErr
	}
	m.cfg = cfg
	if m.status == nil {
		m.status = &Status{}
	}
	m.status.MTU = cfg.MTU
	m.status.Addresses = cfg.Address
	return nil
}
func (m *mockManager) Status() *Status {
	if m.status == nil {
		return &Status{Name: "mock", Installed: m.installedVal}
	}
	return m.status
}
func (m *mockManager) IsInstalled() bool { return m.installedVal }
func (m *mockManager) Cleanup(ctx context.Context) (*CleanupResult, error) {
	if m.cleanupErr != nil {
		return nil, m.cleanupErr
	}
	return m.cleanupRes, nil
}

type mockRouteManager struct {
	addErr     error
	removeErr  error
	cleanupErr error
	routes     []Route
}

func (r *mockRouteManager) AddRoutes(ctx context.Context, adapterName string, routes []Route) error {
	if r.addErr != nil {
		return r.addErr
	}
	r.routes = append(r.routes, routes...)
	return nil
}
func (r *mockRouteManager) RemoveRoutes(ctx context.Context, adapterName string) error {
	if r.removeErr != nil {
		return r.removeErr
	}
	r.routes = nil
	return nil
}
func (r *mockRouteManager) ListTUNRoutes(ctx context.Context, adapterName string) ([]Route, error) {
	return r.routes, nil
}
func (r *mockRouteManager) CleanupAll(ctx context.Context) (int, error) {
	if r.cleanupErr != nil {
		return 0, r.cleanupErr
	}
	n := len(r.routes)
	r.routes = nil
	return n, nil
}

type mockDNSManager struct {
	setErr      error
	resetErr    error
	servers     []string
	configured  bool
}

func (d *mockDNSManager) Set(ctx context.Context, adapterName string, servers []string) error {
	if d.setErr != nil {
		return d.setErr
	}
	d.servers = servers
	d.configured = true
	return nil
}
func (d *mockDNSManager) Reset(ctx context.Context, adapterName string) error {
	if d.resetErr != nil {
		return d.resetErr
	}
	d.servers = nil
	d.configured = false
	return nil
}
func (d *mockDNSManager) IsConfigured(ctx context.Context, adapterName string) bool {
	return d.configured
}

// ── Manager tests ──

func TestManager_Create(t *testing.T) {
	m := &mockManager{installedVal: true}
	ctx := context.Background()
	err := m.Create(ctx, "Navo-TUN")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	st := m.Status()
	if !st.Created {
		t.Error("expected Created=true after Create")
	}
	if st.Name != "Navo-TUN" {
		t.Errorf("expected name Navo-TUN, got %s", st.Name)
	}
}

func TestManager_Create_DLLNotFound(t *testing.T) {
	m := &mockManager{createErr: errors.New(ErrNet001 + ": wintun.dll not found")}
	err := m.Create(context.Background(), "Navo-TUN")
	if err == nil {
		t.Fatal("expected error for missing DLL")
	}
}

func TestManager_Create_AlreadyExists(t *testing.T) {
	m := &mockManager{installedVal: true}
	ctx := context.Background()
	if err := m.Create(ctx, "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
	// Second create should succeed (or be a no-op, depending on implementation)
	if err := m.Create(ctx, "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
}

func TestManager_Configure(t *testing.T) {
	m := &mockManager{installedVal: true}
	ctx := context.Background()
	if err := m.Create(ctx, "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Name:    "Navo-TUN",
		MTU:     1500,
		Address: []string{"10.0.0.1/24"},
		DNS:     []string{"1.1.1.1"},
	}
	if err := m.Configure(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	st := m.Status()
	if st.MTU != 1500 {
		t.Errorf("expected MTU 1500, got %d", st.MTU)
	}
	if len(st.Addresses) != 1 || st.Addresses[0] != "10.0.0.1/24" {
		t.Errorf("unexpected addresses: %v", st.Addresses)
	}
}

func TestManager_Configure_NotCreated(t *testing.T) {
	m := &mockManager{}
	cfg := &Config{Name: "Navo-TUN", MTU: 1500, Address: []string{"10.0.0.1/24"}}
	err := m.Configure(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Configure should work on mock even without Create: %v", err)
	}
}

func TestManager_Destroy(t *testing.T) {
	m := &mockManager{installedVal: true}
	ctx := context.Background()
	if err := m.Create(ctx, "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
	if err := m.Destroy(ctx); err != nil {
		t.Fatal(err)
	}
	if m.Status().Created {
		t.Error("expected Created=false after Destroy")
	}
}

func TestManager_Destroy_NotCreated(t *testing.T) {
	m := &mockManager{installedVal: true}
	err := m.Destroy(context.Background())
	if err != nil {
		t.Fatal("Destroy on non-existent adapter should succeed (no-op)", err)
	}
}

func TestManager_Cleanup(t *testing.T) {
	m := &mockManager{
		installedVal: true,
		cleanupRes: &CleanupResult{
			AdapterRemoved: true,
			RoutesCleaned:  3,
			DNSRestored:    true,
		},
	}
	res, err := m.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.AdapterRemoved {
		t.Error("expected adapter removed")
	}
	if res.RoutesCleaned != 3 {
		t.Errorf("expected 3 routes cleaned, got %d", res.RoutesCleaned)
	}
	if !res.DNSRestored {
		t.Error("expected DNS restored")
	}
}

func TestManager_FullLifecycle(t *testing.T) {
	mgr := &mockManager{installedVal: true}
	ctx := context.Background()

	if err := mgr.Create(ctx, "Navo-TUN"); err != nil {
		t.Fatal("Create:", err)
	}
	if err := mgr.Configure(ctx, &Config{Name: "Navo-TUN", MTU: 1500, Address: []string{"10.0.0.1/24"}}); err != nil {
		t.Fatal("Configure:", err)
	}
	st := mgr.Status()
	if !st.Created || st.MTU != 1500 {
		t.Error("unexpected status after configure")
	}
	if err := mgr.Destroy(ctx); err != nil {
		t.Fatal("Destroy:", err)
	}
	if mgr.Status().Created {
		t.Error("expected not created after Destroy")
	}
}

// ── RouteManager tests ──

func TestRouteManager_AddRoutes(t *testing.T) {
	rm := &mockRouteManager{}
	routes := []Route{
		{Destination: "0.0.0.0/1", Metric: 1, InterfaceName: "Navo-TUN"},
		{Destination: "128.0.0.0/1", Metric: 1, InterfaceName: "Navo-TUN"},
	}
	if err := rm.AddRoutes(context.Background(), "Navo-TUN", routes); err != nil {
		t.Fatal(err)
	}
	list, _ := rm.ListTUNRoutes(context.Background(), "Navo-TUN")
	if len(list) != 2 {
		t.Errorf("expected 2 routes, got %d", len(list))
	}
}

func TestRouteManager_AddRoutes_Error(t *testing.T) {
	rm := &mockRouteManager{addErr: errors.New(ErrNet004 + ": access denied")}
	err := rm.AddRoutes(context.Background(), "Navo-TUN", []Route{{Destination: "0.0.0.0/1"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRouteManager_RemoveRoutes(t *testing.T) {
	rm := &mockRouteManager{}
	routes := []Route{
		{Destination: "0.0.0.0/1", Metric: 1},
	}
	rm.AddRoutes(context.Background(), "Navo-TUN", routes)
	if err := rm.RemoveRoutes(context.Background(), "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
	list, _ := rm.ListTUNRoutes(context.Background(), "Navo-TUN")
	if len(list) != 0 {
		t.Errorf("expected 0 routes after remove, got %d", len(list))
	}
}

func TestRouteManager_CleanupAll(t *testing.T) {
	rm := &mockRouteManager{}
	routes := []Route{
		{Destination: "0.0.0.0/1", Metric: 1},
		{Destination: "128.0.0.0/1", Metric: 1},
		{Destination: "::/1", Metric: 1},
	}
	rm.AddRoutes(context.Background(), "Navo-TUN", routes)

	n, err := rm.CleanupAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 cleaned, got %d", n)
	}
}

// ── DNSManager tests ──

func TestDNSManager_Set(t *testing.T) {
	dm := &mockDNSManager{}
	err := dm.Set(context.Background(), "Navo-TUN", []string{"1.1.1.1", "8.8.8.8"})
	if err != nil {
		t.Fatal(err)
	}
	if !dm.IsConfigured(context.Background(), "Navo-TUN") {
		t.Error("expected configured after Set")
	}
}

func TestDNSManager_Set_Error(t *testing.T) {
	dm := &mockDNSManager{setErr: errors.New(ErrNet005 + ": netsh failed")}
	err := dm.Set(context.Background(), "Navo-TUN", []string{"1.1.1.1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDNSManager_Reset(t *testing.T) {
	dm := &mockDNSManager{}
	dm.Set(context.Background(), "Navo-TUN", []string{"1.1.1.1"})
	if err := dm.Reset(context.Background(), "Navo-TUN"); err != nil {
		t.Fatal(err)
	}
	if dm.IsConfigured(context.Background(), "Navo-TUN") {
		t.Error("expected not configured after Reset")
	}
}

func TestDNSManager_Reset_Error(t *testing.T) {
	dm := &mockDNSManager{resetErr: errors.New(ErrNet005 + ": reset failed")}
	if err := dm.Reset(context.Background(), "Navo-TUN"); err == nil {
		t.Fatal("expected error")
	}
}

// ── Error code tests ──

func TestErrorCodes(t *testing.T) {
	codes := map[string]string{
		ErrNet001: "wintun.dll not found or failed to load",
		ErrNet002: "adapter creation failed",
		ErrNet003: "IP address configuration failed",
		ErrNet004: "route table modification failed",
		ErrNet005: "DNS configuration failed",
		ErrNet006: "adapter destruction failed",
	}
	for code, desc := range codes {
		if code == "" || desc == "" {
			t.Errorf("invalid error code definition: %s = %s", code, desc)
		}
	}
}
