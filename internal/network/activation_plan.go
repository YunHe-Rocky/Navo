package network

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ActivationPlanRequest contains only non-secret facts required to freeze one
// TUN activation before the proxy core or virtual adapter starts.
type ActivationPlanRequest struct {
	SessionID          string
	CoreID             string
	AdapterName        string
	TUNIPv4Address     string
	TUNIPv4Peer        string
	TUNDNSIPv4         string
	MTU                int
	SelectedOutboundID string
	OriginalServerHost string
	IPv6Mode           IPv6Mode
}

type endpointResolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
	FindPhysicalRoute(ctx context.Context, endpointIP, excludedAdapter string) (EndpointRoutePlan, error)
}

// NewTUNSessionID returns a process-independent identifier safe for journal
// tags, NRPT comments, firewall names, and route metrics.
func NewTUNSessionID() string {
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
}

func buildActivationPlan(
	ctx context.Context,
	request ActivationPlanRequest,
	resolver endpointResolver,
) (TUNActivationPlan, error) {
	if request.SessionID == "" {
		request.SessionID = NewTUNSessionID()
	}
	if request.AdapterName == "" {
		request.AdapterName = OwnedTUNAdapterName
	}
	if !strings.EqualFold(strings.TrimSpace(request.AdapterName), OwnedTUNAdapterName) {
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNAdapterConflict, Stage: TUNStagePreflight, Resource: request.AdapterName, Expected: OwnedTUNAdapterName, Actual: "adapter is outside Navo ownership"}
	}
	request.AdapterName = OwnedTUNAdapterName
	if request.TUNIPv4Address == "" {
		request.TUNIPv4Address = "172.19.0.1/30"
	}
	if request.TUNIPv4Peer == "" {
		request.TUNIPv4Peer = "172.19.0.2"
	}
	if request.TUNDNSIPv4 == "" {
		request.TUNDNSIPv4 = request.TUNIPv4Peer
	}
	if request.MTU <= 0 {
		request.MTU = 1500
	}
	if request.IPv6Mode == "" {
		request.IPv6Mode = IPv6Block
	}
	plan := TUNActivationPlan{
		SessionID: request.SessionID, CoreID: request.CoreID,
		AdapterName: request.AdapterName, TUNIPv4Address: request.TUNIPv4Address,
		TUNIPv4Peer: request.TUNIPv4Peer, TUNDNSIPv4: request.TUNDNSIPv4,
		MTU: request.MTU, SelectedOutboundID: request.SelectedOutboundID,
		OriginalServerHost: strings.TrimSpace(request.OriginalServerHost),
		IPv6Mode:           request.IPv6Mode, CreatedAt: time.Now().UTC(),
	}
	if resolver == nil {
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNPhysicalRouteNotFound, Stage: TUNStagePreflight, Cause: fmt.Errorf("physical route resolver is unavailable")}
	}
	if plan.OriginalServerHost == "" { // Direct mode still freezes the core's physical egress.
		return freezePhysicalEgress(ctx, plan, resolver)
	}

	addresses := []net.IP(nil)
	if parsed := net.ParseIP(plan.OriginalServerHost); parsed != nil {
		if parsed.To4() != nil && parsed.IsLoopback() {
			plan.PinnedServerIP = parsed.String()
			return freezePhysicalEgress(ctx, plan, resolver)
		}
		addresses = []net.IP{parsed}
	} else {
		resolved, err := resolver.LookupIP(ctx, plan.OriginalServerHost)
		if err != nil {
			return TUNActivationPlan{}, &TUNError{Code: ErrTUNEndpointResolveFailed, Stage: TUNStagePreflight, Resource: plan.OriginalServerHost, Cause: err}
		}
		addresses = resolved
	}

	type candidate struct {
		ip    string
		route EndpointRoutePlan
	}
	var candidates []candidate
	var routeErrors []string
	seen := make(map[string]struct{})
	for _, address := range addresses {
		if !usableEndpointIP(address, plan.IPv6Mode) {
			continue
		}
		canonical := address.String()
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		route, err := resolver.FindPhysicalRoute(ctx, canonical, plan.AdapterName)
		if err != nil {
			routeErrors = append(routeErrors, canonical+": "+err.Error())
			continue
		}
		route.EndpointHost = plan.OriginalServerHost
		route.EndpointIP = canonical
		if address.To4() != nil {
			route.AddressFamily = "IPv4"
		} else {
			route.AddressFamily = "IPv6"
		}
		candidates = append(candidates, candidate{ip: canonical, route: route})
	}
	if len(candidates) == 0 {
		cause := fmt.Errorf("no routable address resolved")
		if len(routeErrors) > 0 {
			cause = fmt.Errorf("no physical route: %s", strings.Join(routeErrors, "; "))
		}
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNPhysicalRouteNotFound, Stage: TUNStagePreflight, Resource: plan.OriginalServerHost, Cause: cause}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i].route.RouteMetric + candidates[i].route.InterfaceMetric
		right := candidates[j].route.RouteMetric + candidates[j].route.InterfaceMetric
		if left != right {
			return left < right
		}
		return candidates[i].ip < candidates[j].ip
	})
	plan.PinnedServerIP = candidates[0].ip
	plan.PhysicalRoute = candidates[0].route
	plan.EndpointRoutes = []EndpointRoutePlan{candidates[0].route}
	if err := plan.validate(); err != nil {
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNEndpointPinFailed, Stage: TUNStagePreflight, Resource: plan.OriginalServerHost, Cause: err}
	}
	return plan, nil
}

func freezePhysicalEgress(
	ctx context.Context,
	plan TUNActivationPlan,
	resolver endpointResolver,
) (TUNActivationPlan, error) {
	physical, err := resolver.FindPhysicalRoute(ctx, "1.1.1.1", plan.AdapterName)
	if err != nil {
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNPhysicalRouteNotFound, Stage: TUNStagePreflight, Resource: "1.1.1.1", Cause: err}
	}
	physical.EndpointHost, physical.EndpointIP, physical.AddressFamily = "physical-egress-probe", "1.1.1.1", "IPv4"
	plan.PhysicalRoute = physical
	if err := plan.validate(); err != nil {
		return TUNActivationPlan{}, &TUNError{Code: ErrTUNPreflightFailed, Stage: TUNStagePreflight, Cause: err}
	}
	return plan, nil
}

func usableEndpointIP(ip net.IP, mode IPv6Mode) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if mode == IPv6Block && ip.To4() == nil {
		return false
	}
	return true
}
