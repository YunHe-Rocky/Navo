package service

import (
	"context"
	"strings"
	"time"

	"navo/internal/domain/capture"
	"navo/internal/network"
	"navo/internal/network/tun"
	"navo/internal/networkenv"
)

const serviceNetworkObservationTimeout = 5 * time.Second

type platformNetworkObservation struct {
	Physical networkenv.PhysicalSnapshot
	External []networkenv.ExternalAdapterRef
}

func (s *Service) handleNetworkObserve(parent context.Context, requestID string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(parent, serviceNetworkObservationTimeout)
	defer cancel()
	observation := s.observeMachineNetwork(ctx)
	return response(requestID, map[string]interface{}{"environment": observation})
}

func (s *Service) observeMachineNetwork(ctx context.Context) networkenv.MachineSnapshot {
	result := networkenv.MachineSnapshot{}
	if ctx == nil {
		result.ObservationErrors = []string{"network observation context is required"}
		return result
	}
	platform, platformErr := observePlatformNetwork(ctx)
	result.Physical = platform.Physical
	result.TUN.External = append([]networkenv.ExternalAdapterRef(nil), platform.External...)
	result.TUN.ExternalPresent = len(result.TUN.External) > 0
	if platformErr != nil {
		result.ObservationErrors = append(result.ObservationErrors, platformErr.Error())
	}

	s.runtimeMu.Lock()
	runtimeTUNEnabled, name := s.runtime.TUNEnabled, s.runtime.TUNName
	s.runtimeMu.Unlock()
	if strings.TrimSpace(name) == "" {
		name = network.OwnedTUNAdapterName
	}
	s.tunRuntimeMu.RLock()
	stage, sessionID, committedAdapter := s.tunStage, s.tunSessionID, s.tunAdapter
	s.tunRuntimeMu.RUnlock()
	faultID, fault := s.currentTUNFault()
	adapter := tun.InspectAdapter(ctx, name)

	owned, ownership := false, networkenv.OwnerNone
	expected := runtimeTUNEnabled || sessionID != ""
	if expected {
		ownership = networkenv.OwnerNavo
		owned = adapterMatchesNetworkIdentity(adapter, committedAdapter.InterfaceGUID, int(committedAdapter.InterfaceIndex))
	}

	journalManager, journalManagerErr := s.newTUNRecoveryManager(name)
	ownedObservation := network.OwnedNetworkObservation{}
	if journalManagerErr != nil {
		result.ObservationErrors = append(result.ObservationErrors, "initialize read-only network observation: "+journalManagerErr.Error())
	} else {
		var observationErr error
		ownedObservation, observationErr = journalManager.Observe(ctx)
		if observationErr != nil {
			result.ObservationErrors = append(result.ObservationErrors, observationErr.Error())
		}
	}
	if !owned && ownedObservation.Journal.Present {
		owned = adapterMatchesNetworkIdentity(
			adapter, ownedObservation.Journal.AdapterGUID, ownedObservation.Journal.AdapterIndex,
		)
		if owned {
			ownership = networkenv.OwnerNavo
		}
	}
	if adapter.State != capture.AdapterMissing && !owned {
		ownership = networkenv.OwnerUnknown
	}

	result.TUN.Expected = expected
	result.TUN.Navo = networkenv.TUNAdapterSnapshot{
		Present: adapter.State != capture.AdapterMissing,
		Enabled: owned && adapter.State == capture.AdapterEnabled,
		Name:    adapter.Name, InterfaceGUID: adapter.InterfaceGUID,
		InterfaceIndex: adapter.InterfaceIndex, State: string(adapter.State),
		Ownership: ownership, SessionID: sessionID, Stage: string(stage),
		FaultID: faultID, LastError: firstNonEmpty(fault, adapter.Error),
	}
	result.Routes = networkResourceSnapshot(ownedObservation.Routes)
	result.NRPT = networkResourceSnapshot(ownedObservation.NRPT)
	result.DNS = result.NRPT
	result.Firewall = networkResourceSnapshot(ownedObservation.Firewall)
	result.Journal = networkJournalSnapshot(ownedObservation.Journal)
	result.ObservationErrors = append(result.ObservationErrors, ownedObservation.Errors...)
	if ctx.Err() != nil {
		result.ObservationErrors = appendUniqueString(result.ObservationErrors, ctx.Err().Error())
	}
	return result
}

func adapterMatchesNetworkIdentity(observed capture.AdapterStatus, guid string, index int) bool {
	if observed.State == capture.AdapterMissing || observed.InterfaceGUID == "" || observed.InterfaceIndex <= 0 {
		return false
	}
	if index > 0 && observed.InterfaceIndex != index {
		return false
	}
	return guid != "" && normalizeAdapterGUID(observed.InterfaceGUID) == normalizeAdapterGUID(guid)
}

func networkResourceSnapshot(value network.ResourceObservation) networkenv.ResourceSnapshot {
	return networkenv.ResourceSnapshot{
		Known: value.Known, Coherent: value.Coherent,
		OwnedCount: value.OwnedCount, ExistingCount: value.ExistingCount,
		MissingCount: value.MissingCount, ConflictCount: value.ConflictCount,
		LastError: value.LastError,
	}
}

func networkJournalSnapshot(value network.JournalObservation) networkenv.JournalSnapshot {
	return networkenv.JournalSnapshot{
		Present: value.Present, Dirty: value.Dirty, Version: value.Version,
		SessionID: value.SessionID, AdapterName: value.AdapterName,
		OwnedResources: value.OwnedResources, PreexistingResources: value.PreexistingResources,
		PendingActions: value.PendingActions, MissingResources: value.MissingResources,
		ConflictingResources: value.ConflictingResources, LastError: value.LastError,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
