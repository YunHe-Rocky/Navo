//go:build windows

package network

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"
)

type windowsPlatform struct{}

// NewPlatform creates the production Windows integration.
func NewPlatform() Platform {
	return windowsPlatform{}
}

func (windowsPlatform) Preflight(_ context.Context, cfg Config) error {
	if err := checkAdministrator(); err != nil {
		return err
	}
	if cfg.WintunDLLPath == "" {
		return fmt.Errorf("WintunDLLPath is required")
	}
	if _, err := os.Stat(cfg.WintunDLLPath); err != nil {
		return fmt.Errorf("Wintun DLL unavailable: %w", err)
	}
	if err := verifyFileSHA256(cfg.WintunDLLPath, cfg.WintunSHA256); err != nil {
		return err
	}
	dll, err := syscall.LoadDLL(cfg.WintunDLLPath)
	if err != nil {
		return fmt.Errorf("load Wintun DLL: %w", err)
	}
	defer dll.Release()
	for _, name := range []string{"WintunCreateAdapter", "WintunOpenAdapter", "WintunCloseAdapter", "WintunGetRunningDriverVersion"} {
		if _, err := dll.FindProc(name); err != nil {
			return fmt.Errorf("invalid Wintun DLL, missing %s: %w", name, err)
		}
	}
	return nil
}

func verifyFileSHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Wintun DLL: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash Wintun DLL: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("Wintun DLL SHA-256 mismatch: got %s", actual)
	}
	return nil
}

func checkAdministrator() error {
	dll := syscall.NewLazyDLL("shell32.dll")
	proc := dll.NewProc("IsUserAnAdmin")
	result, _, callErr := proc.Call()
	if result == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("check administrator token: %w", callErr)
		}
		return fmt.Errorf("administrator privileges are required for TUN mode")
	}
	return nil
}
