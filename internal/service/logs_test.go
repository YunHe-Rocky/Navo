package service

import (
	"errors"
	"testing"
)

func TestCaptureLogsAreNotClassifiedAsTUN(t *testing.T) {
	if got := logServiceForMethod("capture.prepare"); got != "Capture" {
		t.Fatalf("capture service = %q, want Capture", got)
	}
	if got := logServiceForMethod("tun.status"); got != "TUN" {
		t.Fatalf("TUN service = %q, want TUN", got)
	}
}

func TestServiceIPCErrorFieldsIncludeActionableReason(t *testing.T) {
	fields := serviceIPCErrorFields("request-123", "FAILED", errors.New("backend unavailable"))
	if got := fields["request_id"]; got != "request-123" {
		t.Fatalf("request_id = %v, want request-123", got)
	}
	if got := fields["error_code"]; got != "FAILED" {
		t.Fatalf("error_code = %v, want FAILED", got)
	}
	if got := fields["reason"]; got != "backend unavailable" {
		t.Fatalf("reason = %v, want backend unavailable", got)
	}
}

func TestServiceIPCErrorFieldsHandleNilError(t *testing.T) {
	fields := serviceIPCErrorFields("request-123", "FAILED", nil)
	if got := fields["reason"]; got != "unknown service error" {
		t.Fatalf("reason = %v, want fallback", got)
	}
}
