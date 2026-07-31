//go:build windows

package network

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"navo/internal/winprocess"
)

type systemExecutor struct{}

// NewSystemExecutor creates the production Windows command executor.
func NewSystemExecutor() Executor {
	return systemExecutor{}
}

func (systemExecutor) Run(ctx context.Context, command Command) error {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	winprocess.ConfigureHidden(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", command.Name, err, output.String())
	}
	return nil
}
