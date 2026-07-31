//go:build !windows

package tun

import (
	"context"
	"errors"
)

var errNotWindows = errors.New("TUN manager is only available on Windows")

type stubManager struct{}

func NewManager() Manager                                               { return &stubManager{} }
func NewManagerWithDLL(string) Manager                                  { return &stubManager{} }
func (m *stubManager) Create(ctx context.Context, name string) error    { return errNotWindows }
func (m *stubManager) Destroy(ctx context.Context) error                { return errNotWindows }
func (m *stubManager) Configure(ctx context.Context, cfg *Config) error { return errNotWindows }
func (m *stubManager) Status() *Status {
	return &Status{Name: "stub", Installed: false, Created: false}
}
func (m *stubManager) IsInstalled() bool { return false }
func (m *stubManager) Cleanup(ctx context.Context) (*CleanupResult, error) {
	return &CleanupResult{}, errNotWindows
}

type stubRouteManager struct{}

func NewRouteManager() RouteManager { return &stubRouteManager{} }
func (r *stubRouteManager) AddRoutes(ctx context.Context, adapterName string, routes []Route) error {
	return errNotWindows
}
func (r *stubRouteManager) RemoveRoutes(ctx context.Context, adapterName string) error {
	return errNotWindows
}
func (r *stubRouteManager) ListTUNRoutes(ctx context.Context, adapterName string) ([]Route, error) {
	return nil, errNotWindows
}
func (r *stubRouteManager) CleanupAll(ctx context.Context) (int, error) {
	return 0, errNotWindows
}

type stubDNSManager struct{}

func NewDNSManager() DNSManager { return &stubDNSManager{} }
func (d *stubDNSManager) Set(ctx context.Context, adapterName string, servers []string) error {
	return errNotWindows
}
func (d *stubDNSManager) Reset(ctx context.Context, adapterName string) error       { return errNotWindows }
func (d *stubDNSManager) IsConfigured(ctx context.Context, adapterName string) bool { return false }
