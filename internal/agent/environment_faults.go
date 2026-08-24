package agent

import (
	"context"
	"errors"
	"time"

	"navo/internal/domain/capture"
	"navo/internal/networkenv"
	"navo/internal/selfheal"
)

func (a *Agent) environmentCaptureFault(
	mode capture.Mode,
) (*attributedCaptureFault, bool) {
	snapshot := a.environmentStore.Load()
	if snapshot.Stale || snapshot.CollectedAt.IsZero() {
		return nil, false
	}
	for _, finding := range snapshot.Findings {
		if finding.Severity != networkenv.SeverityError || finding.Transitional ||
			!findingAppliesToMode(finding, mode) {
			continue
		}
		return attributedFaultFromFinding(mode, finding), true
	}
	return nil, !snapshot.Partial
}

func findingAppliesToMode(finding networkenv.Finding, mode capture.Mode) bool {
	switch finding.Code {
	case networkenv.FindingPhysicalNetworkUnavailable,
		networkenv.FindingCaptureDataPlaneFailed:
		return mode != capture.ModeOff
	case networkenv.FindingSystemProxyStaleNavo,
		networkenv.FindingSystemProxyEndpointUnreachable:
		return mode == capture.ModeSystemProxy
	case networkenv.FindingNavoTUNMissing,
		networkenv.FindingNavoTUNDisabled,
		networkenv.FindingNavoRouteResidual,
		networkenv.FindingNavoDNSInconsistent,
		networkenv.FindingNavoNRPTInconsistent,
		networkenv.FindingNavoFirewallInconsistent,
		networkenv.FindingNetworkJournalPending:
		return mode == capture.ModeTUN
	default:
		return false
	}
}

func attributedFaultFromFinding(
	mode capture.Mode,
	finding networkenv.Finding,
) *attributedCaptureFault {
	domain, code := selfHealIdentityForFinding(finding)
	severity := selfheal.SeverityError
	if finding.Severity == networkenv.SeverityWarn {
		severity = selfheal.SeverityWarn
	}
	message := finding.Summary
	if finding.Detail != "" {
		message += ": " + finding.Detail
	}
	cause := errors.New(message)
	plan := selfheal.PlanFor(domain)
	return &attributedCaptureFault{
		cause: cause,
		evidence: selfheal.FaultEvidence{
			Code: code, Domain: domain, Severity: severity,
			Summary: finding.Summary, Symptom: message, Impact: plan.Impact,
			SourceService: "NetworkEnvironment", CaptureMode: mode.String(),
			ObservedAt: time.Now().UTC(),
			Details: map[string]any{
				"environment_finding_code": finding.Code,
				"ownership":                string(finding.Ownership),
				"recoverable":              finding.Recoverable,
				"transitional":             finding.Transitional,
			},
		},
	}
}

func selfHealIdentityForFinding(
	finding networkenv.Finding,
) (selfheal.FaultDomain, selfheal.ErrorCode) {
	switch finding.Code {
	case networkenv.FindingSystemProxyStaleNavo,
		networkenv.FindingSystemProxyEndpointUnreachable:
		return selfheal.FaultDomainSystemProxy, selfheal.CodeSystemProxyMismatch
	case networkenv.FindingNavoTUNMissing:
		return selfheal.FaultDomainTUN, selfheal.CodeTUNAdapterMissing
	case networkenv.FindingNavoTUNDisabled:
		return selfheal.FaultDomainTUN, selfheal.CodeTUNAdapterDisabled
	case networkenv.FindingNavoRouteResidual,
		networkenv.FindingNetworkJournalPending:
		return selfheal.FaultDomainRoute, selfheal.CodeRouteBypassMissing
	case networkenv.FindingNavoDNSInconsistent:
		return selfheal.FaultDomainDNS, selfheal.CodeDNSMismatch
	case networkenv.FindingNavoNRPTInconsistent:
		return selfheal.FaultDomainNRPT, selfheal.CodeNRPTMismatch
	case networkenv.FindingNavoFirewallInconsistent:
		return selfheal.FaultDomainFirewall, selfheal.CodeFirewallMismatch
	case networkenv.FindingPhysicalNetworkUnavailable:
		return selfheal.FaultDomainPhysicalNetwork, selfheal.CodePhysicalNetworkDown
	case networkenv.FindingCaptureDataPlaneFailed:
		return selfheal.FaultDomainNode, selfheal.CodeCaptureDataPlaneFailed
	case networkenv.FindingObservationPartial:
		return selfheal.FaultDomainDetection, selfheal.CodeDetectionFailed
	default:
		return selfheal.FaultDomainUnknown, selfheal.CodeConnectivityUnknown
	}
}

func environmentFindingCode(fault *attributedCaptureFault) string {
	if fault == nil || fault.evidence.Details == nil {
		return ""
	}
	code, _ := fault.evidence.Details["environment_finding_code"].(string)
	return code
}

func (a *Agent) verifyEnvironmentFindingCleared(
	ctx context.Context,
	code string,
) (capture.ReadinessEvidence, error) {
	snapshot := a.refreshEnvironment(ctx)
	for _, finding := range snapshot.Findings {
		if finding.Code == code {
			return capture.ReadinessEvidence{
				State: "failed", Scope: "network_environment",
				CheckedAt: time.Now().UTC(), Error: finding.Summary,
			}, errors.New(finding.Summary)
		}
	}
	return capture.ReadinessEvidence{
		State: "ready", Scope: "network_environment", CheckedAt: time.Now().UTC(),
	}, nil
}
