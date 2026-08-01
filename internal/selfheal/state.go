package selfheal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"navo/internal/fsatomic"
)

const stateVersion = 1

type circuitRecord struct {
	ErrorCode    ErrorCode `json:"error_code"`
	ResourceHash string    `json:"resource_hash"`
	Attempts     int       `json:"attempts"`
	WindowStart  time.Time `json:"window_start"`
	LastAttempt  time.Time `json:"last_attempt,omitempty"`
	OpenUntil    time.Time `json:"open_until,omitempty"`
	HalfOpen     bool      `json:"half_open,omitempty"`
	LastResult   string    `json:"last_result,omitempty"`
}

type stateDocument struct {
	Version int                      `json:"version"`
	Records map[string]circuitRecord `json:"records"`
}

type budgetStore struct {
	mu      sync.Mutex
	path    string
	records map[string]circuitRecord
	now     func() time.Time
}

func newBudgetStore(path string) (*budgetStore, error) {
	b := &budgetStore{path: path, records: make(map[string]circuitRecord), now: time.Now}
	if err := b.load(); err != nil {
		return nil, err
	}
	return b, nil
}

func eventKey(event ErrorEvent) (string, string) {
	raw := string(event.Code) + "\x00" + event.ResourceID + "\x00" + event.CoreID + "\x00" + event.OutboundID
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	return hash, hash[:16]
}

func (b *budgetStore) begin(event ErrorEvent, budget Budget) (attempt int, state string, allowed bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if budget.MaxAttempts <= 0 {
		return 0, "disabled", false, nil
	}
	now := b.now().UTC()
	key, resourceHash := eventKey(event)
	record := b.records[key]
	record.ErrorCode = event.Code
	record.ResourceHash = resourceHash
	if record.WindowStart.IsZero() || now.Sub(record.WindowStart) >= budget.Window {
		record.Attempts = 0
		record.WindowStart = now
		record.OpenUntil = time.Time{}
		record.HalfOpen = false
	}
	if !record.OpenUntil.IsZero() {
		if now.Before(record.OpenUntil) {
			return record.Attempts, "open", false, nil
		}
		if record.HalfOpen {
			return record.Attempts, "half_open_busy", false, nil
		}
		record.HalfOpen = true
		b.records[key] = record
		if err := b.persistLocked(); err != nil {
			return 0, "", false, err
		}
		return record.Attempts + 1, "half_open", true, nil
	}
	if record.Attempts >= budget.MaxAttempts {
		record.OpenUntil = now.Add(budget.Cooldown)
		b.records[key] = record
		if err := b.persistLocked(); err != nil {
			return 0, "", false, err
		}
		return record.Attempts, "opened", false, nil
	}
	return record.Attempts + 1, "closed", true, nil
}

func (b *budgetStore) complete(event ErrorEvent, budget Budget, success bool) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now().UTC()
	key, resourceHash := eventKey(event)
	record := b.records[key]
	record.ErrorCode = event.Code
	record.ResourceHash = resourceHash
	record.LastAttempt = now
	if success {
		record.Attempts = 0
		record.OpenUntil = time.Time{}
		record.HalfOpen = false
		record.LastResult = "succeeded"
		b.records[key] = record
		return "closed", b.persistLocked()
	}
	record.Attempts++
	record.HalfOpen = false
	record.LastResult = "failed"
	state := "closed"
	if record.Attempts >= budget.MaxAttempts {
		record.OpenUntil = now.Add(budget.Cooldown)
		state = "opened"
	}
	b.records[key] = record
	return state, b.persistLocked()
}

func (b *budgetStore) load() error {
	if b.path == "" {
		return nil
	}
	data, err := os.ReadFile(b.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read self-heal state: %w", err)
	}
	var document stateDocument
	if err := json.Unmarshal(data, &document); err != nil || document.Version != stateVersion {
		quarantine := b.path + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		if renameErr := os.Rename(b.path, quarantine); renameErr != nil {
			return fmt.Errorf("quarantine corrupt self-heal state: %w", renameErr)
		}
		return nil
	}
	if document.Records != nil {
		b.records = document.Records
	}
	return nil
}

func (b *budgetStore) persistLocked() error {
	if b.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(stateDocument{Version: stateVersion, Records: b.records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode self-heal state: %w", err)
	}
	if err := fsatomic.WriteFile(filepath.Clean(b.path), data, 0600); err != nil {
		return fmt.Errorf("persist self-heal state: %w", err)
	}
	return nil
}
