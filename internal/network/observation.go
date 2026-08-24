package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

// ResourceObservation summarizes only resources represented by the V2
// Journal. It is read-only and deliberately omits third-party configuration.
type ResourceObservation struct {
	Known         bool   `json:"known"`
	Coherent      bool   `json:"coherent"`
	OwnedCount    int    `json:"owned_count"`
	ExistingCount int    `json:"existing_count"`
	MissingCount  int    `json:"missing_count"`
	ConflictCount int    `json:"conflict_count"`
	LastError     string `json:"last_error,omitempty"`
}

type JournalObservation struct {
	Present              bool   `json:"present"`
	Dirty                bool   `json:"dirty"`
	Version              int    `json:"version,omitempty"`
	SessionID            string `json:"session_id,omitempty"`
	AdapterName          string `json:"adapter_name,omitempty"`
	AdapterGUID          string `json:"adapter_guid,omitempty"`
	AdapterIndex         int    `json:"adapter_index,omitempty"`
	OwnedResources       int    `json:"owned_resources"`
	PreexistingResources int    `json:"preexisting_resources"`
	PendingActions       int    `json:"pending_actions"`
	MissingResources     int    `json:"missing_resources"`
	ConflictingResources int    `json:"conflicting_resources"`
	LastError            string `json:"last_error,omitempty"`
}

type OwnedNetworkObservation struct {
	Journal  JournalObservation  `json:"journal"`
	Routes   ResourceObservation `json:"routes"`
	NRPT     ResourceObservation `json:"nrpt"`
	Firewall ResourceObservation `json:"firewall"`
	Errors   []string            `json:"errors,omitempty"`
}

// Observe reads and validates the current Journal, then checks each recorded
// resource with fixed read-only commands. It never calls Recover, applies a
// command, or removes a resource.
func (m *Manager) Observe(ctx context.Context) (OwnedNetworkObservation, error) {
	result := cleanOwnedNetworkObservation()
	if m == nil || !m.cfg.Enabled {
		return result, nil
	}
	if ctx == nil {
		return result, fmt.Errorf("network observation context is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	value, err := readJournal(m.cfg.JournalPath)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	result.Journal.Present = true
	if err != nil {
		result.Journal.Dirty = true
		result.Journal.LastError = err.Error()
		return result, fmt.Errorf("read network journal observation: %w", err)
	}
	result.Journal = JournalObservation{
		Present: true, Version: value.Version, SessionID: value.SessionID,
		AdapterName: value.AdapterName, AdapterGUID: value.Adapter.InterfaceGUID,
		AdapterIndex: int(value.Adapter.InterfaceIndex),
	}
	if value.Version != 2 {
		result.Journal.Dirty = true
		result.Journal.LastError = "legacy network journal requires ownership-safe recovery"
		return result, errors.New(result.Journal.LastError)
	}
	if err := m.validateRecoveryJournal(value); err != nil {
		result.Journal.Dirty = true
		result.Journal.LastError = err.Error()
		return result, fmt.Errorf("validate network journal observation: %w", err)
	}
	if m.executor == nil {
		result.Journal.Dirty = true
		result.Journal.LastError = "network observation executor is unavailable"
		return result, errors.New(result.Journal.LastError)
	}

	prepared := make([]preparedResourceObservation, 0, len(value.Actions))
	for _, action := range value.Actions {
		resource := resourceObservationFor(&result, action.Resource.Kind)
		if action.Resource.CreatedByNavo {
			resource.OwnedCount++
			result.Journal.OwnedResources++
		} else {
			result.Journal.PreexistingResources++
		}
		if action.Status != actionApplied {
			result.Journal.PendingActions++
		}
		script, scriptErr := observationStateScript(action.Resource)
		if scriptErr != nil {
			observeResourceError(&result, resource, action.Name, scriptErr)
			continue
		}
		prepared = append(prepared, preparedResourceObservation{action: action, resource: resource, script: script})
	}
	if len(prepared) > 0 {
		output, inspectErr := m.executor.RunOutput(ctx, observationBatchCommand(prepared))
		if inspectErr != nil {
			for _, item := range prepared {
				observeResourceError(&result, item.resource, item.action.Name, inspectErr)
			}
		} else {
			states, decodeErr := decodeObservationBatch(output, len(prepared))
			if decodeErr != nil {
				for _, item := range prepared {
					observeResourceError(&result, item.resource, item.action.Name, decodeErr)
				}
			} else {
				for index, item := range prepared {
					state := states[index]
					if state.Error != "" {
						observeResourceError(&result, item.resource, item.action.Name, errors.New(state.Error))
						continue
					}
					applyObservationState(&result, item.resource, item.action, state.State)
				}
			}
		}
	}
	finalizeResourceObservation(&result.Routes)
	finalizeResourceObservation(&result.NRPT)
	finalizeResourceObservation(&result.Firewall)
	result.Journal.Dirty = result.Journal.PendingActions > 0 ||
		result.Journal.MissingResources > 0 || result.Journal.ConflictingResources > 0 ||
		len(result.Errors) > 0
	if len(result.Errors) > 0 {
		result.Journal.LastError = strings.Join(result.Errors, "; ")
	}
	return result, nil
}

func cleanOwnedNetworkObservation() OwnedNetworkObservation {
	return OwnedNetworkObservation{
		Routes:   ResourceObservation{Known: true, Coherent: true},
		NRPT:     ResourceObservation{Known: true, Coherent: true},
		Firewall: ResourceObservation{Known: true, Coherent: true},
	}
}

func resourceObservationFor(result *OwnedNetworkObservation, kind journalResourceKind) *ResourceObservation {
	switch kind {
	case resourceEndpointRoute, resourceSplitRoute:
		return &result.Routes
	case resourceNRPTRule:
		return &result.NRPT
	default:
		return &result.Firewall
	}
}

func observeResourceError(result *OwnedNetworkObservation, resource *ResourceObservation, name string, err error) {
	message := fmt.Sprintf("observe %s: %v", name, err)
	resource.ConflictCount++
	resource.LastError = message
	result.Journal.ConflictingResources++
	result.Errors = append(result.Errors, message)
}

func finalizeResourceObservation(resource *ResourceObservation) {
	resource.Coherent = resource.MissingCount == 0 && resource.ConflictCount == 0
}

type preparedResourceObservation struct {
	action   journalAction
	resource *ResourceObservation
	script   string
}

type observationBatchResult struct {
	Index int    `json:"index"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

func observationBatchCommand(items []preparedResourceObservation) Command {
	var script strings.Builder
	script.WriteString("# NAVO_NETWORK_OBSERVATION_BATCH\n")
	script.WriteString("$results=[System.Collections.Generic.List[object]]::new();")
	for index, item := range items {
		fmt.Fprintf(&script, "\n# NAVO_NETWORK_OBSERVATION_ITEM %d\n", index)
		script.WriteString("try{$state=[string](&{")
		script.WriteString(item.script)
		script.WriteString("});if($state -notin @('EXACT','MISSING','CONFLICT')){throw ('unexpected observation state '+$state)};")
		fmt.Fprintf(&script, "$results.Add([pscustomobject]@{index=%d;state=$state;error=''})|Out-Null", index)
		script.WriteString("}catch{")
		fmt.Fprintf(&script, "$results.Add([pscustomobject]@{index=%d;state='';error=[string]$_.Exception.Message})|Out-Null}", index)
	}
	script.WriteString(";[pscustomobject]@{results=@($results)}|ConvertTo-Json -Compress -Depth 4")
	return powershell(script.String())
}

func decodeObservationBatch(output string, expected int) ([]observationBatchResult, error) {
	var envelope struct {
		Results []observationBatchResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &envelope); err != nil {
		return nil, fmt.Errorf("decode network observation batch: %w", err)
	}
	if len(envelope.Results) != expected {
		return nil, fmt.Errorf("network observation batch returned %d results, want %d", len(envelope.Results), expected)
	}
	ordered := make([]observationBatchResult, expected)
	seen := make([]bool, expected)
	for _, result := range envelope.Results {
		if result.Index < 0 || result.Index >= expected {
			return nil, fmt.Errorf("network observation batch returned invalid index %d", result.Index)
		}
		if seen[result.Index] {
			return nil, fmt.Errorf("network observation batch returned duplicate index %d", result.Index)
		}
		result.State = strings.TrimSpace(result.State)
		result.Error = strings.TrimSpace(result.Error)
		if result.Error == "" && result.State != "EXACT" && result.State != "MISSING" && result.State != "CONFLICT" {
			return nil, fmt.Errorf("network observation batch returned invalid state %q", result.State)
		}
		seen[result.Index] = true
		ordered[result.Index] = result
	}
	return ordered, nil
}

func applyObservationState(result *OwnedNetworkObservation, resource *ResourceObservation, action journalAction, state string) {
	switch state {
	case "EXACT":
		resource.ExistingCount++
	case "MISSING":
		if action.Resource.CreatedByNavo {
			resource.MissingCount++
			result.Journal.MissingResources++
		} else {
			resource.ConflictCount++
			result.Journal.ConflictingResources++
		}
	case "CONFLICT":
		resource.ConflictCount++
		result.Journal.ConflictingResources++
	}
}

func observationStateScript(resource journalResource) (string, error) {
	if err := validateJournalResource(resource); err != nil {
		return "", err
	}
	switch resource.Kind {
	case resourceEndpointRoute, resourceSplitRoute:
		selector := routeSelector(resource)
		script := "$all=@(Get-NetRoute -AddressFamily " + resource.AddressFamily + " -DestinationPrefix " + psQuote(resource.DestinationPrefix) + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue);" +
			"$exact=@($all|Where-Object {" + selector + "});if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT'}elseif($all.Count -gt 0){'CONFLICT'}else{'MISSING'}"
		return script, nil
	case resourceNRPTRule:
		if len(resource.NameServers) == 0 || net.ParseIP(resource.NameServers[0]) == nil {
			return "", fmt.Errorf("NRPT observation has no valid nameserver")
		}
		script := "$all=@(Get-DnsClientNrptRule -ErrorAction SilentlyContinue|Where-Object {(@($_.Namespace) -contains '.')});" +
			"$exact=@($all|Where-Object {[string]$_.Comment -eq " + psQuote(resource.NRPTComment) + " -and (@($_.NameServers) -contains " + psQuote(resource.NameServers[0]) + ")});" +
			"if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT'}elseif($all.Count -gt 0){'CONFLICT'}else{'MISSING'}"
		return script, nil
	case resourceFirewallRule:
		script := "$all=@(Get-NetFirewallRule -DisplayName " + psQuote(resource.FirewallDisplayName) + " -ErrorAction SilentlyContinue);" +
			"$exact=@($all|Where-Object {[string]$_.Enabled -eq 'True' -and [string]$_.Direction -eq 'Outbound' -and [string]$_.Action -eq 'Block'});" +
			"if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT'}elseif($all.Count -gt 0){'CONFLICT'}else{'MISSING'}"
		return script, nil
	default:
		return "", fmt.Errorf("unsupported observed resource kind %q", resource.Kind)
	}
}
