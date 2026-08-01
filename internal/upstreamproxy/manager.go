package upstreamproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"navo/internal/domain/endpoint"
	"navo/internal/fsatomic"
)

type Manager struct {
	path   string
	mu     sync.RWMutex
	values map[string]endpoint.UpstreamProxy
}

func NewManager(path string) (*Manager, error) {
	manager := &Manager{path: path, values: make(map[string]endpoint.UpstreamProxy)}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Add(value endpoint.UpstreamProxy) error {
	if err := value.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.values[value.ID]; exists {
		return fmt.Errorf("upstream proxy %q already exists", value.ID)
	}
	m.values[value.ID] = value
	if err := m.saveLocked(); err != nil {
		delete(m.values, value.ID)
		return err
	}
	return nil
}

func (m *Manager) Update(value endpoint.UpstreamProxy) error {
	if err := value.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous, exists := m.values[value.ID]
	if !exists {
		return fmt.Errorf("upstream proxy %q not found", value.ID)
	}
	m.values[value.ID] = value
	if err := m.saveLocked(); err != nil {
		m.values[value.ID] = previous
		return err
	}
	return nil
}

func (m *Manager) Remove(id string) (endpoint.UpstreamProxy, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, exists := m.values[id]
	if !exists {
		return endpoint.UpstreamProxy{}, false, nil
	}
	delete(m.values, id)
	if err := m.saveLocked(); err != nil {
		m.values[id] = value
		return endpoint.UpstreamProxy{}, false, err
	}
	return value, true, nil
}

func (m *Manager) Get(id string) (endpoint.UpstreamProxy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.values[id]
	return value, exists
}

func (m *Manager) List() []endpoint.UpstreamProxy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]endpoint.UpstreamProxy, 0, len(m.values))
	for _, value := range m.values {
		values = append(values, value)
	}
	return values
}

func (m *Manager) load() error {
	if strings.TrimSpace(m.path) == "" {
		return nil
	}
	data, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read upstream proxies: %w", err)
	}
	var values []endpoint.UpstreamProxy
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("decode upstream proxies: %w", err)
	}
	for _, value := range values {
		if err := value.Validate(); err != nil {
			return fmt.Errorf("stored upstream proxy %q is invalid: %w", value.ID, err)
		}
		m.values[value.ID] = value
	}
	return nil
}

func (m *Manager) saveLocked() error {
	if strings.TrimSpace(m.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return fmt.Errorf("create upstream proxy directory: %w", err)
	}
	values := make([]endpoint.UpstreamProxy, 0, len(m.values))
	for _, value := range m.values {
		values = append(values, value)
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("encode upstream proxies: %w", err)
	}
	if err := fsatomic.WriteFile(m.path, data, 0600); err != nil {
		return fmt.Errorf("write upstream proxies: %w", err)
	}
	return nil
}
