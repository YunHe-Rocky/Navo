//go:build windows

package fsatomic

import (
	"syscall"
	"testing"
)

func TestReplaceFileRetriesTransientWindowsLock(t *testing.T) {
	originalMoveFileEx := moveFileEx
	t.Cleanup(func() { moveFileEx = originalMoveFileEx })

	attempts := 0
	moveFileEx = func(_, _ *uint16) error {
		attempts++
		if attempts < 3 {
			return syscall.Errno(32)
		}
		return nil
	}

	if err := replaceFile("source", "destination"); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestReplaceFileDoesNotRetryPermanentWindowsError(t *testing.T) {
	originalMoveFileEx := moveFileEx
	t.Cleanup(func() { moveFileEx = originalMoveFileEx })

	attempts := 0
	moveFileEx = func(_, _ *uint16) error {
		attempts++
		return syscall.Errno(87) // ERROR_INVALID_PARAMETER
	}

	if err := replaceFile("source", "destination"); err == nil {
		t.Fatal("expected replacement error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
