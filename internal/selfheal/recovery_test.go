package selfheal

import "testing"

func TestFaultMatrixCoversEveryV1DomainWithBoundedActions(t *testing.T) {
	want := []FaultDomain{
		FaultDomainNode, FaultDomainCore, FaultDomainSystemProxy, FaultDomainTUN,
		FaultDomainRoute, FaultDomainDNS, FaultDomainNRPT, FaultDomainFirewall,
		FaultDomainTrafficRule, FaultDomainPhysicalNetwork, FaultDomainDetection,
		FaultDomainUnknown,
	}
	plans := FaultMatrix()
	if len(plans) != len(want) {
		t.Fatalf("fault plans = %d, want %d", len(plans), len(want))
	}
	seen := make(map[FaultDomain]bool, len(plans))
	for _, plan := range plans {
		if seen[plan.Domain] {
			t.Fatalf("duplicate fault plan for %s", plan.Domain)
		}
		seen[plan.Domain] = true
		if len(plan.Evidence) == 0 || plan.Impact == "" {
			t.Fatalf("fault plan lacks evidence or impact: %#v", plan)
		}
		for round := 1; round <= MaxRepairRounds; round++ {
			action := plan.Action(round)
			if plan.Controllable && action == ActionNone {
				t.Fatalf("controllable domain %s has no round %d action", plan.Domain, round)
			}
			if !plan.Controllable && action != ActionNone {
				t.Fatalf("read-only domain %s mutates with %s", plan.Domain, action)
			}
		}
		if plan.Action(0) != ActionNone || plan.Action(MaxRepairRounds+1) != ActionNone {
			t.Fatalf("domain %s accepts an out-of-range repair round", plan.Domain)
		}
		if plan.AllowFailover != (plan.Domain == FaultDomainNode) {
			t.Fatalf("domain %s failover=%t", plan.Domain, plan.AllowFailover)
		}
	}
	for _, domain := range want {
		if !seen[domain] {
			t.Fatalf("missing fault plan for %s", domain)
		}
	}
}

func TestFaultMatrixReturnsDefensiveEvidenceCopies(t *testing.T) {
	plans := FaultMatrix()
	plans[0].Evidence[0] = "mutated"
	if PlanFor(FaultDomainNode).Evidence[0] == "mutated" {
		t.Fatal("FaultMatrix exposed mutable package evidence")
	}
	unknown := PlanFor(FaultDomain("not_registered"))
	if unknown.Domain != FaultDomainUnknown || unknown.Controllable {
		t.Fatalf("unknown plan = %#v", unknown)
	}
}
