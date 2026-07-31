package network

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type operation struct {
	name  string
	apply Command
	undo  Command
}

// Manager applies and rolls back all TUN-related system networking changes.
type Manager struct {
	mu       sync.Mutex
	cfg      Config
	executor Executor
	platform Platform
	active   bool
}

// NewManager creates a network manager. cfg.Enabled=false produces an inert manager.
func NewManager(cfg Config, executor Executor, platform Platform) (*Manager, error) {
	cfg.withDefaults()
	if cfg.Enabled {
		if cfg.JournalPath == "" {
			return nil, fmt.Errorf("JournalPath is required when TUN mode is enabled")
		}
		if err := cfg.validate(); err != nil {
			return nil, err
		}
		if executor == nil || platform == nil {
			return nil, fmt.Errorf("executor and platform are required")
		}
	}
	return &Manager{cfg: cfg, executor: executor, platform: platform}, nil
}

// Preflight validates administrator access and Wintun before sing-box starts.
func (m *Manager) Preflight(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}
	return m.platform.Preflight(ctx, m.cfg)
}

// Activate waits for sing-box to create the Wintun adapter, then applies policy transactionally.
func (m *Manager) Activate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	if m.active {
		return nil
	}
	if err := m.recoverLocked(ctx); err != nil {
		return fmt.Errorf("recover previous network state: %w", err)
	}
	if err := m.platform.WaitForAdapter(ctx, m.cfg.AdapterName, m.cfg.AdapterTimeout); err != nil {
		return err
	}

	value := &journal{
		Version:     1,
		AdapterName: m.cfg.AdapterName,
		SessionID:   strconv.FormatInt(time.Now().UTC().UnixNano(), 36),
		CreatedAt:   time.Now().UTC(),
	}
	if err := writeJournal(m.cfg.JournalPath, value); err != nil {
		return err
	}

	for _, op := range m.operations(value.SessionID) {
		// Persist intent before mutation. Recovery treats pending undo as idempotent.
		value.Actions = append(value.Actions, journalAction{Name: op.name, Undo: op.undo, Status: actionPending})
		if err := writeJournal(m.cfg.JournalPath, value); err != nil {
			_ = m.rollbackLocked(ctx, value)
			return err
		}
		if err := m.executor.Run(ctx, op.apply); err != nil {
			rollbackErr := m.rollbackLocked(ctx, value)
			return errors.Join(fmt.Errorf("apply %s: %w", op.name, err), rollbackErr)
		}
		value.Actions[len(value.Actions)-1].Status = actionApplied
		if err := writeJournal(m.cfg.JournalPath, value); err != nil {
			rollbackErr := m.rollbackLocked(ctx, value)
			return errors.Join(err, rollbackErr)
		}
	}
	m.active = true
	return nil
}

// Deactivate restores the baseline network state in reverse order.
func (m *Manager) Deactivate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	err := m.recoverLocked(ctx)
	if err == nil {
		m.active = false
	}
	return err
}

// Recover replays a journal left by a crash. It is safe to call on every service startup.
func (m *Manager) Recover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	return m.recoverLocked(ctx)
}

func (m *Manager) recoverLocked(ctx context.Context) error {
	value, err := readJournal(m.cfg.JournalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return m.rollbackLocked(ctx, value)
}

func (m *Manager) rollbackLocked(ctx context.Context, value *journal) error {
	if err := m.validateRecoveryJournal(value); err != nil {
		return err
	}
	var rollbackErr error
	for index := len(value.Actions) - 1; index >= 0; index-- {
		action := value.Actions[index]
		undo, err := m.safeUndoForAction(value, action.Name)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		if err := m.executor.Run(ctx, undo); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("undo %s: %w", action.Name, err))
			continue
		}
		value.Actions = append(value.Actions[:index], value.Actions[index+1:]...)
		if err := writeJournal(m.cfg.JournalPath, value); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			break
		}
	}
	if rollbackErr != nil {
		return rollbackErr
	}
	if err := os.Remove(m.cfg.JournalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove network journal: %w", err)
	}
	return nil
}

func (m *Manager) validateRecoveryJournal(value *journal) error {
	if value == nil || value.Version != 1 {
		return fmt.Errorf("invalid network recovery journal")
	}
	if value.AdapterName == "" || value.SessionID == "" || len(value.SessionID) > 64 {
		return fmt.Errorf("network recovery journal has an invalid identity")
	}
	for _, char := range value.SessionID {
		if (char < '0' || char > '9') && (char < 'a' || char > 'z') {
			return fmt.Errorf("network recovery journal has an invalid session id")
		}
	}
	recoveryConfig := m.cfg
	recoveryConfig.AdapterName = value.AdapterName
	if err := recoveryConfig.validate(); err != nil {
		return fmt.Errorf("network recovery journal has an unsafe adapter: %w", err)
	}
	return nil
}

func (m *Manager) safeUndoForAction(value *journal, name string) (Command, error) {
	recoveryConfig := m.cfg
	recoveryConfig.AdapterName = value.AdapterName
	recovery := &Manager{cfg: recoveryConfig}
	for _, op := range recovery.operations(value.SessionID) {
		if op.name == name {
			return op.undo, nil
		}
	}

	const endpointPrefix = "proxy endpoint bypass "
	if !strings.HasPrefix(name, endpointPrefix) {
		return Command{}, fmt.Errorf("unknown network recovery action %q", name)
	}
	endpoint := strings.TrimSpace(strings.TrimPrefix(name, endpointPrefix))
	ip := net.ParseIP(endpoint)
	if ip == nil {
		return Command{}, fmt.Errorf("invalid proxy endpoint recovery action %q", name)
	}
	metric := strconv.Itoa(40000 + sessionMetric(value.SessionID))
	if ip.To4() != nil {
		prefix := psQuote(ip.String() + "/32")
		return powershell(
			"Get-NetRoute -AddressFamily IPv4 -DestinationPrefix " + prefix +
				" -ErrorAction SilentlyContinue | Where-Object RouteMetric -eq " + metric +
				" | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0",
		), nil
	}
	prefix := psQuote(ip.String() + "/128")
	return powershell(
		"Get-NetRoute -AddressFamily IPv6 -DestinationPrefix " + prefix +
			" -ErrorAction SilentlyContinue | Where-Object RouteMetric -eq " + metric +
			" | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0",
	), nil
}

func (m *Manager) operations(sessionID string) []operation {
	adapter := psQuote(m.cfg.AdapterName)
	gateway4 := psQuote(m.cfg.TUNIPv4Gateway)
	tag := psQuote("Navo:TUN:" + sessionID)
	endpointMetric := 40000 + sessionMetric(sessionID)
	var result []operation

	// Host routes preserve the proxy transport path before split-default routes are installed.
	for _, endpoint := range m.cfg.ProxyEndpointIPs {
		name := "proxy endpoint bypass " + endpoint
		if strings.Contains(endpoint, ":") {
			prefix := psQuote(endpoint + "/128")
			result = append(result, operation{name: name,
				apply: powershell("$r=Find-NetRoute -RemoteIPAddress " + psQuote(endpoint) + "; if(!$r){throw 'no IPv6 route to proxy endpoint'}; New-NetRoute -DestinationPrefix " + prefix + " -InterfaceIndex $r.InterfaceIndex -NextHop $r.NextHop -RouteMetric " + strconv.Itoa(endpointMetric) + " -PolicyStore ActiveStore -ErrorAction Stop"),
				undo:  powershell("Get-NetRoute -AddressFamily IPv6 -DestinationPrefix " + prefix + " -ErrorAction SilentlyContinue | Where-Object RouteMetric -eq " + strconv.Itoa(endpointMetric) + " | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0"),
			})
		} else {
			prefix := psQuote(endpoint + "/32")
			result = append(result, operation{name: name,
				apply: powershell("$r=Find-NetRoute -RemoteIPAddress " + psQuote(endpoint) + "; if(!$r){throw 'no IPv4 route to proxy endpoint'}; New-NetRoute -DestinationPrefix " + prefix + " -InterfaceIndex $r.InterfaceIndex -NextHop $r.NextHop -RouteMetric " + strconv.Itoa(endpointMetric) + " -PolicyStore ActiveStore -ErrorAction Stop"),
				undo:  powershell("Get-NetRoute -AddressFamily IPv4 -DestinationPrefix " + prefix + " -ErrorAction SilentlyContinue | Where-Object RouteMetric -eq " + strconv.Itoa(endpointMetric) + " | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0"),
			})
		}
	}

	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		quotedPrefix := psQuote(prefix)
		result = append(result, operation{name: "IPv4 route " + prefix,
			apply: powershell("New-NetRoute -DestinationPrefix " + quotedPrefix + " -InterfaceAlias " + adapter + " -NextHop " + gateway4 + " -RouteMetric 1 -PolicyStore ActiveStore -ErrorAction Stop"),
			undo:  powershell("Get-NetRoute -DestinationPrefix " + quotedPrefix + " -InterfaceAlias " + adapter + " -ErrorAction SilentlyContinue | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0"),
		})
	}

	dns := make([]string, 0, len(m.cfg.DNSServers))
	for _, server := range m.cfg.DNSServers {
		dns = append(dns, psQuote(server))
	}
	result = append(result, operation{name: "DNS leak protection",
		apply: powershell("Add-DnsClientNrptRule -Namespace '.' -NameServers @(" + strings.Join(dns, ",") + ") -Comment " + tag + " -ErrorAction Stop"),
		undo:  powershell("Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object Comment -eq " + tag + " | Remove-DnsClientNrptRule -Force -ErrorAction SilentlyContinue; exit 0"),
	})

	switch m.cfg.IPv6Mode {
	case IPv6Tunnel:
		gateway6 := psQuote(m.cfg.TUNIPv6Gateway)
		for _, prefix := range []string{"::/1", "8000::/1"} {
			quotedPrefix := psQuote(prefix)
			result = append(result, operation{name: "IPv6 route " + prefix,
				apply: powershell("New-NetRoute -DestinationPrefix " + quotedPrefix + " -InterfaceAlias " + adapter + " -NextHop " + gateway6 + " -RouteMetric 1 -PolicyStore ActiveStore -ErrorAction Stop"),
				undo:  powershell("Get-NetRoute -DestinationPrefix " + quotedPrefix + " -InterfaceAlias " + adapter + " -ErrorAction SilentlyContinue | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue; exit 0"),
			})
		}
	case IPv6Block:
		ruleName := psQuote("Navo TUN IPv6 Block " + sessionID)
		// Windows Firewall rejects ::/0 on some Windows 11 builds. The two
		// disjoint halves cover the complete IPv6 address space equivalently.
		remoteIPv6 := "@('::/1','8000::/1')"
		result = append(result, operation{name: "IPv6 leak protection",
			apply: powershell("New-NetFirewallRule -DisplayName " + ruleName + " -Direction Outbound -Action Block -RemoteAddress " + remoteIPv6 + " -Profile Any -ErrorAction Stop | Out-Null"),
			undo:  ipv6BlockUndo(sessionID),
		})
	}
	return result
}

func ipv6BlockUndo(sessionID string) Command {
	ruleName := psQuote("Navo TUN IPv6 Block " + sessionID)
	return powershell("$rule=Get-NetFirewallRule -DisplayName " + ruleName + " -ErrorAction SilentlyContinue; if($rule){$rule | Remove-NetFirewallRule -ErrorAction SilentlyContinue}; exit 0")
}

func sessionMetric(sessionID string) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(sessionID))
	return int(hasher.Sum32() % 10000)
}

func powershell(script string) Command {
	return Command{Name: "powershell.exe", Args: []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}}
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
