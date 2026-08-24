package service

import (
	"errors"
	"testing"
)

func TestValidateRuntimeExitIdentity(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		directIP  string
		proxyIP   string
		wantError bool
	}{
		{name: "global proxy differs", mode: runtimeModeGlobal, directIP: "198.51.100.10", proxyIP: "203.0.113.20"},
		{name: "global direct leak", mode: runtimeModeGlobal, directIP: "198.51.100.10", proxyIP: "198.51.100.10", wantError: true},
		{name: "direct policy keeps exit", mode: runtimeModeDirect, directIP: "2001:db8::1", proxyIP: "2001:0db8:0:0:0:0:0:1"},
		{name: "direct policy changed exit", mode: runtimeModeDirect, directIP: "198.51.100.10", proxyIP: "203.0.113.20", wantError: true},
		{name: "invalid evidence", mode: runtimeModeGlobal, directIP: "", proxyIP: "203.0.113.20", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRuntimeExitIdentity(test.mode, test.directIP, test.proxyIP)
			if (err != nil) != test.wantError {
				t.Fatalf("validateRuntimeExitIdentity() error = %v, wantError=%t", err, test.wantError)
			}
			if err != nil {
				var captureErr *captureTransitionError
				if !errors.As(err, &captureErr) || captureErr.code != proxyExitIPVerifyFailed {
					t.Fatalf("error = %T %v, want %s", err, err, proxyExitIPVerifyFailed)
				}
			}
		})
	}
}
