//go:build windows

package tun

import (
	"context"
	"fmt"
	"strings"
)

type routeManager struct{}

// NewRouteManager creates a Windows route table manager using netsh.
func NewRouteManager() RouteManager {
	return &routeManager{}
}

func (r *routeManager) AddRoutes(ctx context.Context, adapterName string, routes []Route) error {
	for _, route := range routes {
		if route.Destination == "" {
			continue
		}

		// Determine IPv4 or IPv6
		netshCmd := "interface"
		if strings.Contains(route.Destination, ":") {
			netshCmd = "interface"
		}

		args := []string{netshCmd, "ipv4", "add", "route", route.Destination}
		if route.Gateway != "" && route.Gateway != "0.0.0.0" {
			args = append(args, route.Gateway)
		}
		if route.InterfaceName != "" {
			args = append(args, route.InterfaceName)
		}
		if route.Metric > 0 {
			args = append(args, fmt.Sprintf("metric=%d", route.Metric))
		}

		cmd := hiddenCommandContext(ctx, "netsh", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: netsh add route failed for %s: %v (output: %s)",
				ErrNet004, route.Destination, err, string(out))
		}
	}
	return nil
}

func (r *routeManager) RemoveRoutes(ctx context.Context, adapterName string) error {
	// Remove all routes on the TUN interface by listing and deleting
	routes, err := r.ListTUNRoutes(ctx, adapterName)
	if err != nil {
		return err
	}
	for _, route := range routes {
		dest := route.Destination
		cmd := hiddenCommandContext(ctx, "netsh", "interface", "ipv4", "delete", "route", dest, adapterName)
		if out, e := cmd.CombinedOutput(); e != nil {
			// Don't fail the whole operation for one failed deletion
			_ = out
		}
	}
	return nil
}

func (r *routeManager) ListTUNRoutes(ctx context.Context, adapterName string) ([]Route, error) {
	// Use route print and filter for the adapter
	cmd := hiddenCommandContext(ctx, "route", "print")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: route print failed: %w", ErrNet004, err)
	}

	var routes []Route
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Look for lines referencing the adapter name
		if strings.Contains(line, adapterName) {
			route := parseRouteLine(line)
			if route != nil {
				route.InterfaceName = adapterName
				routes = append(routes, *route)
			}
		}
	}
	return routes, nil
}

func (r *routeManager) CleanupAll(ctx context.Context) (int, error) {
	cleaned := 0

	// Clean up IPv4 routes
	for _, netshVer := range []string{"ipv4", "ipv6"} {
		cmd := hiddenCommandContext(ctx, "netsh", "interface", netshVer, "show", "route")
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue // skip if IPv6 disabled or no routes
		}

		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Navo") {
				// Parse and delete Navo-marked routes
				fields := strings.Fields(line)
				if len(fields) > 0 {
					dest := fields[0]
					delCmd := hiddenCommandContext(ctx, "netsh", "interface", netshVer, "delete", "route", dest)
					if e := delCmd.Run(); e == nil {
						cleaned++
					}
				}
			}
		}
	}
	return cleaned, nil
}

// parseRouteLine attempts to parse a route print line into a Route struct.
func parseRouteLine(line string) *Route {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil
	}

	route := &Route{Destination: fields[0]}

	// Try to extract metric from fields
	for i, f := range fields {
		if f == "metric" && i+1 < len(fields) {
			fmt.Sscanf(fields[i+1], "%d", &route.Metric)
			break
		}
	}

	if route.Metric == 0 {
		route.Metric = 1
	}

	return route
}
