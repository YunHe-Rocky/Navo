package connection

import "strings"

// Intent describes why a connection mutation is running. It is deliberately
// separate from Operation: the same capture switch has different admission
// urgency when it comes from a user click, login startup, or background repair.
type Intent string

const (
	IntentSafety      Intent = "safety"
	IntentInteractive Intent = "interactive"
	IntentRecovery    Intent = "recovery"
	IntentStartup     Intent = "startup"
	IntentBackground  Intent = "background"
)

// Domain is the user-facing control area affected by a mutation. Domains are
// used for contextual supersession; they do not create independent physical
// network writers.
type Domain string

const (
	DomainCapture Domain = "capture"
	DomainRouting Domain = "routing"
	DomainCore    Domain = "core"
	DomainSource  Domain = "source"
)

// Policy is resolved centrally for every Coordinator request. Rank is only an
// admission class; Domain prevents a route click from discarding unrelated core
// or source work.
type Policy struct {
	Intent Intent
	Domain Domain
	Rank   int
}

func PolicyFor(request Request) Policy {
	intent := intentFor(request)
	return Policy{Intent: intent, Domain: domainFor(request), Rank: intentRank(intent)}
}

func intentFor(request Request) Intent {
	switch request.Origin {
	case OriginShutdown:
		return IntentSafety
	case OriginUser, OriginTray:
		return IntentInteractive
	case OriginSelfHeal:
		return IntentRecovery
	case OriginStartup:
		return IntentStartup
	case OriginScheduler:
		return IntentBackground
	}
	if request.Operation == OperationRecovery || request.Operation == OperationSelfHeal {
		return IntentRecovery
	}
	return IntentBackground
}

func intentRank(intent Intent) int {
	switch intent {
	case IntentSafety:
		return 500
	case IntentInteractive:
		return 400
	case IntentRecovery:
		return 300
	case IntentStartup:
		return 200
	default:
		return 100
	}
}

func domainFor(request Request) Domain {
	switch request.Operation {
	case OperationCaptureSwitch:
		return DomainCapture
	case OperationNodeSwitch, OperationPolicyChange:
		return DomainRouting
	case OperationCoreSwitch, OperationCoreUpdate:
		return DomainCore
	case OperationSourceMutation:
		return DomainSource
	case OperationRecovery, OperationSelfHeal:
		domain := strings.ToLower(strings.TrimSpace(request.FaultDomain))
		switch {
		case strings.Contains(domain, "core"):
			return DomainCore
		case strings.Contains(domain, "node"), strings.Contains(domain, "outbound"), strings.Contains(domain, "policy"), strings.Contains(domain, "runtime"):
			return DomainRouting
		case strings.Contains(domain, "source"), strings.Contains(domain, "subscription"):
			return DomainSource
		default:
			return DomainCapture
		}
	default:
		return DomainCapture
	}
}
