package tun

import (
	"testing"

	"navo/internal/domain/capture"
)

func TestNormalizeAdapterState(t *testing.T) {
	tests := map[string]capture.AdapterState{
		"Up":           capture.AdapterEnabled,
		"Disabled":     capture.AdapterDisabled,
		"Disconnected": capture.AdapterStarting,
		"Not Present":  capture.AdapterDriverError,
		"":             capture.AdapterMissing,
		"Other":        capture.AdapterUnknown,
	}
	for input, want := range tests {
		if got := normalizeAdapterState(input); got != want {
			t.Errorf("normalizeAdapterState(%q) = %s, want %s", input, got, want)
		}
	}
}
