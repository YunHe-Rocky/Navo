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

func (unsupportedPlatform) WaitForAdapter(context.Context, string, time.Duration) error {
	return fmt.Errorf("Wintun is supported only on Windows")
}
