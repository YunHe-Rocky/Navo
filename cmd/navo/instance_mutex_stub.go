//go:build !windows

package main

type singleInstanceLock struct{}

func acquireSingleInstance() (*singleInstanceLock, bool, error) {
	return &singleInstanceLock{}, false, nil
}

func (*singleInstanceLock) Close() error { return nil }
