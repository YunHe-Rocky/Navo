//go:build !windows

package network

import (
	"context"
	"fmt"
)

func InspectAdapterSnapshot(context.Context, string) (AdapterSnapshot, error) {
	return AdapterSnapshot{}, fmt.Errorf("adapter inspection is supported only on Windows")
}
