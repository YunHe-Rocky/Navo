package networkenv

import (
	"testing"
	"time"
)

func healthyBase() Snapshot {
	return Snapshot{
		Version: SnapshotVersion, CollectedAt: time.Now().UTC(),
		Physical:    PhysicalSnapshot{Known: true, Available: true, ActiveInterfaces: []string{"Ethernet"}},
		Capture:     CaptureSnapshot{State: "stopped", DesiredMode: "off", CommittedMode: "off"},
		SystemProxy: SystemProxySnapshot{Ownership: OwnerNone},
		TUN:         TUNSnapshot{Navo: TUNAdapterSnapshot{Ownership: OwnerNone}},
	}
}

func TestAnalyzeDocumentCases(t *testing.T) {
	tests := []struct {
		name         string
		mutate       func(*Snapshot)
		code         string
		severity     Severity
		ownership    Ownership
		recoverable  bool
		health       HealthState
		transitional bool
	}{
		{
			name: "external system proxy while Navo is off",
			mutate: func(snapshot *Snapshot) {
				snapshot.SystemProxy = SystemProxySnapshot{Enabled: true, ProxyServer: "127.0.0.1:10808", Ownership: OwnerExternal}
			},
			code: FindingSystemProxyExternal, severity: SeverityInfo,
			ownership: OwnerExternal, health: HealthHealthy,
		},
		{
			name: "owned system proxy is healthy",
			mutate: func(snapshot *Snapshot) {
				snapshot.Capture = CaptureSnapshot{State: "running_system_proxy", DesiredMode: "system_proxy", CommittedMode: "system_proxy", ReadinessState: "ready"}
				snapshot.SystemProxy = SystemProxySnapshot{Enabled: true, ProxyServer: "127.0.0.1:12080", Ownership: OwnerNavo, OwnedByNavo: true, OwnershipMarker: true, Reachable: true, ReachableKnown: true}
			},
			health: HealthHealthy,
		},
		{
			name: "external takeover preserves new proxy",
			mutate: func(snapshot *Snapshot) {
				snapshot.Capture = CaptureSnapshot{State: "running_system_proxy", DesiredMode: "system_proxy", CommittedMode: "system_proxy"}
				snapshot.SystemProxy = SystemProxySnapshot{Enabled: true, ProxyServer: "127.0.0.1:10808", Ownership: OwnerExternal, OwnershipMarker: true, OwnershipLost: true}
			},
			code: FindingSystemProxyStaleNavo, severity: SeverityError,
			ownership: OwnerExternal, health: HealthUnavailable,
		},
		{
			name: "expected Navo TUN is missing",
			mutate: func(snapshot *Snapshot) {
				snapshot.Capture = CaptureSnapshot{State: "running_tun", DesiredMode: "tun", CommittedMode: "tun"}
				snapshot.TUN = TUNSnapshot{Expected: true, Navo: TUNAdapterSnapshot{Ownership: OwnerNavo}}
			},
			code: FindingNavoTUNMissing, severity: SeverityError,
			ownership: OwnerNavo, recoverable: true, health: HealthUnavailable,
		},
		{
			name: "external TUN is informational",
			mutate: func(snapshot *Snapshot) {
				snapshot.TUN = TUNSnapshot{ExternalPresent: true, External: []ExternalAdapterRef{{Name: "External VPN", InterfaceIndex: 9}}}
			},
			code: FindingExternalTUNPresent, severity: SeverityInfo,
			ownership: OwnerExternal, health: HealthHealthy,
		},
		{
			name: "physical network is unavailable and not repairable",
			mutate: func(snapshot *Snapshot) {
				snapshot.Physical = PhysicalSnapshot{Known: true, Available: false}
			},
			code: FindingPhysicalNetworkUnavailable, severity: SeverityError,
			ownership: OwnerNone, health: HealthUnavailable,
		},
		{
			name: "coordinator applying suppresses premature repair",
			mutate: func(snapshot *Snapshot) {
				snapshot.Transition = TransitionSnapshot{Busy: true, Operation: "capture_switch", Phase: "applying"}
				snapshot.Routes = ResourceSnapshot{Known: true, Coherent: false, OwnedCount: 2, MissingCount: 1}
			},
			code: FindingNavoRouteResidual, severity: SeverityError,
			ownership: OwnerNavo, health: HealthChecking, transitional: true,
		},
		{
			name: "owned pending journal is repairable",
			mutate: func(snapshot *Snapshot) {
				snapshot.Journal = JournalSnapshot{Present: true, Dirty: true, OwnedResources: 3, PendingActions: 1}
			},
			code: FindingNetworkJournalPending, severity: SeverityError,
			ownership: OwnerNavo, recoverable: true, health: HealthDegraded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyBase()
			test.mutate(&snapshot)
			result := Analyze(snapshot)
			if result.Health != test.health {
				t.Fatalf("health = %q, want %q; findings=%+v", result.Health, test.health, result.Findings)
			}
			if test.code == "" {
				if len(result.Findings) != 0 {
					t.Fatalf("healthy snapshot findings = %+v", result.Findings)
				}
				return
			}
			finding, ok := findByCode(result.Findings, test.code)
			if !ok {
				t.Fatalf("finding %s missing: %+v", test.code, result.Findings)
			}
			if finding.Severity != test.severity || finding.Ownership != test.ownership ||
				finding.Recoverable != test.recoverable || finding.Transitional != test.transitional {
				t.Fatalf("finding = %+v", finding)
			}
		})
	}
}

func TestAnalyzePartialSnapshotDoesNotDiscardUsefulFacts(t *testing.T) {
	snapshot := healthyBase()
	snapshot.Partial = true
	snapshot.ObservationErrors = []string{"route observer timed out"}
	snapshot.SystemProxy = SystemProxySnapshot{Enabled: true, ProxyServer: "127.0.0.1:10808", Ownership: OwnerExternal}

	result := Analyze(snapshot)
	if result.Health != HealthDegraded {
		t.Fatalf("health = %q", result.Health)
	}
	if _, ok := findByCode(result.Findings, FindingObservationPartial); !ok {
		t.Fatal("partial finding missing")
	}
	if _, ok := findByCode(result.Findings, FindingSystemProxyExternal); !ok {
		t.Fatal("external proxy fact was discarded")
	}
}

func findByCode(findings []Finding, code string) (Finding, bool) {
	for _, finding := range findings {
		if finding.Code == code {
			return finding, true
		}
	}
	return Finding{}, false
}
