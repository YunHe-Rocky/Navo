package capture

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

// TransitionJournal is written before mutation and cleared only after commit or
// successful safe rollback.
type TransitionJournal struct {
	Version           int            `json:"version"`
	ID                string         `json:"id"`
	From              Mode           `json:"from"`
	To                Mode           `json:"to"`
	CurrentStep       Phase          `json:"current_step"`
	CreatedAdapterID  string         `json:"created_adapter_id,omitempty"`
	AddedRouteIDs     []string       `json:"added_route_ids,omitempty"`
	DNSBackup         []string       `json:"dns_backup,omitempty"`
	SystemProxyBackup map[string]any `json:"system_proxy_backup,omitempty"`
	CorePID           int            `json:"core_pid,omitempty"`
	StartedAt         time.Time      `json:"started_at"`
	Committed         bool           `json:"committed"`
}

type JournalStore struct {
	mu   sync.Mutex
	path string
}

func NewJournalStore(path string) *JournalStore {
	return &JournalStore{path: path}
}

func (s *JournalStore) Load() (*TransitionJournal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var value TransitionJournal
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode capture transition journal: %w", err)
	}
	if value.Version != 1 || value.ID == "" {
		return nil, fmt.Errorf("invalid capture transition journal")
	}
	return &value, nil
}

func (s *JournalStore) Save(value TransitionJournal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return fmt.Errorf("capture transition journal path is empty")
	}
	value.Version = 1
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create capture journal directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode capture transition journal: %w", err)
	}
	if err := fsatomic.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("replace capture transition journal: %w", err)
	}
	return nil
}

func (s *JournalStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove capture transition journal: %w", err)
	}
	return nil
}
