//go:build windows

package network

import (
	"testing"
	"time"
)

func TestAdapterReadinessTrackerRequiresStableOwnedIdentity(t *testing.T) {
	base := time.Unix(100, 0)
	adapter := AdapterSnapshot{
		Name:                 OwnedTUNAdapterName,
		InterfaceDescription: ownedSingTUNDescription,
		InterfaceIndex:       60,
		InterfaceGUID:        "{NAVO-GUID}",
		InterfaceLUID:        600,
		OperationalStatus:    "Up",
		MTU:                  1500,
		IPv4Addresses:        []string{"172.19.0.1/30"},
	}
	var tracker adapterReadinessTracker
	if tracker.observe(adapter, "172.19.0.1/30", 1500, base) {
		t.Fatal("first ready observation satisfied the stability window")
	}
	if tracker.observe(adapter, "172.19.0.1/30", 1500, base.Add(adapterReadyStabilityWindow-time.Millisecond)) {
		t.Fatal("adapter became ready before the stability window elapsed")
	}
	if !tracker.observe(adapter, "172.19.0.1/30", 1500, base.Add(adapterReadyStabilityWindow)) {
		t.Fatal("stable adapter did not become ready")
	}
}

func TestAdapterReadinessTrackerResetsOnGapOrIdentityChange(t *testing.T) {
	base := time.Unix(200, 0)
	adapter := AdapterSnapshot{
		Name: OwnedTUNAdapterName, InterfaceDescription: ownedTUNDescription,
		InterfaceIndex: 27, InterfaceGUID: "{FIRST}", InterfaceLUID: 270,
		OperationalStatus: "Up", MTU: 1500, IPv4Addresses: []string{"172.19.0.1/30"},
	}
	var tracker adapterReadinessTracker
	tracker.observe(adapter, "172.19.0.1/30", 1500, base)
	downtime := adapter
	downtime.OperationalStatus = "Down"
	tracker.observe(downtime, "172.19.0.1/30", 1500, base.Add(time.Second))
	if tracker.observe(adapter, "172.19.0.1/30", 1500, base.Add(adapterReadyStabilityWindow)) {
		t.Fatal("readiness gap did not reset the stability window")
	}
	adapter.InterfaceGUID = "{SECOND}"
	adapter.InterfaceLUID++
	if tracker.observe(adapter, "172.19.0.1/30", 1500, base.Add(2*adapterReadyStabilityWindow)) {
		t.Fatal("identity change did not reset the stability window")
	}
}
