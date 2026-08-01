package initialization

import (
	"fmt"
	"os"
	"path/filepath"

	"navo/internal/securestore"
)

const (
	ErrorDeviceStateInvalid = "NAVO_INIT_DEVICE_STATE_INVALID"
	ErrorPrivacyResetFailed = "NAVO_INIT_PRIVACY_RESET_FAILED"
	ErrorMigrationFailed    = "NAVO_INIT_MIGRATION_FAILED"
	ErrorStorageUnavailable = "NAVO_INIT_STORAGE_UNAVAILABLE"
)

type Result struct {
	FirstRun       bool
	Migrated       bool
	ForeignContext bool
	PrivacyReset   bool
	Ready          bool
	ErrorCode      string
}

type Options struct {
	Protect   func([]byte) ([]byte, error)
	Unprotect func([]byte) ([]byte, error)
}

func Run(dataDir string) (Result, error) {
	return RunWithOptions(dataDir, Options{
		Protect: securestore.Protect, Unprotect: securestore.Unprotect,
	})
}

func RunWithOptions(dataDir string, options Options) (Result, error) {
	result := Result{}
	if dataDir == "" {
		result.ErrorCode = ErrorStorageUnavailable
		return result, fmt.Errorf("data directory is empty")
	}
	if options.Protect == nil || options.Unprotect == nil {
		result.ErrorCode = ErrorStorageUnavailable
		return result, fmt.Errorf("device-state protector is not configured")
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		result.ErrorCode = ErrorStorageUnavailable
		return result, fmt.Errorf("create data directory: %w", err)
	}
	statePath := filepath.Join(dataDir, "device-state.dat")
	state, secret, err := readDeviceState(statePath, options.Unprotect)
	switch {
	case os.IsNotExist(err):
		result.FirstRun = true
		if err := cleanupLegacyEnvironment(dataDir); err != nil {
			result.ErrorCode = ErrorMigrationFailed
			return result, err
		}
		if err := createDeviceState(statePath, options.Protect); err != nil {
			result.ErrorCode = ErrorStorageUnavailable
			return result, err
		}
		result.Ready = true
		return result, nil
	case isUnprotectError(err):
		result.ForeignContext = true
		if resetErr := resetForeignContext(dataDir); resetErr != nil {
			result.ErrorCode = ErrorPrivacyResetFailed
			return result, resetErr
		}
		if cleanupErr := cleanupLegacyEnvironment(dataDir); cleanupErr != nil {
			result.ErrorCode = ErrorPrivacyResetFailed
			return result, cleanupErr
		}
		if createErr := createDeviceState(statePath, options.Protect); createErr != nil {
			result.ErrorCode = ErrorPrivacyResetFailed
			return result, createErr
		}
		result.Migrated = true
		result.PrivacyReset = true
		result.Ready = true
		return result, nil
	case err != nil:
		result.ErrorCode = ErrorDeviceStateInvalid
		return result, err
	}
	if err := validateDeviceState(state, secret); err != nil {
		result.ErrorCode = ErrorDeviceStateInvalid
		return result, err
	}
	if err := cleanupLegacyEnvironment(dataDir); err != nil {
		result.ErrorCode = ErrorMigrationFailed
		return result, err
	}
	result.Ready = true
	return result, nil
}
