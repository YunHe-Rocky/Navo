//go:build !windows

package monitor

import "fmt"

func ReadSystemBytes() (uint64, uint64, error) {
	return 0, 0, fmt.Errorf("system interface counters are unsupported on this platform")
}
