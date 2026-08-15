//go:build !windows

package network

import (
	"context"
	"fmt"
)

func BuildTUNActivationPlan(context.Context, ActivationPlanRequest) (TUNActivationPlan, error) {
	return TUNActivationPlan{}, fmt.Errorf("TUN activation planning is supported only on Windows")
}

func FindPhysicalRoute(context.Context, string, string) (EndpointRoutePlan, error) {
	return EndpointRoutePlan{}, fmt.Errorf("physical route discovery is supported only on Windows")
}
