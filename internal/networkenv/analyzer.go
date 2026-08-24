package networkenv

import (
	"sort"
	"strings"
)

// Analyze derives stable findings from one immutable collection of facts. It
// deliberately has no repair callback: a Snapshot is evidence, not authority.
func Analyze(snapshot Snapshot) Snapshot {
	if snapshot.Version == 0 {
		snapshot.Version = SnapshotVersion
	}
	snapshot.Findings = nil
	if snapshot.TUN.Expected || snapshot.Capture.CommittedMode == "tun" ||
		(snapshot.Capture.State == "faulted" && snapshot.Capture.DesiredMode == "tun") {
		snapshot.TUN.Expected = true
	}
	state := analyzerState{snapshot: &snapshot, seen: make(map[string]struct{})}
	state.analyzePhysical()
	state.analyzeSystemProxy()
	state.analyzeTUN()
	state.analyzeOwnedResources()
	state.analyzeCapture()
	state.analyzePartial()
	sort.SliceStable(snapshot.Findings, func(i, j int) bool {
		if snapshot.Findings[i].Severity != snapshot.Findings[j].Severity {
			return severityRank(snapshot.Findings[i].Severity) > severityRank(snapshot.Findings[j].Severity)
		}
		return snapshot.Findings[i].Code < snapshot.Findings[j].Code
	})
	snapshot.Health = deriveHealth(snapshot)
	if snapshot.Findings == nil {
		snapshot.Findings = []Finding{}
	}
	return snapshot
}

type analyzerState struct {
	snapshot *Snapshot
	seen     map[string]struct{}
}

func (a *analyzerState) add(finding Finding) {
	if _, exists := a.seen[finding.Code]; exists {
		return
	}
	if a.transitional() && (finding.Recoverable || finding.Severity == SeverityError) {
		finding.Transitional = true
		finding.Recoverable = false
	}
	a.seen[finding.Code] = struct{}{}
	a.snapshot.Findings = append(a.snapshot.Findings, finding)
}

func (a *analyzerState) transitional() bool {
	transition := a.snapshot.Transition
	if !transition.Busy {
		return false
	}
	switch strings.ToLower(transition.Phase) {
	case "applying", "committing", "rolling_back":
		return true
	}
	switch strings.ToLower(transition.Operation) {
	case "recovery", "self_heal":
		return true
	default:
		return false
	}
}

func (a *analyzerState) analyzePhysical() {
	physical := a.snapshot.Physical
	if physical.Known && !physical.Available {
		a.add(Finding{
			Code: FindingPhysicalNetworkUnavailable, Severity: SeverityError,
			Domain: "physical_network", Summary: "物理网络不可用",
			Detail: physical.LastError, Ownership: OwnerNone,
		})
	}
}

func (a *analyzerState) analyzeSystemProxy() {
	proxy := a.snapshot.SystemProxy
	expected := a.snapshot.Capture.CommittedMode == "system_proxy" ||
		(a.snapshot.Capture.State == "faulted" && a.snapshot.Capture.DesiredMode == "system_proxy")
	if proxy.Enabled && !proxy.OwnedByNavo {
		if proxy.OwnershipMarker || proxy.OwnershipLost || expected {
			a.add(Finding{
				Code: FindingSystemProxyStaleNavo, Severity: SeverityError,
				Domain: "system_proxy", Summary: "System Proxy 已由外部配置接管",
				Detail: "Navo 不会覆盖当前外部代理设置", Ownership: OwnerExternal,
			})
		} else {
			a.add(Finding{
				Code: FindingSystemProxyExternal, Severity: SeverityInfo,
				Domain: "system_proxy", Summary: "检测到外部 System Proxy",
				Detail: proxy.ProxyServer, Ownership: OwnerExternal,
			})
		}
	}
	if !proxy.Enabled && (proxy.OwnershipMarker || expected) {
		a.add(Finding{
			Code: FindingSystemProxyStaleNavo, Severity: SeverityError,
			Domain: "system_proxy", Summary: "Navo System Proxy 状态不一致",
			Detail:    "当前 WinINet 状态与 Navo ownership 不匹配",
			Ownership: OwnerNavo, Recoverable: true,
		})
	}
	if proxy.OwnedByNavo && proxy.ReachableKnown && !proxy.Reachable {
		a.add(Finding{
			Code: FindingSystemProxyEndpointUnreachable, Severity: SeverityError,
			Domain: "system_proxy", Summary: "Navo 本地代理端点不可达",
			Detail: proxy.ProxyServer, Ownership: OwnerNavo, Recoverable: true,
		})
	}
}

func (a *analyzerState) analyzeTUN() {
	tun := a.snapshot.TUN
	if tun.ExternalPresent {
		a.add(Finding{
			Code: FindingExternalTUNPresent, Severity: SeverityInfo,
			Domain: "tun", Summary: "检测到外部虚拟网卡",
			Detail:    "Navo 不会修改或禁用外部虚拟网卡",
			Ownership: OwnerExternal,
		})
	}
	if !tun.Expected {
		return
	}
	if !tun.Navo.Present {
		a.add(Finding{
			Code: FindingNavoTUNMissing, Severity: SeverityError,
			Domain: "tun", Summary: "Navo TUN 网卡缺失",
			Detail: tun.Navo.LastError, Ownership: OwnerNavo, Recoverable: true,
		})
		return
	}
	if !tun.Navo.Enabled {
		ownership := tun.Navo.Ownership
		recoverable := ownership == OwnerNavo
		if ownership == "" {
			ownership = OwnerUnknown
		}
		a.add(Finding{
			Code: FindingNavoTUNDisabled, Severity: SeverityError,
			Domain: "tun", Summary: "Navo TUN 网卡未启用",
			Detail: tun.Navo.LastError, Ownership: ownership, Recoverable: recoverable,
		})
	}
}

func (a *analyzerState) analyzeOwnedResources() {
	journal := a.snapshot.Journal
	if journal.Dirty || journal.PendingActions > 0 {
		ownership := OwnerUnknown
		recoverable := false
		if journal.OwnedResources > 0 && journal.ConflictingResources == 0 {
			ownership, recoverable = OwnerNavo, true
		}
		a.add(Finding{
			Code: FindingNetworkJournalPending, Severity: SeverityError,
			Domain: "route", Summary: "Navo 网络事务尚未完整恢复",
			Detail: journal.LastError, Ownership: ownership, Recoverable: recoverable,
		})
	}
	if journal.Present && journal.OwnedResources > 0 && a.snapshot.Capture.CommittedMode != "tun" {
		recoverable := journal.ConflictingResources == 0
		ownership := OwnerNavo
		if !recoverable {
			ownership = OwnerUnknown
		}
		a.add(Finding{
			Code: FindingNavoRouteResidual, Severity: SeverityError,
			Domain: "route", Summary: "检测到 Navo 网络残留",
			Detail:    "TUN 已停止，但 V2 Journal 仍记录 Navo-owned 资源",
			Ownership: ownership, Recoverable: recoverable,
		})
	}
	a.resourceFinding(FindingNavoRouteResidual, "route", "Navo 路由状态不一致", a.snapshot.Routes)
	a.resourceFinding(FindingNavoDNSInconsistent, "dns", "Navo DNS 状态不一致", a.snapshot.DNS)
	a.resourceFinding(FindingNavoNRPTInconsistent, "nrpt", "Navo NRPT 状态不一致", a.snapshot.NRPT)
	a.resourceFinding(FindingNavoFirewallInconsistent, "firewall", "Navo 防火墙状态不一致", a.snapshot.Firewall)
}

func (a *analyzerState) resourceFinding(code, domain, summary string, resource ResourceSnapshot) {
	if !resource.Known || resource.Coherent {
		return
	}
	ownership := OwnerNavo
	recoverable := resource.OwnedCount > 0 && resource.ConflictCount == 0
	if resource.ConflictCount > 0 || resource.OwnedCount == 0 {
		ownership = OwnerUnknown
	}
	a.add(Finding{
		Code: code, Severity: SeverityError, Domain: domain, Summary: summary,
		Detail: resource.LastError, Ownership: ownership, Recoverable: recoverable,
	})
}

func (a *analyzerState) analyzeCapture() {
	capture := a.snapshot.Capture
	if capture.CommittedMode == "off" || capture.ReadinessState != "failed" {
		return
	}
	a.add(Finding{
		Code: FindingCaptureDataPlaneFailed, Severity: SeverityError,
		Domain: "node", Summary: "Navo 数据面验证失败",
		Detail: capture.ReadinessError, Ownership: OwnerNavo, Recoverable: true,
	})
}

func (a *analyzerState) analyzePartial() {
	if a.snapshot.SystemProxy.LastError != "" || a.snapshot.Physical.LastError != "" ||
		len(a.snapshot.ObservationErrors) > 0 {
		a.snapshot.Partial = true
	}
	if !a.snapshot.Partial {
		return
	}
	detail := strings.Join(a.snapshot.ObservationErrors, "; ")
	a.add(Finding{
		Code: FindingObservationPartial, Severity: SeverityWarn,
		Domain: "detection", Summary: "部分网络状态暂时无法读取",
		Detail: detail, Ownership: OwnerUnknown,
	})
}

func deriveHealth(snapshot Snapshot) HealthState {
	if snapshot.Transition.Busy && analyzerTransition(snapshot.Transition) {
		return HealthChecking
	}
	physicalDown := false
	warn, failure := false, false
	for _, finding := range snapshot.Findings {
		if finding.Code == FindingPhysicalNetworkUnavailable {
			physicalDown = true
		}
		switch finding.Severity {
		case SeverityError:
			failure = true
		case SeverityWarn:
			warn = true
		}
	}
	if physicalDown {
		return HealthUnavailable
	}
	if failure {
		if snapshot.Capture.CommittedMode != "off" || snapshot.TUN.Expected {
			return HealthUnavailable
		}
		return HealthDegraded
	}
	if warn || snapshot.Partial {
		return HealthDegraded
	}
	if snapshot.Physical.Known {
		if snapshot.Physical.Available {
			return HealthHealthy
		}
		return HealthUnavailable
	}
	return HealthUnknown
}

func analyzerTransition(transition TransitionSnapshot) bool {
	phase := strings.ToLower(transition.Phase)
	operation := strings.ToLower(transition.Operation)
	return phase == "applying" || phase == "committing" || phase == "rolling_back" ||
		operation == "recovery" || operation == "self_heal"
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 3
	case SeverityWarn:
		return 2
	default:
		return 1
	}
}
