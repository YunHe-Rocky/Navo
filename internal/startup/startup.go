package startup

import (
	"context"
	"strings"
)

const (
	ModeSystemProxy = "system_proxy"
	ModeTUN         = "tun"
)

type Settings struct {
	Supported  bool   `json:"supported"`
	Enabled    bool   `json:"enabled"`
	Mode       string `json:"mode"`
	Registered bool   `json:"registered"`
	LastError  string `json:"last_error,omitempty"`
}

type Controller interface {
	Status(context.Context) (Settings, error)
	Configure(context.Context, bool, string) (Settings, error)
}

func ValidMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case ModeSystemProxy, ModeTUN:
		return true
	default:
		return false
	}
}
