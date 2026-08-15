package network

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeEndpointResolver struct {
	addresses []net.IP
	lookupErr error
	routes    map[string]EndpointRoutePlan
	routeErr  map[string]error
}

func (f fakeEndpointResolver) LookupIP(context.Context, string) ([]net.IP, error) {
	return f.addresses, f.lookupErr
}
func (f fakeEndpointResolver) FindPhysicalRoute(_ context.Context, endpointIP, _ string) (EndpointRoutePlan, error) {
	if err := f.routeErr[endpointIP]; err != nil {
		return EndpointRoutePlan{}, err
	}
	value, ok := f.routes[endpointIP]
	if !ok {
		return EndpointRoutePlan{}, errors.New("missing route")
	}
	return value, nil
}

func planRequest(host string) ActivationPlanRequest {
	return ActivationPlanRequest{SessionID: "plan-test", CoreID: "sing-box", AdapterName: "Navo", TUNIPv4Address: "172.19.0.1/30", TUNIPv4Peer: "172.19.0.2", TUNDNSIPv4: "172.19.0.2", MTU: 1500, SelectedOutboundID: "node", OriginalServerHost: host, IPv6Mode: IPv6Block}
}

func TestBuildActivationPlanRejectsUnownedAdapterBeforeRouteLookup(t *testing.T) {
	request := planRequest("proxy.example")
	request.AdapterName = "以太网"

	_, err := buildActivationPlan(context.Background(), request, fakeEndpointResolver{})
	if err == nil {
		t.Fatal("physical adapter name was accepted as the TUN adapter")
	}
	var tunErr *TUNError
	if !errors.As(err, &tunErr) || tunErr.Code != ErrTUNAdapterConflict {
		t.Fatalf("unexpected error: %v", err)
	}
}

func physicalRoute(index uint32, routeMetric, interfaceMetric int) EndpointRoutePlan {
	return EndpointRoutePlan{InterfaceIndex: index, InterfaceGUID: "{PHYSICAL}", InterfaceAlias: "Ethernet", NextHop: "192.0.2.1", RouteMetric: routeMetric, InterfaceMetric: interfaceMetric}
}

func TestBuildActivationPlanUsesLiteralIPWithoutDNS(t *testing.T) {
	resolver := fakeEndpointResolver{lookupErr: errors.New("DNS must not be called"), routes: map[string]EndpointRoutePlan{"203.0.113.7": physicalRoute(8, 10, 20)}}
	plan, err := buildActivationPlan(context.Background(), planRequest("203.0.113.7"), resolver)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PinnedServerIP != "203.0.113.7" || plan.EndpointRoutes[0].InterfaceIndex != 8 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestBuildActivationPlanSelectsLowestCombinedMetricAcrossMultipleARecords(t *testing.T) {
	resolver := fakeEndpointResolver{
		addresses: []net.IP{net.ParseIP("203.0.113.8"), net.ParseIP("203.0.113.7"), net.ParseIP("127.0.0.1"), net.ParseIP("2001:db8::7")},
		routes: map[string]EndpointRoutePlan{
			"203.0.113.8": physicalRoute(9, 5, 50),
			"203.0.113.7": physicalRoute(8, 10, 10),
		},
	}
	plan, err := buildActivationPlan(context.Background(), planRequest("proxy.example"), resolver)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PinnedServerIP != "203.0.113.7" || len(plan.EndpointRoutes) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestBuildActivationPlanFailsBeforeMutationWhenResolutionOrRouteFails(t *testing.T) {
	tests := []fakeEndpointResolver{
		{lookupErr: errors.New("DNS unavailable")},
		{addresses: []net.IP{net.ParseIP("203.0.113.7")}, routeErr: map[string]error{"203.0.113.7": errors.New("no physical route")}},
	}
	for _, resolver := range tests {
		if _, err := buildActivationPlan(context.Background(), planRequest("proxy.example"), resolver); err == nil {
			t.Fatal("invalid activation plan unexpectedly succeeded")
		}
	}
}

func TestBuildActivationPlanAllowsDirectModeWithoutEndpoint(t *testing.T) {
	request := planRequest("")
	resolver := fakeEndpointResolver{routes: map[string]EndpointRoutePlan{"1.1.1.1": physicalRoute(7, 10, 15)}}
	plan, err := buildActivationPlan(context.Background(), request, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PinnedServerIP != "" || len(plan.EndpointRoutes) != 0 || plan.PhysicalRoute.InterfaceIndex != 7 {
		t.Fatalf("direct plan = %#v", plan)
	}
}

func TestBuildActivationPlanAllowsIPv4LoopbackUpstreamWithoutBypass(t *testing.T) {
	request := planRequest("127.0.0.1")
	resolver := fakeEndpointResolver{routes: map[string]EndpointRoutePlan{"1.1.1.1": physicalRoute(7, 10, 15)}}
	plan, err := buildActivationPlan(context.Background(), request, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PinnedServerIP != "127.0.0.1" || len(plan.EndpointRoutes) != 0 || plan.PhysicalRoute.InterfaceIndex != 7 {
		t.Fatalf("loopback plan = %#v", plan)
	}
}

func TestBuildActivationPlanRejectsIPv6LoopbackWhenIPv6IsBlocked(t *testing.T) {
	request := planRequest("::1")
	resolver := fakeEndpointResolver{routes: map[string]EndpointRoutePlan{"1.1.1.1": physicalRoute(7, 10, 15)}}
	if _, err := buildActivationPlan(context.Background(), request, resolver); err == nil {
		t.Fatal("IPv6 loopback unexpectedly bypassed IPv6 block mode")
	}
}
