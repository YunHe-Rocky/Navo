//go:build !windows

package startup

import (
	"context"
	"fmt"
)

type unsupportedController struct{}

func New(string, string) Controller { return unsupportedController{} }

func (unsupportedController) Status(context.Context) (Settings, error) {
	return Settings{Supported: false, Mode: ModeSystemProxy}, nil
}

func (unsupportedController) Configure(context.Context, bool, string) (Settings, error) {
	return Settings{}, fmt.Errorf("login startup is supported only on Windows")
}
