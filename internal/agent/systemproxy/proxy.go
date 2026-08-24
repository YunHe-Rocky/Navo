// Package systemproxy manages Windows system proxy settings.
// It controls the WinHTTP/WinINET proxy configuration via the Windows Registry.
//
// Phase 2: Windows-only. On other platforms, all operations are no-ops.
package systemproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"navo/internal/fsatomic"
)

// ProxyConfig holds a snapshot of system proxy state.
type ProxyConfig struct {
	Enabled              bool   `json:"enabled"`
	ProxyServer          string `json:"proxy_server"`
	BypassList           string `json:"bypass_list,omitempty"`
	AutoConfigURL        string `json:"auto_config_url,omitempty"`
	AutoDetect           bool   `json:"auto_detect"`
	ProxyServerPresent   bool   `json:"proxy_server_present,omitempty"`
	BypassListPresent    bool   `json:"bypass_list_present,omitempty"`
	AutoConfigURLPresent bool   `json:"auto_config_url_present,omitempty"`
	AutoDetectPresent    bool   `json:"auto_detect_present,omitempty"`
}

// Manager manages Windows system proxy settings.
type Manager struct {
	mu           sync.Mutex
	active       bool
	currentProxy string
	backupPath   string
	ownerPath    string
	getProxy     func() (*ProxyConfig, error)
	applyProxy   func(ProxyConfig) error
	notify       func() error
}

type ownershipRecord struct {
	ProxyServer string `json:"proxy_server"`
	Phase       string `json:"phase,omitempty"`
}

// OwnershipStatus is a read-only comparison of Navo's marker and WinINet.
type OwnershipStatus struct {
	Present     bool   `json:"present"`
	Owned       bool   `json:"owned"`
	Lost        bool   `json:"lost"`
	ProxyServer string `json:"proxy_server,omitempty"`
	Phase       string `json:"phase,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

// CurrentConfig reads the current-user WinINet proxy without taking ownership
// or changing any registry value.
func CurrentConfig() (ProxyConfig, error) {
	config, err := getSystemProxy()
	if err != nil {
		return ProxyConfig{}, err
	}
	return *config, nil
}

const (
	ownershipPending   = "pending"
	ownershipCommitted = "committed"
)

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

	if err := m.writeOwnership(proxyServer, ownershipPending); err != nil {
		return fmt.Errorf("persist pending proxy ownership: %w", err)
	}

	if err := m.applyProxy(ownedProxyConfig(proxyServer)); err != nil {
		return errors.Join(fmt.Errorf("set proxy failed: %w", err), m.restore())
	}
	if err := m.notify(); err != nil {
		return errors.Join(fmt.Errorf("notify proxy change: %w", err), m.restore())
	}
	if err := m.writeOwnership(proxyServer, ownershipCommitted); err != nil {
		return errors.Join(fmt.Errorf("commit proxy ownership: %w", err), m.restore())
	}
	m.active = true
	m.currentProxy = proxyServer

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
		return fmt.Errorf("read current system proxy: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fsatomic.WriteFile(m.backupPath, data, 0o600)
}

func (m *Manager) writeOwnership(proxyServer, phase string) error {
	owner, err := json.Marshal(struct {
		ProxyServer string    `json:"proxy_server"`
		Phase       string    `json:"phase"`
		UpdatedAt   time.Time `json:"updated_at"`
	}{ProxyServer: proxyServer, Phase: phase, UpdatedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return fsatomic.WriteFile(m.ownerPath, owner, 0o600)
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

// OwnershipStatus compares the marker with raw WinINet without mutation.
func (m *Manager) OwnershipStatus() OwnershipStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.ownerPath)
	if os.IsNotExist(err) {
		return OwnershipStatus{}
	}
	if err != nil {
		return OwnershipStatus{LastError: fmt.Sprintf("read proxy ownership: %v", err)}
	}
	status := OwnershipStatus{Present: true}
	var owner ownershipRecord
	if err := json.Unmarshal(data, &owner); err != nil || owner.ProxyServer == "" {
		status.Lost = true
		status.LastError = "corrupted proxy ownership record"
		return status
	}
	status.ProxyServer = owner.ProxyServer
	status.Phase = owner.Phase
	current, err := m.getProxy()
	if err != nil {
		status.LastError = fmt.Sprintf("read current system proxy: %v", err)
		return status
	}
	status.Owned = (owner.Phase == "" || owner.Phase == ownershipCommitted) && ownershipMatchesCurrent(owner, *current)
	status.Lost = !status.Owned
	return status
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
	if owner.Phase != "" && owner.Phase != ownershipCommitted {
		return false
	}
	return ownershipMatchesCurrent(owner, cfg)
}

func ownershipMatchesCurrent(owner ownershipRecord, current ProxyConfig) bool {
	return current.Enabled &&
		owner.ProxyServer != "" &&
		owner.ProxyServer == current.ProxyServer &&
		current.AutoConfigURL == "" &&
		!current.AutoDetect
}

// ownedProxyConfig removes PAC/WPAD and stale bypass rules only while Navo
// owns WinINet. Disable restores the exact pre-existing snapshot.
func ownedProxyConfig(proxyServer string) ProxyConfig {
	return ProxyConfig{
		Enabled:              true,
		ProxyServer:          proxyServer,
		ProxyServerPresent:   true,
		BypassList:           "<local>",
		BypassListPresent:    true,
		AutoConfigURLPresent: false,
		AutoDetect:           false,
		AutoDetectPresent:    true,
	}
}

// IsActive returns whether the proxy is currently enabled.
func (m *Manager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}
