//go:build windows

package tun

import (
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"navo/internal/domain/capture"
)

type wintunManager struct {
	mu               sync.Mutex
	dll              *syscall.LazyDLL
	createAdapter    *syscall.LazyProc
	openAdapter      *syscall.LazyProc
	closeAdapter     *syscall.LazyProc
	adapter          syscall.Handle
	name             string
	cfg              Config
	createdByManager bool
}

// NewManager creates a Windows Wintun-based TUN manager.
func NewManager() Manager {
	return NewManagerWithDLL("wintun.dll")
}

// NewManagerWithDLL binds Wintun by an explicit path.
func NewManagerWithDLL(path string) Manager {
	dll := syscall.NewLazyDLL(path)
	return &wintunManager{
		dll:           dll,
		createAdapter: dll.NewProc("WintunCreateAdapter"),
		openAdapter:   dll.NewProc("WintunOpenAdapter"),
		closeAdapter:  dll.NewProc("WintunCloseAdapter"),
	}
}

func (m *wintunManager) IsInstalled() bool {
	err := m.dll.Load()
	return err == nil
}

func (m *wintunManager) Create(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.dll.Load(); err != nil {
		return fmt.Errorf("%s: wintun.dll not found: %w", ErrNet001, err)
	}
	if m.adapter != 0 {
		return nil
	}

	adapterName, _ := syscall.UTF16PtrFromString(name)
	tunnelType, _ := syscall.UTF16PtrFromString("sing-tun")
	requestedGUID := stableAdapterGUID(name)

	ret, _, _ := m.createAdapter.Call(
		uintptr(unsafe.Pointer(adapterName)),
		uintptr(unsafe.Pointer(tunnelType)),
		uintptr(unsafe.Pointer(requestedGUID)),
	)
	if ret == 0 {
		// sing-tun follows the same create-or-open strategy. Opening an existing
		// adapter keeps recovery idempotent after an interrupted transition.
		ret, _, _ = m.openAdapter.Call(uintptr(unsafe.Pointer(adapterName)))
		if ret == 0 {
			return fmt.Errorf("%s: create or open Wintun adapter %q failed", ErrNet002, name)
		}
		m.createdByManager = false
	} else {
		m.createdByManager = true
	}

	m.adapter = syscall.Handle(ret)
	m.name = name
	return nil
}

func (m *wintunManager) Destroy(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.adapter != 0 {
		m.closeAdapter.Call(uintptr(m.adapter))
		m.adapter = 0
	}
	// Driver lifetime is machine-global and must not be tied to one transition.
	m.createdByManager = false
	return nil
}

func (m *wintunManager) Configure(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.adapter == 0 {
		return fmt.Errorf("%s: adapter not created, cannot configure", ErrNet003)
	}

	// Set IP address for each address entry
	// netsh interface ip set address "<name>" static <addr> <mask> [gateway]
	for _, addr := range cfg.Address {
		parts := strings.Split(addr, "/")
		if len(parts) != 2 {
			return fmt.Errorf("%s: invalid address format %q, expected CIDR", ErrNet003, addr)
		}
		ip := parts[0]
		_, prefix, err := net.ParseCIDR(addr)
		if err != nil {
			return fmt.Errorf("%s: invalid address %q: %w", ErrNet003, addr, err)
		}
		mask := net.IP(prefix.Mask).String()

		args := []string{"interface", "ip", "set", "address", cfg.Name, "static", ip, mask}
		if cfg.Gateway != "" {
			args = append(args, cfg.Gateway)
		}
		cmd := hiddenCommandContext(ctx, "netsh", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: netsh set address failed: %v (output: %s)", ErrNet003, err, string(out))
		}
	}

	// Set MTU
	if cfg.MTU > 0 {
		cmd := hiddenCommandContext(ctx, "netsh",
			"interface", "ipv4", "set", "subinterface",
			cfg.Name,
			fmt.Sprintf("mtu=%d", cfg.MTU),
			"store=persistent",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: netsh set mtu failed: %v (output: %s)", ErrNet003, err, string(out))
		}
	}

	// Set DNS
	for _, dns := range cfg.DNS {
		cmd := hiddenCommandContext(ctx, "netsh",
			"interface", "ip", "set", "dns",
			cfg.Name, "static", dns,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: netsh set dns failed: %v (output: %s)", ErrNet005, err, string(out))
		}
	}

	m.cfg = *cfg
	return nil
}

func (m *wintunManager) Status() *Status {
	m.mu.Lock()
	name := m.name
	cfg := m.cfg
	handleOpen := m.adapter != 0
	m.mu.Unlock()
	if name == "" {
		name = "Navo"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	adapter := InspectAdapter(ctx, name)
	return &Status{
		Name:       adapter.Name,
		Identifier: adapter.InterfaceGUID,
		State:      string(adapter.State),
		Installed:  m.IsInstalled(),
		Created:    handleOpen || adapter.State != capture.AdapterMissing,
		Addresses:  cfg.Address,
		MTU:        cfg.MTU,
	}
}

func (m *wintunManager) Cleanup(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{}
	m.mu.Lock()
	hasHandle := m.adapter != 0
	m.mu.Unlock()
	if hasHandle {
		if err := m.Destroy(ctx); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("destroy adapter: %v", err))
		} else {
			result.AdapterRemoved = true
		}
	} else {
		result.AdapterRemoved = true // nothing to clean
	}

	return result, nil
}

// stableAdapterGUID matches sing-tun's deterministic Windows adapter identity.
func stableAdapterGUID(name string) *windows.GUID {
	sum := md5.Sum([]byte("wintun" + name))
	return (*windows.GUID)(unsafe.Pointer(&sum[0]))
}
