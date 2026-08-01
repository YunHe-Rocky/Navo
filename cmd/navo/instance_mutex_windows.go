//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

type singleInstanceLock struct {
	handle windows.Handle
}

func acquireSingleInstance() (*singleInstanceLock, bool, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, false, fmt.Errorf("resolve current user SID: %w", err)
	}
	sid := user.User.Sid.String()
	if sid == "" {
		return nil, false, fmt.Errorf("resolve current user SID: empty SID")
	}
	return acquireNamedInstance(`Local\Navo.` + sid)
}

func acquireNamedInstance(mutexName string) (*singleInstanceLock, bool, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, false, fmt.Errorf("encode mutex name: %w", err)
	}
	handle, createErr := windows.CreateMutex(nil, false, name)
	if errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return nil, true, nil
	}
	if createErr != nil {
		return nil, false, fmt.Errorf("CreateMutexW: %w", createErr)
	}
	return &singleInstanceLock{handle: handle}, false, nil
}

func (l *singleInstanceLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}
