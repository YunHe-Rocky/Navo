//go:build windows

package main

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32DLL              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32DLL.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32DLL.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhysical        uint64
	AvailablePhysical    uint64
	TotalPageFile        uint64
	AvailablePageFile    uint64
	TotalVirtual         uint64
	AvailableVirtual     uint64
	AvailableExtendedVir uint64
}

func collectHostStatus(startedAt time.Time) HostStatus {
	status := HostStatus{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		AppVersion:    diagnosticsAppVersion,
		GoVersion:     runtime.Version(),
		LogicalCPU:    runtime.NumCPU(),
		ProcessUptime: int64(time.Since(startedAt).Seconds()),
	}
	memory := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if result, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memory))); result != 0 {
		status.MemoryTotalBytes = memory.TotalPhysical
		status.MemoryAvailable = memory.AvailablePhysical
		status.MemoryUsagePercent = float64(memory.MemoryLoad)
	}
	if ticks, _, _ := procGetTickCount64.Call(); ticks > 0 {
		status.SystemUptime = int64(uint64(ticks) / 1000)
	}
	return status
}
