//go:build windows

package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("navo"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	const digest = "2e07e62ae87441c37a9ec6cf1b5be49587ee5ef92e4ce87539bf495607542df0"
	if err := verifyFileSHA256(path, digest); err != nil {
		t.Fatalf("verifyFileSHA256: %v", err)
	}
	if err := verifyFileSHA256(path, BundledWintunSHA256); err == nil {
		t.Fatal("verifyFileSHA256 accepted the wrong digest")
	}
}
