//go:build windows

package tun

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"navo/internal/domain/capture"
)

func inspectAdapter(ctx context.Context, name string) capture.AdapterStatus {
	result := capture.AdapterStatus{Name: name, State: capture.AdapterMissing}
	if name == "" {
		result.State = capture.AdapterUnknown
		result.Error = "adapter name is empty"
		return result
	}
	if err := ctx.Err(); err != nil {
		result.State = capture.AdapterUnknown
		result.Error = err.Error()
		return result
	}

	adapters, err := windowsAdapters()
	if err != nil {
		result.State = capture.AdapterUnknown
		result.Error = fmt.Sprintf("inspect adapter: %v", err)
		return result
	}
	for adapter := adapters; adapter != nil; adapter = adapter.Next {
		friendlyName := windows.UTF16PtrToString(adapter.FriendlyName)
		if !strings.EqualFold(friendlyName, name) {
			continue
		}
		result.Name = friendlyName
		result.InterfaceGUID = windows.BytePtrToString(adapter.AdapterName)
		result.InterfaceIndex = int(adapter.IfIndex)
		result.State = adapterStateFromOperStatus(adapter.OperStatus)
		if result.State == capture.AdapterUnknown {
			result.Error = fmt.Sprintf(
				"unrecognized Windows adapter operational status: %d",
				adapter.OperStatus,
			)
		}
		return result
	}
	return result
}

func windowsAdapters() (*windows.IpAdapterAddresses, error) {
	flags := uint32(
		windows.GAA_FLAG_INCLUDE_ALL_INTERFACES |
			windows.GAA_FLAG_SKIP_UNICAST |
			windows.GAA_FLAG_SKIP_ANYCAST |
			windows.GAA_FLAG_SKIP_MULTICAST |
			windows.GAA_FLAG_SKIP_DNS_SERVER,
	)
	size := uint32(15 * 1024)
	for attempts := 0; attempts < 2; attempts++ {
		buffer := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC,
			flags,
			0,
			first,
			&size,
		)
		if err == nil {
			return first, nil
		}
		if err != windows.ERROR_BUFFER_OVERFLOW {
			return nil, err
		}
	}
	return nil, windows.ERROR_BUFFER_OVERFLOW
}

func adapterStateFromOperStatus(status uint32) capture.AdapterState {
	switch status {
	case windows.IfOperStatusUp:
		return capture.AdapterEnabled
	case windows.IfOperStatusDown,
		windows.IfOperStatusNotPresent,
		windows.IfOperStatusLowerLayerDown:
		return capture.AdapterDisabled
	case windows.IfOperStatusTesting, windows.IfOperStatusDormant:
		return capture.AdapterStarting
	default:
		return capture.AdapterUnknown
	}
}
