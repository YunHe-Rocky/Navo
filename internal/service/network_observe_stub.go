//go:build !windows

package service

import (
	"context"

	"navo/internal/networkenv"
)

func observePlatformNetwork(context.Context) (platformNetworkObservation, error) {
	return platformNetworkObservation{
		Physical: networkenv.PhysicalSnapshot{Known: false},
	}, nil
}
