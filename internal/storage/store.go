// Package storage provides an in-memory key-value store for Phase 2.
// It will be replaced by SQLite in Phase 4.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"navo/internal/fsatomic"
)

// Store is a simple JSON-file-backed key-value store.
// Phase 2: in-memory with JSON persistence.
// Phase 4: replaced by SQLite.
type Store struct {
	mu    sync.RWMutex
	data  map[string]json.RawMessage
	path  string
	dirty bool
}

// NewStore creates a new Store. If path is empty, uses in-memory only.
func NewStore(path string) *Store {
	s := &Store{
		data: make(map[string]json.RawMessage),
		path: path,
	}

	if path != "" {
		s.load()
	}

	return s
}

// Get retrieves a value by key.
func (s *Store) Get(key string, v interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, ok := s.data[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	return json.Unmarshal(raw, v)
}

// Put stores a value under the given key.
func (s *Store) Put(key string, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	s.data[key] = raw
	s.dirty = true
	return nil
}

// Delete removes a key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	s.dirty = true
}

// Keys returns all stored keys.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Count returns the number of stored items.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Flush writes dirty data to disk.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" || !s.dirty {
		return nil
	}

	// Wrap data with metadata
	record := struct {
		UpdatedAt time.Time                  `json:"updated_at"`
		Data      map[string]json.RawMessage `json:"data"`
	}{
		UpdatedAt: time.Now(),
		Data:      s.data,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := fsatomic.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("persist store: %w", err)
	}

	s.dirty = false
	return nil
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	var record struct {
		Data map[string]json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(data, &record); err != nil {
		return
	}

	s.data = record.Data
}
