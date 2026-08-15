//go:build !windows

package service

import "context"

func verifyRuntimeRouting(context.Context, int, string) (RuntimeRoutingVerification, error) {
	return RuntimeRoutingVerification{}, nil
}
