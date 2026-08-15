//go:build !windows

package network

import (
	"context"
	"fmt"
)

type unsupportedExecutor struct{}

func NewSystemExecutor() Executor {
	return unsupportedExecutor{}
}

func (unsupportedExecutor) Run(context.Context, Command) error {
	return fmt.Errorf("TUN networking is supported only on Windows")
}

func (unsupportedExecutor) RunOutput(context.Context, Command) (string, error) {
	return "", fmt.Errorf("TUN networking is supported only on Windows")
}
