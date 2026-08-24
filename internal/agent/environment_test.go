package agent

import (
	"testing"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
	"navo/internal/networkenv"
	"navo/internal/selfheal"
)

func TestSystemProxyEnvironmentSeparatesRawStateFromOwnership(t *testing.T) {
	raw := systemproxy.ProxyConfig{
		Enabled: true, ProxyServer: "127.0.0.1:12080",
		AutoDetect: true, AutoConfigURL: "https://example.invalid/pac",
	}
	external := systemProxyEnvironment(raw, systemproxy.OwnershipStatus{}, 12080)
	if external.Ownership != networkenv.OwnerExternal || external.OwnedByNavo {
		t.Fatalf("external proxy = %#v", external)
	}
	if !external.AutoDetect || !external.AutoConfigConfigured || !external.LocalEndpoint {
		t.Fatalf("raw WinINet facts were lost: %#v", external)
	}

	owned := systemProxyEnvironment(raw, systemproxy.OwnershipStatus{
		Present: true, Owned: true, ProxyServer: raw.ProxyServer,
	}, 12080)
	if owned.Ownership != networkenv.OwnerNavo || !owned.OwnedByNavo || !owned.OwnershipMarker {
		t.Fatalf("owned proxy = %#v", owned)
	}
}

func TestEnvironmentFindingMapsToStableSelfHealIdentity(t *testing.T) {
	tests := []struct {
		finding string
		domain  selfheal.FaultDomain
		code    selfheal.ErrorCode
	}{
		{networkenv.FindingSystemProxyStaleNavo, selfheal.FaultDomainSystemProxy, selfheal.CodeSystemProxyMismatch},
		{networkenv.FindingNavoTUNMissing, selfheal.FaultDomainTUN, selfheal.CodeTUNAdapterMissing},
		{networkenv.FindingNavoTUNDisabled, selfheal.FaultDomainTUN, selfheal.CodeTUNAdapterDisabled},
		{networkenv.FindingNavoRouteResidual, selfheal.FaultDomainRoute, selfheal.CodeRouteBypassMissing},
		{networkenv.FindingNavoDNSInconsistent, selfheal.FaultDomainDNS, selfheal.CodeDNSMismatch},
		{networkenv.FindingNavoNRPTInconsistent, selfheal.FaultDomainNRPT, selfheal.CodeNRPTMismatch},
		{networkenv.FindingNavoFirewallInconsistent, selfheal.FaultDomainFirewall, selfheal.CodeFirewallMismatch},
		{networkenv.FindingPhysicalNetworkUnavailable, selfheal.FaultDomainPhysicalNetwork, selfheal.CodePhysicalNetworkDown},
	}
	for _, test := range tests {
		domain, code := selfHealIdentityForFinding(networkenv.Finding{Code: test.finding})
		if domain != test.domain || code != test.code {
			t.Fatalf("%s maps to %s/%s, want %s/%s", test.finding, domain, code, test.domain, test.code)
		}
	}
}

func TestEnvironmentCaptureFaultUsesFreshCompleteEvidence(t *testing.T) {
	instance := &Agent{environmentStore: networkenv.NewStore()}
	instance.environmentStore.Publish(networkenv.Snapshot{
		CollectedAt: time.Now().UTC(),
		Findings: []networkenv.Finding{{
			Code:     networkenv.FindingNavoTUNMissing,
			Severity: networkenv.SeverityError, Domain: "tun",
			Summary: "missing", Ownership: networkenv.OwnerNavo, Recoverable: true,
		}},
	})
	fault, trusted := instance.environmentCaptureFault(capture.ModeTUN)
	if fault == nil || !trusted || fault.evidence.Code != selfheal.CodeTUNAdapterMissing {
		t.Fatalf("fresh environment fault = %#v trusted=%v", fault, trusted)
	}

	instance.environmentStore.Publish(networkenv.Snapshot{
		CollectedAt: time.Now().UTC(), Partial: true, Findings: []networkenv.Finding{},
	})
	if fault, trusted := instance.environmentCaptureFault(capture.ModeTUN); fault != nil || trusted {
		t.Fatalf("partial evidence fault=%#v trusted=%v", fault, trusted)
	}

	instance.environmentStore.Publish(networkenv.Snapshot{
		CollectedAt: time.Now().UTC(),
		Findings: []networkenv.Finding{{
			Code:     networkenv.FindingNavoTUNMissing,
			Severity: networkenv.SeverityError, Domain: "tun",
			Summary: "transitioning", Ownership: networkenv.OwnerNavo,
			Transitional: true,
		}},
	})
	if fault, trusted := instance.environmentCaptureFault(capture.ModeTUN); fault != nil || !trusted {
		t.Fatalf("transitional evidence fault=%#v trusted=%v", fault, trusted)
	}

	instance.environmentStore.Publish(networkenv.Snapshot{
		CollectedAt: time.Now().UTC().Add(-networkenv.SnapshotStaleAfter - time.Second),
		Findings:    []networkenv.Finding{},
	})
	if fault, trusted := instance.environmentCaptureFault(capture.ModeTUN); fault != nil || trusted {
		t.Fatalf("stale evidence fault=%#v trusted=%v", fault, trusted)
	}
}

func TestExternalFindingsNeverEnterRepairableCapturePath(t *testing.T) {
	for _, finding := range []networkenv.Finding{
		{Code: networkenv.FindingSystemProxyExternal, Severity: networkenv.SeverityInfo, Ownership: networkenv.OwnerExternal},
		{Code: networkenv.FindingExternalTUNPresent, Severity: networkenv.SeverityInfo, Ownership: networkenv.OwnerExternal},
	} {
		if findingAppliesToMode(finding, capture.ModeSystemProxy) ||
			findingAppliesToMode(finding, capture.ModeTUN) {
			t.Fatalf("external finding entered capture repair path: %#v", finding)
		}
	}
}
