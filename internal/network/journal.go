package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type actionStatus string

const (
	actionPending actionStatus = "pending"
	actionApplied actionStatus = "applied"
)

type journalAction struct {
	Name   string       `json:"name"`
	Undo   Command      `json:"undo"`
	Status actionStatus `json:"status"`
}

type journal struct {
	Version     int             `json:"version"`
	AdapterName string          `json:"adapter_name"`
	SessionID   string          `json:"session_id"`
	CreatedAt   time.Time       `json:"created_at"`
	Actions     []journalAction `json:"actions"`
}

func readJournal(path string) (*journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value journal
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode network journal: %w", err)
	}
	if value.Version != 1 {
		return nil, fmt.Errorf("unsupported network journal version %d", value.Version)
	}
	return &value, nil
}

func writeJournal(path string, value *journal) error {
	if path == "" {
		return fmt.Errorf("network journal path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode network journal: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".network-journal-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary journal: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure journal: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write journal: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync journal: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace journal: %w", err)
	}
	return nil
}
