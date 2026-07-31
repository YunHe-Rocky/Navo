//go:build !windows

package main

import (
	"runtime"
	"time"
)

func collectHostStatus(startedAt time.Time) HostStatus {
	return HostStatus{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		AppVersion:    diagnosticsAppVersion,
		GoVersion:     runtime.Version(),
		LogicalCPU:    runtime.NumCPU(),
		ProcessUptime: int64(time.Since(startedAt).Seconds()),
	}
}
