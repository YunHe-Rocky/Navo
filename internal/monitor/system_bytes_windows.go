//go:build windows

package monitor

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/sys/windows"
)

func ReadSystemBytes() (upload, download uint64, err error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return 0, 0, err
	}
	read := 0
	for _, iface := range interfaces {
		name := strings.ToLower(iface.Name)
		if iface.Flags&net.FlagLoopback != 0 ||
			strings.Contains(name, "navo") || strings.Contains(name, "wintun") ||
			strings.Contains(name, "hyper-v") || strings.Contains(name, "vethernet") {
			continue
		}
		row := windows.MibIfRow2{InterfaceIndex: uint32(iface.Index)}
		if queryErr := windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row); queryErr != nil {
			continue
		}
		if row.OperStatus != windows.IfOperStatusUp ||
			row.Type == windows.IF_TYPE_SOFTWARE_LOOPBACK || row.Type == windows.IF_TYPE_TUNNEL {
			continue
		}
		upload += row.OutOctets
		download += row.InOctets
		read++
	}
	if read == 0 {
		return 0, 0, fmt.Errorf("no active physical network interface counters")
	}
	return upload, download, nil
}
