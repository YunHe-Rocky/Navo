package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"navo/internal/fsatomic"
)

type actionStatus string

const (
	actionPending actionStatus = "pending"
	actionApplied actionStatus = "applied"
)

type journalResourceKind string

const (
	resourceEndpointRoute journalResourceKind = "endpoint_route"
	resourceSplitRoute    journalResourceKind = "split_route"
	resourceNRPTRule      journalResourceKind = "nrpt_rule"
	resourceFirewallRule  journalResourceKind = "firewall_rule"
)

// journalResource is the only recovery authority. V2 never executes a
// command persisted on disk.
type journalResource struct {
	Kind                journalResourceKind `json:"kind"`
	DestinationPrefix   string              `json:"destination_prefix,omitempty"`
	AddressFamily       string              `json:"address_family,omitempty"`
	InterfaceIndex      uint32              `json:"interface_index,omitempty"`
	InterfaceGUID       string              `json:"interface_guid,omitempty"`
	InterfaceAlias      string              `json:"interface_alias,omitempty"`
	NextHop             string              `json:"next_hop,omitempty"`
	RouteMetric         int                 `json:"route_metric,omitempty"`
	NRPTNamespace       string              `json:"nrpt_namespace,omitempty"`
	NRPTComment         string              `json:"nrpt_comment,omitempty"`
	NameServers         []string            `json:"name_servers,omitempty"`
	FirewallDisplayName string              `json:"firewall_display_name,omitempty"`
	CreatedByNavo       bool                `json:"created_by_navo"`
}

type journalAction struct {
	Name     string          `json:"name"`
	Status   actionStatus    `json:"status"`
	Resource journalResource `json:"resource"`
}

type legacyJournalAction struct {
	Name   string       `json:"name"`
	Undo   Command      `json:"undo"`
	Status actionStatus `json:"status"`
}

type legacyJournal struct {
	Version     int                   `json:"version"`
	AdapterName string                `json:"adapter_name"`
	SessionID   string                `json:"session_id"`
	CreatedAt   time.Time             `json:"created_at"`
	Actions     []legacyJournalAction `json:"actions"`
}

type journal struct {
	Version     int               `json:"version"`
	AdapterName string            `json:"adapter_name"`
	SessionID   string            `json:"session_id"`
	CreatedAt   time.Time         `json:"created_at"`
	Plan        TUNActivationPlan `json:"plan,omitempty"`
	Adapter     AdapterSnapshot   `json:"adapter,omitempty"`
	Actions     []journalAction   `json:"actions"`
}

func readJournal(path string) (*journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode network journal header: %w", err)
	}
	if header.Version != 1 && header.Version != 2 {
		return nil, fmt.Errorf("unsupported network journal version %d", header.Version)
	}
	if header.Version == 1 {
		var legacy legacyJournal
		if err := json.Unmarshal(data, &legacy); err != nil {
			return nil, fmt.Errorf("decode legacy network journal: %w", err)
		}
		value := &journal{Version: 1, AdapterName: legacy.AdapterName, SessionID: legacy.SessionID, CreatedAt: legacy.CreatedAt}
		for _, action := range legacy.Actions {
			value.Actions = append(value.Actions, journalAction{Name: action.Name, Status: action.Status})
		}
		return value, nil
	}
	var value journal
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode network journal: %w", err)
	}
	return &value, nil
}

func writeJournal(path string, value *journal) error {
	if path == "" {
		return fmt.Errorf("network journal path is empty")
	}
	if value == nil || value.Version != 2 {
		return fmt.Errorf("only network journal V2 can be persisted")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode network journal: %w", err)
	}
	if err := fsatomic.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("replace journal: %w", err)
	}
	return nil
}
