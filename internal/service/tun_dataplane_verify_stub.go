//go:build !windows

package service

import (
	"context"
	"fmt"
)

type unsupportedTUNDataPlaneVerifier struct{}

func newTUNDataPlaneVerifier() TUNDataPlaneVerifier { return unsupportedTUNDataPlaneVerifier{} }
func (unsupportedTUNDataPlaneVerifier) CaptureDirectIP(context.Context) (string, error) {
	return "", fmt.Errorf("TUN verification is supported only on Windows")
}
func (unsupportedTUNDataPlaneVerifier) Verify(context.Context, VerifyRequest) (VerifyResult, error) {
	return VerifyResult{}, fmt.Errorf("TUN verification is supported only on Windows")
}
