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

const networkRollbackTimeout = 30 * time.Second

type operation struct {
	name     string
	code     string
	resource journalResource
	inspect  Command
	apply    Command
}

// Manager is the single owner of Navo-created TUN network resources.
type Manager struct {
	mu       sync.Mutex
	cfg      Config
	executor Executor
	platform Platform
	active   bool
	adapter  AdapterSnapshot
}

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

func (m *Manager) Preflight(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}
	if err := m.platform.Preflight(ctx, m.cfg); err != nil {
		return &TUNError{Code: ErrTUNPreflightFailed, Stage: TUNStagePreflight, Cause: err}
	}
	return nil
}

// Activate waits for a fully configured adapter, writes a V2 resource journal,
// applies resources idempotently, then hard-verifies the Windows control plane.
func (m *Manager) Activate(ctx context.Context) (AdapterSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activateLocked(ctx, true)
}

// Rebind refreshes every adapter-bound resource after a live core reload.
// Wintun may recreate the named adapter with a different interface identity,
// so keeping routes from the previous process can silently blackhole apps.
func (m *Manager) Rebind(ctx context.Context) (AdapterSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return AdapterSnapshot{}, nil
	}
	if err := m.recoverNetworkState(ctx); err != nil {
		return AdapterSnapshot{}, &TUNError{Code: ErrTUNRecoveryDirty, Stage: TUNStagePreflight, RollbackStatus: "dirty", Cause: err}
	}
	m.active = false
	m.adapter = AdapterSnapshot{}
	return m.activateLocked(ctx, false)
}

func (m *Manager) activateLocked(ctx context.Context, recoverFirst bool) (AdapterSnapshot, error) {
	if !m.cfg.Enabled {
		return AdapterSnapshot{}, nil
	}
	if m.active {
		return m.adapter, nil
	}
	if m.cfg.ActivationPlan.SessionID == "" {
		return AdapterSnapshot{}, &TUNError{Code: ErrTUNPreflightFailed, Stage: TUNStagePreflight, Cause: fmt.Errorf("activation plan is required")}
	}
	if recoverFirst {
		if err := m.recoverNetworkState(ctx); err != nil {
			return AdapterSnapshot{}, &TUNError{Code: ErrTUNRecoveryDirty, Stage: TUNStagePreflight, RollbackStatus: "dirty", Cause: err}
		}
	}
	plan := m.cfg.ActivationPlan
	adapter, err := m.platform.WaitForAdapterReady(ctx, plan.AdapterName, plan.TUNIPv4Address, plan.MTU, m.cfg.AdapterTimeout)
	if err != nil {
		return AdapterSnapshot{}, err
	}
	m.reportStage(TUNStageAdapterReady)

	value := &journal{
		Version: 2, AdapterName: plan.AdapterName, SessionID: plan.SessionID,
		CreatedAt: time.Now().UTC(), Plan: plan, Adapter: adapter,
	}
	if err := writeJournal(m.cfg.JournalPath, value); err != nil {
		return AdapterSnapshot{}, err
	}
	for _, op := range m.operations(plan, adapter) {
		state, inspectErr := m.inspectOperation(ctx, op)
		if inspectErr != nil {
			return AdapterSnapshot{}, m.activationFailure(op, inspectErr, value)
		}
		action := journalAction{Name: op.name, Status: actionApplied, Resource: op.resource}
		if state == "EXACT" {
			action.Resource.CreatedByNavo = false
			value.Actions = append(value.Actions, action)
			if err := writeJournal(m.cfg.JournalPath, value); err != nil {
				return AdapterSnapshot{}, m.activationFailure(op, err, value)
			}
			if m.shouldInjectFailure(op) {
				return AdapterSnapshot{}, m.activationFailure(op, fmt.Errorf("injected failure at %s", m.cfg.FailurePoint), value)
			}
			m.injectCrash(op)
			continue
		}
		action.Status = actionPending
		action.Resource.CreatedByNavo = true
		value.Actions = append(value.Actions, action)
		if err := writeJournal(m.cfg.JournalPath, value); err != nil {
			return AdapterSnapshot{}, m.activationFailure(op, err, value)
		}
		if err := m.executor.Run(ctx, op.apply); err != nil {
			return AdapterSnapshot{}, m.activationFailure(op, err, value)
		}
		value.Actions[len(value.Actions)-1].Status = actionApplied
		if err := writeJournal(m.cfg.JournalPath, value); err != nil {
			return AdapterSnapshot{}, m.activationFailure(op, err, value)
		}
		if m.shouldInjectFailure(op) {
			return AdapterSnapshot{}, m.activationFailure(op, fmt.Errorf("injected failure at %s", m.cfg.FailurePoint), value)
		}
		m.injectCrash(op)
	}
	m.reportStage(TUNStageNetworkApplied)
	if err := m.platform.VerifyControlPlane(ctx, plan, adapter); err != nil {
		return AdapterSnapshot{}, m.activationFailure(operation{name: "control-plane verification", code: ErrTUNPublicRouteNotCaptured}, err, value)
	}
	m.reportStage(TUNStageControlPlaneVerified)
	m.active = true
	m.adapter = adapter
	return adapter, nil
}

func (m *Manager) injectCrash(op operation) {
	if m.cfg.CrashFn == nil || m.cfg.CrashPoint == "" {
		return
	}
	failurePoint := m.cfg.FailurePoint
	m.cfg.FailurePoint = m.cfg.CrashPoint
	matched := m.shouldInjectFailure(op)
	m.cfg.FailurePoint = failurePoint
	if matched {
		m.cfg.CrashFn()
	}
}

func (m *Manager) shouldInjectFailure(op operation) bool {
	switch m.cfg.FailurePoint {
	case "after-endpoint-bypass":
		return op.resource.Kind == resourceEndpointRoute
	case "after-first-split-route":
		return op.resource.Kind == resourceSplitRoute && op.resource.DestinationPrefix == "0.0.0.0/1"
	case "after-second-split-route":
		return op.resource.Kind == resourceSplitRoute && op.resource.DestinationPrefix == "128.0.0.0/1"
	case "after-nrpt":
		return op.resource.Kind == resourceNRPTRule
	case "after-ipv6":
		return op.resource.Kind == resourceFirewallRule
	default:
		return false
	}
}

func (m *Manager) reportStage(stage TUNStage) {
	if m.cfg.StageFn != nil {
		m.cfg.StageFn(stage)
	}
}

func (m *Manager) inspectOperation(ctx context.Context, op operation) (string, error) {
	output, err := m.executor.RunOutput(ctx, op.inspect)
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(output)
	if state != "EXACT" && state != "MISSING" {
		return "", fmt.Errorf("unexpected resource inspection state %q", state)
	}
	return state, nil
}

func (m *Manager) activationFailure(op operation, cause error, value *journal) error {
	rollbackErr := m.rollbackAfterFailure(value)
	code := op.code
	if code == "" {
		code = ErrTUNRollbackFailed
	}
	if original := asTUNError(cause); original != nil {
		code = original.Code
	}
	status := "complete"
	if rollbackErr != nil {
		status = "dirty"
	}
	return &TUNError{
		Code: code, Stage: operationStage(op.resource.Kind), Resource: op.name,
		RollbackStatus: status, Cause: errors.Join(cause, rollbackErr),
	}
}

func operationStage(kind journalResourceKind) TUNStage {
	if kind == "" {
		return TUNStageControlPlaneVerified
	}
	return TUNStageNetworkApplied
}

// Deactivate deliberately ignores a possibly canceled request context. Network
// rollback always owns its own bounded lifetime.
func (m *Manager) Deactivate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	err := m.recoverNetworkState(ctx)
	if err == nil {
		m.active = false
		m.adapter = AdapterSnapshot{}
	}
	return err
}

func (m *Manager) Recover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	return m.recoverNetworkState(ctx)
}

func (m *Manager) recoverNetworkState(ctx context.Context) error {
	if err := m.recoverLocked(); err != nil {
		return err
	}
	if err := m.cleanupOrphanedFirewallRules(ctx); err != nil {
		return &TUNError{
			Code: ErrTUNRecoveryDirty, Stage: TUNStagePreflight,
			Resource: "orphaned_ipv6_firewall", RollbackStatus: "dirty", Cause: err,
		}
	}
	return nil
}

func (m *Manager) cleanupOrphanedFirewallRules(ctx context.Context) error {
	inspect := "$marker='NAVO_ORPHAN_FIREWALL_SCAN';" +
		"$all=@(Get-NetFirewallRule -ErrorAction SilentlyContinue|Where-Object {[string]$_.DisplayName -like 'Navo TUN IPv6 Block *'});" +
		"foreach($rule in $all){if([string]$rule.DisplayName -notmatch '^Navo TUN IPv6 Block [0-9a-z-]{1,64}$' -or [string]$rule.Direction -ne 'Outbound' -or [string]$rule.Action -ne 'Block'){throw 'orphaned Navo firewall identity conflict'};" +
		"$remote=@($rule|Get-NetFirewallAddressFilter -ErrorAction Stop|ForEach-Object {$_.RemoteAddress});if($remote.Count -ne 2 -or $remote -notcontains '::/1' -or $remote -notcontains '8000::/1'){throw 'orphaned Navo firewall address conflict'}};" +
		"if($all.Count -gt 0){'ORPHANED'}else{'CLEAN'}"
	state, err := m.executor.RunOutput(ctx, powershell(inspect))
	if err != nil {
		return fmt.Errorf("inspect orphaned Navo firewall rules: %w", err)
	}
	switch strings.TrimSpace(state) {
	case "CLEAN":
		return nil
	case "ORPHANED":
		remove := "$all=@(Get-NetFirewallRule -ErrorAction SilentlyContinue|Where-Object {[string]$_.DisplayName -match '^Navo TUN IPv6 Block [0-9a-z-]{1,64}$' -and [string]$_.Direction -eq 'Outbound' -and [string]$_.Action -eq 'Block'});" +
			"if($all.Count -gt 0){$all|Remove-NetFirewallRule -ErrorAction Stop};exit 0"
		if err := m.executor.Run(ctx, powershell(remove)); err != nil {
			return fmt.Errorf("remove orphaned Navo firewall rules: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unexpected orphaned firewall inspection state %q", state)
	}
}

func (m *Manager) recoverLocked() error {
	value, err := readJournal(m.cfg.JournalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if value.Version == 1 {
		if err := m.migrateV1(value); err != nil {
			return err
		}
	}
	return m.rollbackAfterFailure(value)
}

func (m *Manager) rollbackAfterFailure(value *journal) error {
	ctx, cancel := context.WithTimeout(context.Background(), networkRollbackTimeout)
	defer cancel()
	if err := m.rollbackLocked(ctx, value); err != nil {
		return &TUNError{Code: ErrTUNRollbackFailed, Stage: TUNStageNetworkApplied, RollbackStatus: "dirty", Cause: err}
	}
	return nil
}

func (m *Manager) rollbackLocked(ctx context.Context, value *journal) error {
	if err := m.validateRecoveryJournal(value); err != nil {
		return err
	}
	var rollbackErr error
	for index := len(value.Actions) - 1; index >= 0; index-- {
		action := value.Actions[index]
		if action.Resource.CreatedByNavo {
			undo, err := m.safeUndoForResource(action.Resource)
			if err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("undo %s: %w", action.Name, err))
				continue
			}
			if err := m.executor.Run(ctx, undo); err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("undo %s: %w", action.Name, err))
				continue
			}
		}
		value.Actions = append(value.Actions[:index], value.Actions[index+1:]...)
		if len(value.Actions) > 0 {
			if err := writeJournal(m.cfg.JournalPath, value); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
				break
			}
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
	if value == nil || value.Version != 2 {
		return fmt.Errorf("invalid network recovery journal")
	}
	if value.AdapterName == "" || value.SessionID == "" || len(value.SessionID) > 64 {
		return fmt.Errorf("network recovery journal has an invalid identity")
	}
	for _, char := range value.SessionID {
		if (char < '0' || char > '9') && (char < 'a' || char > 'z') && char != '-' {
			return fmt.Errorf("network recovery journal has an invalid session id")
		}
	}
	if !adapterNamePattern.MatchString(value.AdapterName) {
		return fmt.Errorf("network recovery journal has an unsafe adapter")
	}
	for _, action := range value.Actions {
		if err := validateJournalResource(action.Resource); err != nil {
			return fmt.Errorf("unsafe journal resource %q: %w", action.Name, err)
		}
	}
	return nil
}

func validateJournalResource(resource journalResource) error {
	switch resource.Kind {
	case resourceEndpointRoute, resourceSplitRoute:
		if _, _, err := net.ParseCIDR(resource.DestinationPrefix); err != nil {
			return fmt.Errorf("invalid route prefix")
		}
		if net.ParseIP(resource.NextHop) == nil || resource.RouteMetric < 0 {
			return fmt.Errorf("invalid route identity")
		}
		if resource.InterfaceIndex == 0 && !adapterNamePattern.MatchString(resource.InterfaceAlias) {
			return fmt.Errorf("route has no stable interface identity")
		}
	case resourceNRPTRule:
		if resource.NRPTNamespace != "." || !strings.HasPrefix(resource.NRPTComment, "Navo:TUN:") {
			return fmt.Errorf("invalid NRPT identity")
		}
	case resourceFirewallRule:
		if !strings.HasPrefix(resource.FirewallDisplayName, "Navo TUN IPv6 Block ") {
			return fmt.Errorf("invalid firewall identity")
		}
	default:
		return fmt.Errorf("unknown resource kind %q", resource.Kind)
	}
	return nil
}

func (m *Manager) migrateV1(value *journal) error {
	for index := range value.Actions {
		action := &value.Actions[index]
		switch action.Name {
		case "IPv4 route 0.0.0.0/1", "IPv4 route 128.0.0.0/1":
			action.Resource = journalResource{Kind: resourceSplitRoute, DestinationPrefix: strings.TrimPrefix(action.Name, "IPv4 route "), AddressFamily: "IPv4", InterfaceAlias: value.AdapterName, NextHop: m.cfg.TUNIPv4Peer, RouteMetric: 1, CreatedByNavo: true}
		case "DNS leak protection":
			action.Resource = journalResource{Kind: resourceNRPTRule, NRPTNamespace: ".", NRPTComment: "Navo:TUN:" + value.SessionID, NameServers: []string{m.cfg.TUNDNSIPv4}, CreatedByNavo: true}
		case "IPv6 leak protection":
			action.Resource = journalResource{Kind: resourceFirewallRule, FirewallDisplayName: "Navo TUN IPv6 Block " + value.SessionID, CreatedByNavo: true}
		default:
			if strings.HasPrefix(action.Name, "proxy endpoint bypass ") {
				return fmt.Errorf("legacy endpoint route lacks a stable interface identity; journal retained")
			}
			return fmt.Errorf("unknown legacy recovery action %q", action.Name)
		}
	}
	value.Version = 2
	return writeJournal(m.cfg.JournalPath, value)
}

func (m *Manager) safeUndoForResource(resource journalResource) (Command, error) {
	if err := validateJournalResource(resource); err != nil {
		return Command{}, err
	}
	switch resource.Kind {
	case resourceEndpointRoute, resourceSplitRoute:
		selector := routeSelector(resource)
		script := "$all=@(Get-NetRoute -AddressFamily " + resource.AddressFamily + " -DestinationPrefix " + psQuote(resource.DestinationPrefix) + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue);" +
			"$exact=@($all|Where-Object {" + selector + "});if($exact.Count -gt 1){throw 'duplicate owned route'};if($all.Count -gt 0 -and $exact.Count -eq 0){throw 'owned route identity conflict'};" +
			"if($exact.Count -eq 1){$exact[0]|Remove-NetRoute -Confirm:$false -ErrorAction Stop};exit 0"
		return powershell(script), nil
	case resourceNRPTRule:
		script := "$all=@(Get-DnsClientNrptRule -ErrorAction SilentlyContinue|Where-Object {[string]$_.Comment -eq " + psQuote(resource.NRPTComment) + "});" +
			"$exact=@($all|Where-Object {(@($_.Namespace) -contains '.')});if($all.Count -ne $exact.Count){throw 'owned NRPT identity conflict'};if($exact.Count -gt 0){$exact|Remove-DnsClientNrptRule -Force -ErrorAction Stop};exit 0"
		return powershell(script), nil
	case resourceFirewallRule:
		script := "$all=@(Get-NetFirewallRule -DisplayName " + psQuote(resource.FirewallDisplayName) + " -ErrorAction SilentlyContinue);$exact=@($all|Where-Object {[string]$_.Direction -eq 'Outbound' -and [string]$_.Action -eq 'Block'});if($all.Count -ne $exact.Count -or $exact.Count -gt 1){throw 'owned firewall identity conflict'};if($exact.Count -eq 1){$exact[0]|Remove-NetFirewallRule -ErrorAction Stop};exit 0"
		return powershell(script), nil
	default:
		return Command{}, fmt.Errorf("unknown resource kind %q", resource.Kind)
	}
}

func (m *Manager) operations(plan TUNActivationPlan, adapter AdapterSnapshot) []operation {
	result := make([]operation, 0, len(plan.EndpointRoutes)+4)
	endpointMetric := 40000 + sessionMetric(plan.SessionID)
	for _, endpoint := range plan.EndpointRoutes {
		prefix := endpoint.EndpointIP + "/32"
		family := "IPv4"
		if strings.Contains(endpoint.EndpointIP, ":") {
			prefix, family = endpoint.EndpointIP+"/128", "IPv6"
		}
		resource := journalResource{Kind: resourceEndpointRoute, DestinationPrefix: prefix, AddressFamily: family, InterfaceIndex: endpoint.InterfaceIndex, InterfaceGUID: endpoint.InterfaceGUID, InterfaceAlias: endpoint.InterfaceAlias, NextHop: endpoint.NextHop, RouteMetric: endpointMetric}
		result = append(result, routeOperation("proxy endpoint bypass "+endpoint.EndpointIP, ErrTUNEndpointBypassFailed, resource, true))
	}
	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		resource := journalResource{Kind: resourceSplitRoute, DestinationPrefix: prefix, AddressFamily: "IPv4", InterfaceIndex: adapter.InterfaceIndex, InterfaceGUID: adapter.InterfaceGUID, InterfaceAlias: adapter.Name, NextHop: plan.TUNIPv4Peer, RouteMetric: 1}
		result = append(result, routeOperation("IPv4 route "+prefix, ErrTUNSplitRouteFailed, resource, false))
	}
	tag := "Navo:TUN:" + plan.SessionID
	nrpt := journalResource{Kind: resourceNRPTRule, NRPTNamespace: ".", NRPTComment: tag, NameServers: []string{plan.TUNDNSIPv4}}
	nrptInspect := "$all=@(Get-DnsClientNrptRule -ErrorAction SilentlyContinue|Where-Object {(@($_.Namespace) -contains '.')});$exact=@($all|Where-Object {[string]$_.Comment -eq " + psQuote(tag) + " -and (@($_.NameServers) -contains " + psQuote(plan.TUNDNSIPv4) + ")});if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT';exit 0};if($all.Count -gt 0){throw 'conflicting NRPT namespace'};'MISSING'"
	result = append(result, operation{name: "DNS leak protection", code: ErrTUNNRPTFailed, resource: nrpt, inspect: powershell(nrptInspect), apply: powershell("Add-DnsClientNrptRule -Namespace '.' -NameServers @(" + psQuote(plan.TUNDNSIPv4) + ") -Comment " + psQuote(tag) + " -ErrorAction Stop|Out-Null")})
	if plan.IPv6Mode == IPv6Block {
		name := "Navo TUN IPv6 Block " + plan.SessionID
		resource := journalResource{Kind: resourceFirewallRule, FirewallDisplayName: name}
		inspect := "$all=@(Get-NetFirewallRule -DisplayName " + psQuote(name) + " -ErrorAction SilentlyContinue);$exact=@($all|Where-Object {[string]$_.Enabled -eq 'True' -and [string]$_.Direction -eq 'Outbound' -and [string]$_.Action -eq 'Block'});if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT';exit 0};if($all.Count -gt 0){throw 'conflicting firewall rule'};'MISSING'"
		apply := "New-NetFirewallRule -DisplayName " + psQuote(name) + " -Direction Outbound -Action Block -RemoteAddress @('::/1','8000::/1') -Profile Any -ErrorAction Stop|Out-Null"
		result = append(result, operation{name: "IPv6 leak protection", code: ErrTUNIPv6PolicyFailed, resource: resource, inspect: powershell(inspect), apply: powershell(apply)})
	}
	return result
}

func routeOperation(name, code string, resource journalResource, validatePhysical bool) operation {
	validation := ""
	if validatePhysical {
		validation = "$adapter=Get-NetAdapter -InterfaceIndex " + strconv.FormatUint(uint64(resource.InterfaceIndex), 10) + " -ErrorAction Stop;if([string]$adapter.InterfaceGuid -ne " + psQuote(resource.InterfaceGUID) + " -or [string]$adapter.Status -ne 'Up'){throw 'frozen physical interface changed'};"
	}
	selector := routeSelector(resource)
	inspect := validation + "$all=@(Get-NetRoute -AddressFamily " + resource.AddressFamily + " -DestinationPrefix " + psQuote(resource.DestinationPrefix) + " -PolicyStore ActiveStore -ErrorAction SilentlyContinue);$exact=@($all|Where-Object {" + selector + "});if($exact.Count -eq 1 -and $all.Count -eq 1){'EXACT';exit 0};if($all.Count -gt 0){throw 'conflicting route'};'MISSING'"
	apply := "New-NetRoute -DestinationPrefix " + psQuote(resource.DestinationPrefix) + " -InterfaceIndex " + strconv.FormatUint(uint64(resource.InterfaceIndex), 10) + " -NextHop " + psQuote(resource.NextHop) + " -RouteMetric " + strconv.Itoa(resource.RouteMetric) + " -PolicyStore ActiveStore -ErrorAction Stop|Out-Null"
	return operation{name: name, code: code, resource: resource, inspect: powershell(inspect), apply: powershell(apply)}
}

func routeSelector(resource journalResource) string {
	selector := "[string]$_.NextHop -eq " + psQuote(resource.NextHop) + " -and [int]$_.RouteMetric -eq " + strconv.Itoa(resource.RouteMetric)
	if resource.InterfaceIndex > 0 {
		selector += " -and [uint32]$_.InterfaceIndex -eq " + strconv.FormatUint(uint64(resource.InterfaceIndex), 10)
	} else {
		selector += " -and [string]$_.InterfaceAlias -eq " + psQuote(resource.InterfaceAlias)
	}
	return selector
}

func sessionMetric(sessionID string) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(sessionID))
	return int(hasher.Sum32() % 10000)
}

func powershell(script string) Command {
	encoding := "[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new();$OutputEncoding=[System.Text.UTF8Encoding]::new();"
	return Command{Name: "powershell.exe", Args: []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", encoding + script}}
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
