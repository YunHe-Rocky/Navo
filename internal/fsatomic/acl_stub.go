//go:build !windows

package fsatomic

func restrictPath(string) error { return nil }
