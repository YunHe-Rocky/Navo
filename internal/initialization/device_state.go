package initialization

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"navo/internal/fsatomic"
)

const deviceStateVersion = 1

var errDeviceStateUnprotect = errors.New("device state cannot be unprotected")

type deviceState struct {
	Version         int    `json:"version"`
	InstallID       string `json:"install_id"`
	ProtectedSecret string `json:"protected_secret"`
	Checksum        string `json:"checksum"`
}

func createDeviceState(path string, protect func([]byte) ([]byte, error)) error {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate install secret: %w", err)
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("generate install ID: %w", err)
	}
	protected, err := protect(secret)
	if err != nil {
		return fmt.Errorf("protect install secret: %w", err)
	}
	state := deviceState{
		Version: deviceStateVersion, InstallID: hex.EncodeToString(idBytes),
		ProtectedSecret: base64.StdEncoding.EncodeToString(protected),
	}
	state.Checksum = deviceChecksum(state, secret)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode device state: %w", err)
	}
	if err := fsatomic.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("persist device state: %w", err)
	}
	return nil
}

func readDeviceState(path string, unprotect func([]byte) ([]byte, error)) (deviceState, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return deviceState{}, nil, err
	}
	var state deviceState
	if err := json.Unmarshal(data, &state); err != nil {
		return deviceState{}, nil, fmt.Errorf("decode device state: %w", err)
	}
	protected, err := base64.StdEncoding.DecodeString(state.ProtectedSecret)
	if err != nil {
		return deviceState{}, nil, fmt.Errorf("decode protected install secret: %w", err)
	}
	secret, err := unprotect(protected)
	if err != nil {
		return state, nil, fmt.Errorf("%w: %v", errDeviceStateUnprotect, err)
	}
	return state, secret, nil
}

func validateDeviceState(state deviceState, secret []byte) error {
	if state.Version != deviceStateVersion {
		return fmt.Errorf("unsupported device-state version %d", state.Version)
	}
	if len(secret) != 32 || len(state.InstallID) != 32 {
		return fmt.Errorf("device-state identity is incomplete")
	}
	want, err := hex.DecodeString(state.Checksum)
	if err != nil {
		return fmt.Errorf("decode device-state checksum: %w", err)
	}
	got, err := hex.DecodeString(deviceChecksum(state, secret))
	if err != nil {
		return err
	}
	if !hmac.Equal(got, want) {
		return fmt.Errorf("device-state integrity check failed")
	}
	return nil
}

func deviceChecksum(state deviceState, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%d:%s", state.Version, state.InstallID)
	return hex.EncodeToString(mac.Sum(nil))
}

func isUnprotectError(err error) bool {
	return errors.Is(err, errDeviceStateUnprotect)
}
