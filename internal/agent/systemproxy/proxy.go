// Package systemproxy manages Windows system proxy settings.
// It controls the WinHTTP/WinINET proxy configuration via the Windows Registry.
//
// Phase 2: Windows-only. On other platforms, all operations are no-ops.
package systemproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProxyConfig holds a snapshot of system proxy state.
type ProxyConfig struct {
	Enabled       bool   `json:"enabled"`
	ProxyServer   string `json:"proxy_server"`
	BypassList    string `json:"bypass_list,omitempty"`
	AutoConfigURL string `json:"auto_config_url,omitempty"`
	AutoDetect    bool   `json:"auto_detect"`
}

// Manager manages Windows system proxy settings.
type Manager struct {
	mu           sync.Mutex
	active       bool
	currentProxy string
	backupPath   string
	ownerPath    string
	getProxy     func() (*ProxyConfig, error)
	setProxy     func(string) error
	applyProxy   func(ProxyConfig) error
	notify       func() error
}

type ownershipRecord struct {
	ProxyServer string `json:"proxy_server"`
}

// NewManager creates a new system proxy manager.
func NewManager() *Manager {
	return NewManagerWithDirectory(filepath.Join(os.TempDir(), "navo"))
}

// NewManagerWithDirectory isolates proxy ownership state for one profile.
func NewManagerWithDirectory(backupDir string) *Manager {
	_ = os.MkdirAll(backupDir, 0o700)
	return &Manager{
		backupPath: filepath.Join(backupDir, "proxy_backup.json"),
		ownerPath:  filepath.Join(backupDir, "proxy_owner.json"),
		getProxy:   getSystemProxy,
		setProxy:   setSystemProxy,
		applyProxy: applySystemProxyConfig,
		notify:     notifyProxyChange,
	}
}

// Enable sets the system proxy to the given endpoint.
func (m *Manager) Enable(proxyServer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if proxyServer == "" {
		return fmt.Errorf("proxy server address is empty")
	}
	if m.active && m.currentProxy == proxyServer {
		if current, err := m.getProxy(); err == nil && m.owns(*current) {
			return nil
		}
		m.active = false
		m.currentProxy = ""
	}

	// Preserve the user's original state only once for this ownership session.
	if _, err := os.Stat(m.ownerPath); err == nil {
		return fmt.Errorf("stale proxy ownership must be recovered before enabling")
	} else if os.IsNotExist(err) {
		if err := m.backup(); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	} else {
		return fmt.Errorf("read proxy ownership: %w", err)
	}

	// Set proxy via registry
	if err := m.setProxy(proxyServer); err != nil {
		return fmt.Errorf("set proxy failed: %w", err)
	}

	// Notify Windows of the change
	if err := m.notify(); err != nil {
		// Non-fatal: proxy is set but notification failed
	}

	m.active = true
	m.currentProxy = proxyServer
	owner, _ := json.Marshal(map[string]interface{}{
		"proxy_server": proxyServer,
		"owned_at":     time.Now().UTC(),
	})
	if err := os.WriteFile(m.ownerPath, owner, 0600); err != nil {
		_ = m.restore()
		return fmt.Errorf("persist proxy ownership: %w", err)
	}

	return nil
}

// Disable relinquishes Navo ownership and restores the user's original state.
func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.ownerPath); err == nil {
		if err := m.restoreOwned(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read proxy ownership: %w", err)
	} else {
		// Another application may own the current WinINET proxy. Without a Navo
		// ownership record, disabling it would corrupt unrelated user state.
		m.active = false
		m.currentProxy = ""
		return nil
	}

	if err := m.notify(); err != nil {
		// Non-fatal
	}

	m.active = false
	m.currentProxy = ""

	return nil
}

// Backup saves the current proxy configuration to disk.
func (m *Manager) Backup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.backup()
}

func (m *Manager) backup() error {
	cfg, err := m.getProxy()
	if err != nil {
		// Can't read current state - save what we know
		cfg = &ProxyConfig{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.backupPath, data, 0644)
}

// Restore restores the system proxy from the last backup.
func (m *Manager) Restore() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restore()
}

func (m *Manager) restore() error {
	data, err := os.ReadFile(m.backupPath)
	if err != nil {
		return fmt.Errorf("no backup found: %w", err)
	}

	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("corrupted backup: %w", err)
	}

	if err := m.applyProxy(cfg); err != nil {
		return fmt.Errorf("restore system proxy: %w", err)
	}
	if err := m.notify(); err != nil {
		return fmt.Errorf("notify proxy restoration: %w", err)
	}
	if err := os.Remove(m.ownerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear proxy ownership: %w", err)
	}
	m.active = false
	m.currentProxy = ""
	return nil
}

func (m *Manager) restoreOwned() error {
	ownerData, err := os.ReadFile(m.ownerPath)
	if err != nil {
		return fmt.Errorf("read proxy ownership: %w", err)
	}
	var owner ownershipRecord
	if err := json.Unmarshal(ownerData, &owner); err != nil || owner.ProxyServer == "" {
		return fmt.Errorf("corrupted proxy ownership record")
	}
	current, err := m.getProxy()
	if err != nil {
		return fmt.Errorf("read current system proxy: %w", err)
	}
	if !ownershipMatchesCurrent(owner, *current) {
		// Another application changed WinINet after Navo. Relinquish only the
		// ownership marker and preserve the newer external configuration.
		if err := os.Remove(m.ownerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear stale proxy ownership: %w", err)
		}
		m.active = false
		m.currentProxy = ""
		return nil
	}
	return m.restore()
}

// RecoverOwned restores a snapshot left by an unclean Navo shutdown.
func (m *Manager) RecoverOwned() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Stat(m.ownerPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read proxy ownership: %w", err)
	}
	return m.restoreOwned()
}

// Status returns the current proxy state.
func (m *Manager) Status() ProxyConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, err := m.getProxy()
	if err != nil {
		return ProxyConfig{Enabled: m.active, ProxyServer: m.currentProxy}
	}
	cfg.Enabled = m.owns(*cfg)
	return *cfg
}

func (m *Manager) owns(cfg ProxyConfig) bool {
	if !cfg.Enabled || cfg.ProxyServer == "" {
		return false
	}
	data, err := os.ReadFile(m.ownerPath)
	if err != nil {
		return false
	}
	var owner ownershipRecord
	if json.Unmarshal(data, &owner) != nil {
		return false
	}
	return ownershipMatchesCurrent(owner, cfg)
}

func ownershipMatchesCurrent(owner ownershipRecord, current ProxyConfig) bool {
	return current.Enabled &&
		owner.ProxyServer != "" &&
		owner.ProxyServer == current.ProxyServer
}

// IsActive returns whether the proxy is currently enabled.
func (m *Manager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}
