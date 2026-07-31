//go:build windows

package tun

import (
	"context"
	"os/exec"

	"navo/internal/winprocess"
)

func hiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	winprocess.ConfigureHidden(command)
	return command
}
