//go:build windows

package tun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// routeManager remains for diagnostics compatibility. Production TUN
// mutations are exclusively owned by network.Manager and Journal V2.
type routeManager struct{}

func NewRouteManager() RouteManager { return &routeManager{} }

func (*routeManager) AddRoutes(context.Context, string, []Route) error {
	return fmt.Errorf("%s: legacy route mutation is disabled; use the V2 network transaction", ErrNet004)
}

func (*routeManager) RemoveRoutes(context.Context, string) error {
	return fmt.Errorf("%s: legacy route mutation is disabled; use the V2 network transaction", ErrNet004)
}

func (*routeManager) ListTUNRoutes(ctx context.Context, adapterName string) ([]Route, error) {
	quoted := "'" + strings.ReplaceAll(adapterName, "'", "''") + "'"
	script := "[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new();" +
		"@(Get-NetRoute -InterfaceAlias " + quoted + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue|ForEach-Object {[pscustomobject]@{destination=[string]$_.DestinationPrefix;gateway=[string]$_.NextHop;metric=[int]$_.RouteMetric;interface_name=[string]$_.InterfaceAlias}})|ConvertTo-Json -Compress"
	cmd := hiddenCommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: query structured routes: %w", ErrNet004, err)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var list []Route
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
			return nil, fmt.Errorf("%s: decode structured routes: %w", ErrNet004, err)
		}
		return list, nil
	}
	var single Route
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return nil, fmt.Errorf("%s: decode structured route: %w", ErrNet004, err)
	}
	return []Route{single}, nil
}

func (*routeManager) CleanupAll(context.Context) (int, error) {
	return 0, fmt.Errorf("%s: bulk route cleanup is disabled; exact Journal V2 ownership is required", ErrNet004)
}
