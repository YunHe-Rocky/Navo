//go:build !windows

package tun

import (
	"context"

	"navo/internal/domain/capture"
)

func inspectAdapter(_ context.Context, name string) capture.AdapterStatus {
	return capture.AdapterStatus{Name: name, State: capture.AdapterMissing}
}
