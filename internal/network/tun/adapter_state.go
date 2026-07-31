package tun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"navo/internal/domain/capture"
)

// InspectAdapter reads Windows state instead of trusting Service runtime flags.
func InspectAdapter(ctx context.Context, name string) capture.AdapterStatus {
	return inspectAdapter(ctx, strings.TrimSpace(name))
}

// WaitForAdapterState observes readiness with a bounded event-like poll. It
// returns immediately on success and never treats elapsed time as readiness.
func WaitForAdapterState(
	ctx context.Context,
	name string,
	want capture.AdapterState,
	interval time.Duration,
) (capture.AdapterStatus, error) {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		status := InspectAdapter(ctx, name)
		if status.State == want {
			return status, nil
		}
		if status.State == capture.AdapterDriverError {
			return status, fmt.Errorf("TUN adapter driver error: %s", status.Error)
		}
		select {
		case <-ctx.Done():
			return status, fmt.Errorf(
				"wait for TUN adapter %q state %s (last=%s): %w",
				name, want, status.State, ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func normalizeAdapterState(status string) capture.AdapterState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "up":
		return capture.AdapterEnabled
	case "disabled":
		return capture.AdapterDisabled
	case "disconnected":
		return capture.AdapterStarting
	case "not present", "hardware not present":
		return capture.AdapterDriverError
	case "":
		return capture.AdapterMissing
	default:
		return capture.AdapterUnknown
	}
}
