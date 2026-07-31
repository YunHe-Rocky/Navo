//go:build windows

package tun

import (
	"testing"

	"golang.org/x/sys/windows"

	"navo/internal/domain/capture"
)

func TestAdapterStateFromOperStatus(t *testing.T) {
	tests := map[uint32]capture.AdapterState{
		windows.IfOperStatusUp:             capture.AdapterEnabled,
		windows.IfOperStatusDown:           capture.AdapterDisabled,
		windows.IfOperStatusNotPresent:     capture.AdapterDisabled,
		windows.IfOperStatusLowerLayerDown: capture.AdapterDisabled,
		windows.IfOperStatusTesting:        capture.AdapterStarting,
		windows.IfOperStatusDormant:        capture.AdapterStarting,
		windows.IfOperStatusUnknown:        capture.AdapterUnknown,
	}
	for input, want := range tests {
		if got := adapterStateFromOperStatus(input); got != want {
			t.Errorf("adapterStateFromOperStatus(%d) = %s, want %s", input, got, want)
		}
	}
}

func TestWindowsAdaptersUsesNativeIPHelper(t *testing.T) {
	adapters, err := windowsAdapters()
	if err != nil {
		t.Fatal(err)
	}
	if adapters == nil {
		t.Fatal("GetAdaptersAddresses returned no adapters")
	}
}
