//go:build !windows

package securestore

import "fmt"

func Protect([]byte) ([]byte, error) {
	return nil, fmt.Errorf("secure storage is only available on Windows")
}

func Unprotect([]byte) ([]byte, error) {
	return nil, fmt.Errorf("secure storage is only available on Windows")
}
