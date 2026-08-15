//go:build windows

package network

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"navo/internal/winprocess"
)

type systemExecutor struct{}

// NewSystemExecutor creates the production Windows command executor.
func NewSystemExecutor() Executor {
	return systemExecutor{}
}

func (systemExecutor) Run(ctx context.Context, command Command) error {
	_, err := (systemExecutor{}).RunOutput(ctx, command)
	return err
}

func (systemExecutor) RunOutput(ctx context.Context, command Command) (string, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	winprocess.ConfigureHidden(cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w: %s", command.Name, err, decodeCommandOutput(output.Bytes()))
	}
	return decodeCommandOutput(output.Bytes()), nil
}

func decodeCommandOutput(output []byte) string {
	if utf8.Valid(output) {
		return string(output)
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), output)
	if err == nil {
		return string(decoded)
	}
	return string(output)
}
