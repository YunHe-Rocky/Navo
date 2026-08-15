//go:build windows

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

type windowsEndpointResolver struct {
	executor Executor
}

// BuildTUNActivationPlan resolves and freezes the selected endpoint against
// the current physical Windows routing table.
func BuildTUNActivationPlan(ctx context.Context, request ActivationPlanRequest) (TUNActivationPlan, error) {
	return buildActivationPlan(ctx, request, windowsEndpointResolver{executor: NewSystemExecutor()})
}

// FindPhysicalRoute returns the current owned physical egress for one remote
// address without mutating routes or adapters.
func FindPhysicalRoute(ctx context.Context, endpointIP, excludedAdapter string) (EndpointRoutePlan, error) {
	return (windowsEndpointResolver{executor: NewSystemExecutor()}).FindPhysicalRoute(ctx, endpointIP, excludedAdapter)
}

func (windowsEndpointResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return ResolveEndpointIPs(ctx, host)
}

func (r windowsEndpointResolver) FindPhysicalRoute(
	ctx context.Context,
	endpointIP, excludedAdapter string,
) (EndpointRoutePlan, error) {
	script := "$routes=@(Find-NetRoute -RemoteIPAddress " + psQuote(endpointIP) + " -ErrorAction Stop);" +
		"$items=@();foreach($route in $routes){" +
		"$adapter=Get-NetAdapter -InterfaceIndex ([uint32]$route.InterfaceIndex) -ErrorAction SilentlyContinue;" +
		"if($null -eq $adapter -or [string]$adapter.Status -ne 'Up' -or [string]$adapter.Name -eq " + psQuote(excludedAdapter) + "){continue};" +
		"if(-not ($route.PSObject.Properties.Name -contains 'NextHop')){continue};" +
		"if([string]::IsNullOrWhiteSpace([string]$route.NextHop)){continue};" +
		"$items += [pscustomobject]@{interface_index=[uint32]$route.InterfaceIndex;interface_guid=[string]$adapter.InterfaceGuid;interface_alias=[string]$adapter.Name;next_hop=[string]$route.NextHop;route_metric=[int]$route.RouteMetric;interface_metric=[int]$route.InterfaceMetric}};" +
		"$selected=$items|Sort-Object @{Expression={$_.route_metric+$_.interface_metric}},interface_index|Select-Object -First 1;" +
		"if($null -eq $selected){throw 'no up physical route'};$selected|ConvertTo-Json -Compress"
	output, err := r.executor.RunOutput(ctx, powershell(script))
	if err != nil {
		return EndpointRoutePlan{}, err
	}
	var route EndpointRoutePlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &route); err != nil {
		return EndpointRoutePlan{}, fmt.Errorf("decode physical route: %w", err)
	}
	if route.InterfaceIndex == 0 || route.InterfaceGUID == "" || route.NextHop == "" {
		return EndpointRoutePlan{}, fmt.Errorf("physical route is incomplete")
	}
	return route, nil
}
