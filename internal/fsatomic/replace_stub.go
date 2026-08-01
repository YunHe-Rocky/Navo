//go:build !windows

package fsatomic

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
