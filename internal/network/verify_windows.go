//go:build windows

package network

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetIPInterfaceEntry = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetIpInterfaceEntry")

const adapterReadyStabilityWindow = 2 * time.Second

type adapterReadinessTracker struct {
	interfaceGUID  string
	interfaceLUID  uint64
	interfaceIndex uint32
	readySince     time.Time
}

func (t *adapterReadinessTracker) reset() {
	*t = adapterReadinessTracker{}
}

func (t *adapterReadinessTracker) observe(snapshot AdapterSnapshot, expectedAddress string, expectedMTU int, now time.Time) bool {
	ready := isOwnedTUNAdapter(snapshot) && strings.EqualFold(snapshot.OperationalStatus, "Up") &&
		snapshot.MTU == expectedMTU && containsString(snapshot.IPv4Addresses, expectedAddress)
	if !ready {
		t.reset()
		return false
	}
	if t.readySince.IsZero() || t.interfaceGUID != snapshot.InterfaceGUID ||
		t.interfaceLUID != snapshot.InterfaceLUID || t.interfaceIndex != snapshot.InterfaceIndex {
		t.interfaceGUID = snapshot.InterfaceGUID
		t.interfaceLUID = snapshot.InterfaceLUID
		t.interfaceIndex = snapshot.InterfaceIndex
		t.readySince = now
		return false
	}
	return !now.Before(t.readySince.Add(adapterReadyStabilityWindow))
}

// InspectAdapterSnapshot reads identity, operational status, addresses and MTU
// through the Windows IP Helper API. Readiness must not depend on starting a
// PowerShell process while the Windows network stack is still converging.
func InspectAdapterSnapshot(ctx context.Context, name string) (AdapterSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return AdapterSnapshot{}, err
	}
	snapshots, err := windowsAdapterSnapshots(name)
	if err != nil {
		return AdapterSnapshot{}, fmt.Errorf("inspect adapter %q: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return AdapterSnapshot{}, err
	}
	if len(snapshots) == 0 {
		return AdapterSnapshot{}, fmt.Errorf("adapter %q is missing", name)
	}
	if len(snapshots) != 1 {
		return AdapterSnapshot{}, &TUNError{Code: ErrTUNAdapterConflict, Stage: TUNStageAdapterReady, Resource: name, Expected: "one adapter", Actual: fmt.Sprintf("%d adapters", len(snapshots))}
	}
	return snapshots[0], nil
}

func windowsAdapterSnapshots(name string) ([]AdapterSnapshot, error) {
	flags := uint32(
		windows.GAA_FLAG_INCLUDE_ALL_INTERFACES |
			windows.GAA_FLAG_SKIP_ANYCAST |
			windows.GAA_FLAG_SKIP_MULTICAST |
			windows.GAA_FLAG_SKIP_DNS_SERVER,
	)
	size := uint32(15 * 1024)
	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil, err
		}

		result := make([]AdapterSnapshot, 0, 1)
		for adapter := first; adapter != nil; adapter = adapter.Next {
			if !strings.EqualFold(windows.UTF16PtrToString(adapter.FriendlyName), name) {
				continue
			}
			row := windows.MibIfRow2{InterfaceLuid: adapter.Luid}
			if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormalWithoutStatistics, &row); err != nil {
				return nil, fmt.Errorf("query interface %d: %w", adapter.IfIndex, err)
			}
			mtu, err := windowsIPv4MTU(adapter.Luid, adapter.IfIndex)
			if err != nil {
				return nil, fmt.Errorf("query IPv4 interface %d MTU: %w", adapter.IfIndex, err)
			}
			snapshot := AdapterSnapshot{
				Name:                 windows.UTF16ToString(row.Alias[:]),
				InterfaceDescription: windows.UTF16ToString(row.Description[:]),
				HardwareInterface:    row.InterfaceAndOperStatusFlags&1 != 0,
				InterfaceIndex:       row.InterfaceIndex,
				InterfaceGUID:        row.InterfaceGuid.String(),
				InterfaceLUID:        row.InterfaceLuid,
				OperationalStatus:    windowsOperStatus(row.OperStatus),
				MTU:                  mtu,
			}
			for address := adapter.FirstUnicastAddress; address != nil; address = address.Next {
				ip := address.Address.IP()
				if ip == nil {
					continue
				}
				value := fmt.Sprintf("%s/%d", ip.String(), address.OnLinkPrefixLength)
				if ipv4 := ip.To4(); ipv4 != nil {
					value = fmt.Sprintf("%s/%d", ipv4.String(), address.OnLinkPrefixLength)
					snapshot.IPv4Addresses = append(snapshot.IPv4Addresses, value)
				} else {
					snapshot.IPv6Addresses = append(snapshot.IPv6Addresses, value)
				}
			}
			result = append(result, snapshot)
		}
		return result, nil
	}
	return nil, windows.ERROR_BUFFER_OVERFLOW
}

func windowsIPv4MTU(luid uint64, interfaceIndex uint32) (int, error) {
	row := windows.MibIpInterfaceRow{
		Family:         windows.AF_INET,
		InterfaceLuid:  luid,
		InterfaceIndex: interfaceIndex,
	}
	result, _, _ := procGetIPInterfaceEntry.Call(uintptr(unsafe.Pointer(&row)))
	if result != 0 {
		return 0, syscall.Errno(result)
	}
	return int(row.NlMtu), nil
}

func windowsOperStatus(status uint32) string {
	switch status {
	case windows.IfOperStatusUp:
		return "Up"
	case windows.IfOperStatusDown:
		return "Down"
	case windows.IfOperStatusTesting:
		return "Testing"
	case windows.IfOperStatusDormant:
		return "Dormant"
	case windows.IfOperStatusNotPresent:
		return "Not Present"
	case windows.IfOperStatusLowerLayerDown:
		return "Lower Layer Down"
	default:
		return "Unknown"
	}
}

func (windowsPlatform) WaitForAdapterReady(
	ctx context.Context,
	expectedName, expectedAddress string,
	expectedMTU int,
	timeout time.Duration,
) (AdapterSnapshot, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var last AdapterSnapshot
	var lastErr error
	var readiness adapterReadinessTracker
	for {
		snapshot, err := InspectAdapterSnapshot(waitCtx, expectedName)
		if err == nil {
			last = snapshot
			lastErr = nil
			if readiness.observe(snapshot, expectedAddress, expectedMTU, time.Now()) {
				return snapshot, nil
			}
		} else {
			readiness.reset()
			// Preserve the last meaningful observation. InspectAdapterSnapshot
			// returns waitCtx.Err once the deadline is reached; replacing a real
			// adapter snapshot with that generic error makes readiness failures
			// impossible to diagnose.
			if waitCtx.Err() == nil {
				lastErr = err
			}
			if tunErr := asTUNError(err); tunErr != nil && tunErr.Code == ErrTUNAdapterConflict {
				return AdapterSnapshot{}, tunErr
			}
		}
		select {
		case <-waitCtx.Done():
			actual := fmt.Sprintf("name=%s description=%s hardware=%t status=%s index=%d guid=%s mtu=%d addresses=%v", last.Name, last.InterfaceDescription, last.HardwareInterface, last.OperationalStatus, last.InterfaceIndex, last.InterfaceGUID, last.MTU, last.IPv4Addresses)
			if lastErr != nil {
				actual = lastErr.Error()
			}
			return last, &TUNError{Code: ErrTUNAdapterNotReady, Stage: TUNStageAdapterReady, Resource: expectedName, Expected: fmt.Sprintf("Up %s MTU=%d", expectedAddress, expectedMTU), Actual: actual, Cause: waitCtx.Err()}
		case <-ticker.C:
		}
	}
}

func (windowsPlatform) VerifyControlPlane(
	ctx context.Context,
	plan TUNActivationPlan,
	adapter AdapterSnapshot,
) error {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	var lastErr error
	for {
		lastErr = verifyControlPlaneOnce(ctx, plan, adapter)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-deadline.C:
			return lastErr
		case <-ticker.C:
		}
	}
}

func verifyControlPlaneOnce(
	ctx context.Context,
	plan TUNActivationPlan,
	adapter AdapterSnapshot,
) error {
	executor := NewSystemExecutor()
	endpointMetric := 40000 + sessionMetric(plan.SessionID)
	for _, endpoint := range plan.EndpointRoutes {
		prefix, family := endpoint.EndpointIP+"/32", "IPv4"
		if strings.Contains(endpoint.EndpointIP, ":") {
			prefix, family = endpoint.EndpointIP+"/128", "IPv6"
		}
		script := "$r=@(Get-NetRoute -AddressFamily " + family + " -DestinationPrefix " + psQuote(prefix) + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue|Where-Object {" +
			"[uint32]$_.InterfaceIndex -eq " + fmt.Sprint(endpoint.InterfaceIndex) + " -and [string]$_.NextHop -eq " + psQuote(endpoint.NextHop) + " -and [int]$_.RouteMetric -eq " + fmt.Sprint(endpointMetric) + "});" +
			"if($r.Count -ne 1){throw 'endpoint route changed'};" +
			"$a=Get-NetAdapter -InterfaceIndex " + fmt.Sprint(endpoint.InterfaceIndex) + " -ErrorAction Stop;" +
			"if([string]$a.InterfaceGuid -ne " + psQuote(endpoint.InterfaceGUID) + "){throw 'endpoint adapter changed'}"
		if _, err := executor.RunOutput(ctx, powershell(script)); err != nil {
			return &TUNError{Code: ErrTUNEndpointLoopDetected, Stage: TUNStageControlPlaneVerified, Resource: endpoint.EndpointIP, Expected: fmt.Sprintf("prefix=%s if=%d guid=%s next=%s metric=%d", prefix, endpoint.InterfaceIndex, endpoint.InterfaceGUID, endpoint.NextHop, endpointMetric), Actual: "exact route differs", Cause: err}
		}
	}
	for _, publicIP := range []string{"1.1.1.1", "8.8.8.8"} {
		script := "$r=Find-NetRoute -RemoteIPAddress " + psQuote(publicIP) + " -ErrorAction Stop|Select-Object -First 1;if([uint32]$r.InterfaceIndex -ne " + fmt.Sprint(adapter.InterfaceIndex) + "){throw 'public route bypasses TUN'}"
		if _, err := executor.RunOutput(ctx, powershell(script)); err != nil {
			return &TUNError{Code: ErrTUNPublicRouteNotCaptured, Stage: TUNStageControlPlaneVerified, Resource: publicIP, Expected: fmt.Sprintf("interface_index=%d", adapter.InterfaceIndex), Actual: "best route differs", Cause: err}
		}
	}
	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		script := "$r=@(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix " + psQuote(prefix) + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue|Where-Object {[uint32]$_.InterfaceIndex -eq " + fmt.Sprint(adapter.InterfaceIndex) + " -and [string]$_.NextHop -eq " + psQuote(plan.TUNIPv4Peer) + " -and [int]$_.RouteMetric -eq 1});if($r.Count -ne 1){throw 'split route mismatch'}"
		if _, err := executor.RunOutput(ctx, powershell(script)); err != nil {
			return &TUNError{Code: ErrTUNSplitRouteFailed, Stage: TUNStageControlPlaneVerified, Resource: prefix, Expected: fmt.Sprintf("if=%d next=%s metric=1", adapter.InterfaceIndex, plan.TUNIPv4Peer), Actual: "exact route missing", Cause: err}
		}
	}
	tag := "Navo:TUN:" + plan.SessionID
	nrptScript := "$r=@(Get-DnsClientNrptRule -ErrorAction SilentlyContinue|Where-Object {(@($_.Namespace) -contains '.') -and [string]$_.Comment -eq " + psQuote(tag) + " -and (@($_.NameServers) -contains " + psQuote(plan.TUNDNSIPv4) + ")});if($r.Count -ne 1){throw 'NRPT mismatch'}"
	if _, err := executor.RunOutput(ctx, powershell(nrptScript)); err != nil {
		return &TUNError{Code: ErrTUNNRPTFailed, Stage: TUNStageControlPlaneVerified, Resource: tag, Expected: plan.TUNDNSIPv4, Actual: "exact NRPT rule missing", Cause: err}
	}
	if plan.IPv6Mode == IPv6Block {
		name := "Navo TUN IPv6 Block " + plan.SessionID
		script := "$r=@(Get-NetFirewallRule -DisplayName " + psQuote(name) + " -ErrorAction SilentlyContinue|Where-Object {[string]$_.Enabled -eq 'True'});if($r.Count -ne 1){throw 'IPv6 firewall rule mismatch'}"
		if _, err := executor.RunOutput(ctx, powershell(script)); err != nil {
			return &TUNError{Code: ErrTUNIPv6PolicyFailed, Stage: TUNStageControlPlaneVerified, Resource: name, Expected: "one enabled owned rule", Actual: "exact rule missing or disabled", Cause: err}
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
