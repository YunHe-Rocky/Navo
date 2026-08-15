//go:build !windows

package fsatomic

func restrictPath(string) error { return nil }

func RepairTree(string) error { return nil }
