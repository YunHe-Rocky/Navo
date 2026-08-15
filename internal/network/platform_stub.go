//go:build !windows

package network

import (
	"context"
	"fmt"
	"time"
)

type unsupportedPlatform struct{}

func NewPlatform() Platform {
	return unsupportedPlatform{}
}

func (unsupportedPlatform) Preflight(context.Context, Config) error {
	return fmt.Errorf("Wintun is supported only on Windows")
}

func (unsupportedPlatform) WaitForAdapterReady(context.Context, string, string, int, time.Duration) (AdapterSnapshot, error) {
	return AdapterSnapshot{}, fmt.Errorf("Wintun is supported only on Windows")
}

func (unsupportedPlatform) VerifyControlPlane(context.Context, TUNActivationPlan, AdapterSnapshot) error {
	return fmt.Errorf("Wintun is supported only on Windows")
}
