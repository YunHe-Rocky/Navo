//go:build windows

package winprocess

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// ConfigureHidden prevents console executables from creating or flashing a
// window. HideWindow alone is insufficient for short-lived PowerShell/netsh
// processes on Windows.
func ConfigureHidden(command *exec.Cmd) {
	if command == nil {
		return
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= createNoWindow
}
