package service

import (
	"context"
	"errors"
	"testing"
)

func TestVerifySelectedOutboundReachableRejectsMissingRouteOutsideDirectMode(t *testing.T) {
	service := &Service{}
	service.runtime.Mode = runtimeModeBypassMainland

	err := service.verifySelectedOutboundReachable(context.Background())
	var captureErr *captureTransitionError
	if !errors.As(err, &captureErr) || captureErr.code != "OUTBOUND_REQUIRED" {
		t.Fatalf("missing route error = %T %v, want OUTBOUND_REQUIRED", err, err)
	}
}

func TestVerifySelectedOutboundReachableAllowsExplicitDirectMode(t *testing.T) {
	service := &Service{}
	service.runtime.Mode = runtimeModeDirect
	if err := service.verifySelectedOutboundReachable(context.Background()); err != nil {
		t.Fatalf("explicit direct mode rejected: %v", err)
	}
}
