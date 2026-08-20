package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"navo/internal/compiler"
	"navo/internal/coreadapter"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
	"navo/internal/domain/revision"
	"navo/internal/domain/selection"
	"navo/internal/domain/source"
	"navo/internal/fsatomic"
	"navo/internal/logstore"
	"navo/internal/network"
	"navo/internal/supervisor"
)

const (
	runtimeModeBypassMainland = "bypass_mainland"
	runtimeModeGlobal         = "global"
	runtimeModeDirect         = "direct"
	routingListModeOff        = "off"
	routingListModeBlacklist  = "blacklist"
	routingListModeWhitelist  = "whitelist"
	maxRoutingRuleEntries     = 256
)

var privateRouteCIDRs = []string{
	"10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
}

// These core-neutral suffixes keep every bundled core functional without
// depending on one core's private Geo database format.
var mainlandDirectDomainSuffixes = []string{
	"cn", "baidu.com", "qq.com", "wechat.com", "weixin.qq.com", "taobao.com",
	"tmall.com", "jd.com", "bilibili.com", "douyin.com", "zhihu.com", "163.com",
	"126.com", "sina.com.cn", "weibo.com", "alipay.com", "aliyun.com", "bytedance.com",
}

var whitelistDirectDomainSuffixes = []string{
	"baidu.com", "qq.com", "wechat.com", "weixin.qq.com", "taobao.com", "tmall.com",
	"jd.com", "bilibili.com", "douyin.com", "zhihu.com", "163.com", "126.com",
}

// openAIServiceDomainSuffixes is Navo's compatibility set for ChatGPT/Codex
// application flows. It is not presented as an official complete allowlist;
// users retain ownership of both editable routing lists.
var openAIServiceDomainSuffixes = []string{
	"auth.openai.com", "chatgpt.com", "ct.sendgrid.net", "intercom.io",
	"intercomcdn.com", "oaistatic.com", "oaiusercontent.com", "openai.com",
	"oaistatsig.com", "cdn.openaimerge.com", "cdn.workos.com",
	"challenges.cloudflare.com", "forwarder.workos.com", "humb.apple.com",
	"images.workoscdn.com", "js.stripe.com", "o207216.ingest.sentry.io",
	"o33249.ingest.sentry.io", "rum.browser-intake-datadoghq.com",
	"setup.workos.com", "workos.imgix.net",
}

var blacklistProxyDomainSuffixes = append([]string{
	"google.com", "googleapis.com", "gstatic.com", "youtube.com", "ytimg.com",
	"github.com", "githubassets.com", "githubusercontent.com", "twitter.com", "x.com",
	"facebook.com", "instagram.com", "wikipedia.org", "wikimedia.org", "reddit.com",
	"telegram.org", "t.me", "discord.com", "discordapp.com",
}, openAIServiceDomainSuffixes...)

type runtimeState struct {
	CoreID                 string   `json:"core_id"`
	SelectedOutbound       string   `json:"selected_outbound"`
	ActiveOutbound         string   `json:"active_outbound,omitempty"`
	CandidateOutbound      string   `json:"candidate_outbound,omitempty"`
	Mode                   string   `json:"mode"`
	RoutingModeConfigured  bool     `json:"routing_mode_configured"`
	RoutingRulesConfigured bool     `json:"routing_rules_configured"`
	RoutingListMode        string   `json:"routing_list_mode"`
	BlacklistRules         []string `json:"blacklist_rules"`
	WhitelistRules         []string `json:"whitelist_rules"`
	RevisionID             string   `json:"revision_id,omitempty"`
	ConfigHash             string   `json:"config_hash,omitempty"`
	RevisionStatus         string   `json:"revision_status,omitempty"`
	LastKnownGood          string   `json:"last_known_good_path,omitempty"`
	TUNEnabled             bool     `json:"-"`
	TUNName                string   `json:"tun_name,omitempty"`
	TUNMTU                 int      `json:"tun_mtu,omitempty"`
	TUNOutboundInterface   string   `json:"-"`
}

func loadRuntimeState(configDir string) runtimeState {
	state := runtimeState{
		CoreID: "sing-box", Mode: runtimeModeBypassMainland, TUNName: network.OwnedTUNAdapterName, TUNMTU: 1500,
		RoutingListMode: routingListModeOff,
		BlacklistRules:  cloneStrings(blacklistProxyDomainSuffixes),
		WhitelistRules:  cloneStrings(whitelistDirectDomainSuffixes),
	}
	if configDir == "" {
		return state
	}
	data, err := os.ReadFile(filepath.Join(configDir, "runtime_state.json"))
	if err == nil {
		_ = json.Unmarshal(data, &state)
	}
	// Older builds persisted "global" but discarded it on every start, so it
	// was not a user preference. Migrate that legacy state to smart routing;
	// explicit choices are preserved from this version onward.
	switch state.Mode {
	case "rule":
		state.Mode = runtimeModeBypassMainland
	case "blacklist":
		// Legacy blacklist mode used a direct default with blacklist proxy overrides.
		state.Mode = runtimeModeDirect
		state.RoutingListMode = routingListModeBlacklist
	case "whitelist":
		// Legacy whitelist mode used a proxy default with whitelist direct overrides.
		state.Mode = runtimeModeGlobal
		state.RoutingListMode = routingListModeWhitelist
	}
	if !state.RoutingModeConfigured || !validRuntimeMode(state.Mode) {
		state.Mode = runtimeModeBypassMainland
		state.RoutingModeConfigured = false
	}
	if !validRoutingListMode(state.RoutingListMode) {
		state.RoutingListMode = routingListModeOff
	}
	// List content is a durable preference, but activation is session-scoped.
	// A reboot or ordinary relaunch must never silently enable routing rules the
	// user did not click in the current session.
	state.RoutingListMode = routingListModeOff
	if !state.RoutingRulesConfigured {
		state.BlacklistRules = cloneStrings(blacklistProxyDomainSuffixes)
		state.WhitelistRules = cloneStrings(whitelistDirectDomainSuffixes)
	} else {
		blacklist, blacklistErr := normalizeRoutingRuleEntries(state.BlacklistRules)
		whitelist, whitelistErr := normalizeRoutingRuleEntries(state.WhitelistRules)
		if blacklistErr != nil || whitelistErr != nil {
			state.RoutingRulesConfigured = false
			state.BlacklistRules = cloneStrings(blacklistProxyDomainSuffixes)
			state.WhitelistRules = cloneStrings(whitelistDirectDomainSuffixes)
		} else {
			state.BlacklistRules = blacklist
			state.WhitelistRules = whitelist
		}
	}
	state.TUNEnabled = false
	// Adapter identity is not a user preference. Discard legacy or tampered
	// persisted names so recovery can never target Ethernet or Wi-Fi.
	state.TUNName = network.OwnedTUNAdapterName
	if state.TUNMTU <= 0 {
		state.TUNMTU = 1500
	}
	if state.CoreID == "" {
		state.CoreID = "sing-box"
	}
	// Migrate the pre-V1 runtime format without promoting an unverified
	// candidate. Only an explicitly active revision may seed ActiveOutbound.
	if state.ActiveOutbound == "" && state.RevisionStatus == "active" {
		state.ActiveOutbound = state.SelectedOutbound
	}
	if state.CandidateOutbound == "" &&
		state.RevisionStatus == "candidate" &&
		state.SelectedOutbound != state.ActiveOutbound {
		state.CandidateOutbound = state.SelectedOutbound
	}
	// Remove stale generated configs from previous runs.
	cleanupGeneratedRuntimeFiles(configDir, state.LastKnownGood)
	return state
}

func (s *Service) wintunAvailable() bool {
	_, err := os.Stat(
		filepath.Join(filepath.Dir(s.cfg.SingBoxPath), "wintun.dll"),
	)
	return err == nil
}

func (s *Service) setTUNRuntime(
	ctx context.Context,
	enabled bool,
	name string,
	mtu int,
) error {
	name, err := normalizeOwnedTUNName(name)
	if err != nil {
		return err
	}
	s.runtimeMu.Lock()
	previous := s.runtime
	s.runtime.TUNEnabled = enabled
	s.runtime.TUNName = name
	s.runtime.TUNMTU = mtu
	s.runtime.TUNOutboundInterface = ""
	s.runtimeMu.Unlock()

	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return fmt.Errorf("apply TUN configuration: %w", err)
	}
	s.tunPlanMu.Lock()
	s.tunPlan = network.TUNActivationPlan{}
	s.tunPlanMu.Unlock()
	return nil
}

func (s *Service) setSystemProxyRuntime(
	ctx context.Context,
	name string,
	mtu int,
) error {
	name, err := normalizeOwnedTUNName(name)
	if err != nil {
		return err
	}
	s.runtimeMu.Lock()
	previous := s.runtime
	s.runtime.TUNEnabled = false
	s.runtime.TUNName = name
	s.runtime.TUNMTU = mtu
	s.runtime.TUNOutboundInterface = ""
	selectedID := s.runtime.SelectedOutbound
	s.runtimeMu.Unlock()

	outbounds := s.currentOutbounds(ctx)
	outbounds, err = pinSystemProxyEndpoint(ctx, outbounds, selectedID)
	if err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return err
	}
	if err := s.applyRuntimeConfig(ctx, outbounds, "", ""); err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return fmt.Errorf("apply pinned system-proxy configuration: %w", err)
	}
	s.tunPlanMu.Lock()
	s.tunPlan = network.TUNActivationPlan{}
	s.tunPlanMu.Unlock()
	return nil
}

func pinSystemProxyEndpoint(
	ctx context.Context,
	outbounds []compiler.Outbound,
	selectedID string,
) ([]compiler.Outbound, error) {
	if selectedID == "" {
		return append([]compiler.Outbound(nil), outbounds...), nil
	}
	var selected *compiler.Outbound
	for index := range outbounds {
		if outbounds[index].ID == selectedID {
			selected = &outbounds[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("selected outbound %q was not found", selectedID)
	}
	host := strings.TrimSpace(selected.Server)
	if host == "" || selected.Type == compiler.OutboundDirect {
		return append([]compiler.Outbound(nil), outbounds...), nil
	}
	addresses, err := network.ResolveEndpointIPs(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("wait for proxy endpoint DNS readiness: %w", err)
	}
	pinnedIP := preferredEndpointIP(addresses)
	if pinnedIP == "" {
		return nil, fmt.Errorf("proxy endpoint %q resolved without a usable address", host)
	}
	return pinSelectedOutbound(outbounds, network.TUNActivationPlan{
		SelectedOutboundID: selectedID,
		OriginalServerHost: host,
		PinnedServerIP:     pinnedIP,
	})
}

func preferredEndpointIP(addresses []net.IP) string {
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	for _, address := range addresses {
		if ipv6 := address.To16(); ipv6 != nil {
			return ipv6.String()
		}
	}
	return ""
}

func (s *Service) resetTUNRuntimeAfterRollback() {
	s.runtimeMu.Lock()
	s.runtime.TUNEnabled = false
	s.runtime.TUNOutboundInterface = ""
	s.runtimeMu.Unlock()
	s.tunPlanMu.Lock()
	s.tunPlan = network.TUNActivationPlan{}
	s.tunPlanMu.Unlock()
}

func (s *Service) setTUNRuntimeWithPlan(
	ctx context.Context,
	enabled bool,
	name string,
	mtu int,
	plan network.TUNActivationPlan,
) error {
	name, err := normalizeOwnedTUNName(name)
	if err != nil {
		return err
	}
	s.runtimeMu.Lock()
	previous := s.runtime
	s.runtime.TUNEnabled = enabled
	s.runtime.TUNName = name
	s.runtime.TUNMTU = mtu
	s.runtime.TUNOutboundInterface = plan.PhysicalRoute.InterfaceAlias
	s.runtimeMu.Unlock()
	outbounds := s.currentOutbounds(ctx)
	outbounds, err = pinSelectedOutbound(outbounds, plan)
	if err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return err
	}
	if err := s.applyRuntimeConfig(ctx, outbounds, "", ""); err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return fmt.Errorf("apply pinned TUN configuration: %w", err)
	}
	s.tunPlanMu.Lock()
	s.tunPlan = plan
	s.tunPlanMu.Unlock()
	return nil
}

func normalizeOwnedTUNName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, network.OwnedTUNAdapterName) {
		return network.OwnedTUNAdapterName, nil
	}
	return "", fmt.Errorf("TUN adapter %q is outside Navo ownership", name)
}

func pinSelectedOutbound(outbounds []compiler.Outbound, plan network.TUNActivationPlan) ([]compiler.Outbound, error) {
	result := append([]compiler.Outbound(nil), outbounds...)
	if plan.OriginalServerHost == "" {
		return result, nil
	}
	for index := range result {
		if result[index].ID != plan.SelectedOutboundID {
			continue
		}
		if result[index].Server != plan.OriginalServerHost || plan.PinnedServerIP == "" {
			return nil, &network.TUNError{Code: network.ErrTUNEndpointPinFailed, Stage: network.TUNStageConfigCompiled, Resource: plan.SelectedOutboundID, Expected: plan.OriginalServerHost, Actual: result[index].Server}
		}
		if result[index].TLS && result[index].SNI == "" && net.ParseIP(plan.OriginalServerHost) == nil {
			// Preserve certificate verification when a hostname is replaced by its
			// cold-start IP pin.
			result[index].SNI = plan.OriginalServerHost
		}
		result[index].Server = plan.PinnedServerIP
		return result, nil
	}
	return nil, &network.TUNError{Code: network.ErrTUNEndpointPinFailed, Stage: network.TUNStageConfigCompiled, Resource: plan.SelectedOutboundID, Expected: plan.PinnedServerIP, Actual: "selected outbound missing"}
}

func validRuntimeMode(mode string) bool {
	switch mode {
	case runtimeModeBypassMainland, runtimeModeGlobal, runtimeModeDirect:
		return true
	default:
		return false
	}
}

func validRoutingListMode(mode string) bool {
	switch mode {
	case routingListModeOff, routingListModeBlacklist, routingListModeWhitelist:
		return true
	default:
		return false
	}
}

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}

func normalizeRoutingRuleEntries(values []string) ([]string, error) {
	if len(values) > maxRoutingRuleEntries {
		return nil, fmt.Errorf("routing list exceeds %d entries", maxRoutingRuleEntries)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q", raw)
			}
			value = network.String()
		} else {
			value = strings.TrimPrefix(strings.TrimPrefix(value, "*."), ".")
			if err := validateDomainSuffix(value); err != nil {
				return nil, fmt.Errorf("invalid domain suffix %q: %w", raw, err)
			}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func validateDomainSuffix(value string) error {
	if len(value) > 253 || !strings.Contains(value, ".") {
		return fmt.Errorf("must be a fully qualified domain suffix")
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("contains an invalid label")
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return fmt.Errorf("only ASCII letters, digits, and hyphens are supported")
			}
		}
	}
	return nil
}

func stringSlicePayload(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return []string{}, nil
	case []string:
		return cloneStrings(typed), nil
	case []interface{}:
		result := make([]string, 0, len(typed))
		for index, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("entry %d must be a string", index+1)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("routing list must be an array of strings")
	}
}

func (s *Service) handleOutboundSelect(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if strings.TrimSpace(id) == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("outbound id is required"))
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "SELECTION_BUSY", err)
	}
	defer s.captureMu.Unlock()
	wasRunning, err := s.waitForRuntimeCoreState(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", err)
	}
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	s.runtimeMu.Lock()
	previousID := activeOutboundID(s.runtime)
	tunEnabled := s.runtime.TUNEnabled
	s.runtimeMu.Unlock()
	if wasRunning && tunEnabled && previousID != id {
		return errorResponse(
			requestID,
			"CAPTURE_RESTART_REQUIRED",
			fmt.Errorf("stop TUN before changing its pinned outbound endpoint"),
		)
	}
	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), id, ""); err != nil {
		return errorResponse(requestID, "SELECTION_APPLY_FAILED", err)
	}
	if wasRunning {
		verification, verifyErr := s.verifyActiveRuntimeRouting(ctx)
		if verifyErr == nil {
			verifyErr = s.commitHealthyRuntime(ctx)
		}
		if verifyErr != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			rollbackErr := s.applyRuntimeConfig(
				rollbackCtx,
				s.currentOutbounds(rollbackCtx),
				previousID,
				"",
			)
			if rollbackErr == nil {
				rollbackErr = s.commitHealthyRuntime(rollbackCtx)
			}
			return errorResponse(
				requestID,
				"SELECTION_VERIFY_FAILED",
				errors.Join(verifyErr, rollbackErr),
			)
		}
		s.runtimeMu.Lock()
		activeID := activeOutboundID(s.runtime)
		candidateID := candidateOutboundID(s.runtime)
		s.runtimeMu.Unlock()
		return response(requestID, map[string]interface{}{
			"active_id": activeID, "candidate_id": candidateID,
			"verified": verification.Verified, "sites": verification.Sites,
		})
	}
	s.runtimeMu.Lock()
	activeID := activeOutboundID(s.runtime)
	candidateID := candidateOutboundID(s.runtime)
	s.runtimeMu.Unlock()
	return response(requestID, map[string]interface{}{
		"active_id": activeID, "candidate_id": candidateID,
	})
}

func (s *Service) handleRuntimeModeSet(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	mode, _ := msg["mode"].(string)
	if !validRuntimeMode(mode) {
		return errorResponse(requestID, "INVALID", fmt.Errorf("mode must be bypass_mainland, global, or direct"))
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "RUNTIME_BUSY", err)
	}
	defer s.captureMu.Unlock()

	s.runtimeMu.Lock()
	previousMode := s.runtime.Mode
	previousConfigured := s.runtime.RoutingModeConfigured
	s.runtimeMu.Unlock()
	wasRunning, err := s.waitForRuntimeCoreState(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", err)
	}
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	if wasRunning && s.sup.State() != supervisor.StateRunning {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", fmt.Errorf("proxy core left running state before routing update"))
	}
	outbounds, err := s.currentOutboundsWithTUNPin(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_MODE_APPLY_FAILED", err)
	}
	if err := s.applyRuntimeConfig(ctx, outbounds, "", mode); err != nil {
		return errorResponse(requestID, "RUNTIME_MODE_APPLY_FAILED", err)
	}
	verification := RuntimeRoutingVerification{}
	if wasRunning {
		verification, err = s.verifyActiveRuntimeRouting(ctx)
		if err == nil {
			err = s.commitHealthyRuntime(ctx)
		}
		if err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			rollbackOutbounds, rollbackErr := s.currentOutboundsWithTUNPin(rollbackCtx)
			if rollbackErr == nil {
				rollbackErr = s.applyRuntimeConfig(rollbackCtx, rollbackOutbounds, "", previousMode)
			}
			s.runtimeMu.Lock()
			s.runtime.RoutingModeConfigured = previousConfigured
			if saveErr := s.saveRuntimeStateLocked(s.cfg.ConfigDir); rollbackErr == nil {
				rollbackErr = saveErr
			}
			s.runtimeMu.Unlock()
			if rollbackErr == nil {
				rollbackErr = s.commitHealthyRuntime(rollbackCtx)
			}
			return errorResponse(requestID, "RUNTIME_MODE_VERIFY_FAILED", errors.Join(err, rollbackErr))
		}
	}
	return response(requestID, map[string]interface{}{
		"mode": mode, "verified": verification.Verified, "sites": verification.Sites,
	})
}

func (s *Service) handleRuntimeRulesSet(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	rawBlacklist, err := stringSlicePayload(msg["blacklist"])
	if err != nil {
		return errorResponse(requestID, "INVALID", fmt.Errorf("blacklist: %w", err))
	}
	rawWhitelist, err := stringSlicePayload(msg["whitelist"])
	if err != nil {
		return errorResponse(requestID, "INVALID", fmt.Errorf("whitelist: %w", err))
	}
	blacklist, err := normalizeRoutingRuleEntries(rawBlacklist)
	if err != nil {
		return errorResponse(requestID, "INVALID", fmt.Errorf("blacklist: %w", err))
	}
	whitelist, err := normalizeRoutingRuleEntries(rawWhitelist)
	if err != nil {
		return errorResponse(requestID, "INVALID", fmt.Errorf("whitelist: %w", err))
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "RUNTIME_BUSY", err)
	}
	defer s.captureMu.Unlock()
	wasRunning, err := s.waitForRuntimeCoreState(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", err)
	}
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	if wasRunning && s.sup.State() != supervisor.StateRunning {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", fmt.Errorf("proxy core left running state before routing-rule update"))
	}
	outbounds, err := s.currentOutboundsWithTUNPin(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_RULES_APPLY_FAILED", err)
	}
	s.runtimeMu.Lock()
	previousBlacklist := cloneStrings(s.runtime.BlacklistRules)
	previousWhitelist := cloneStrings(s.runtime.WhitelistRules)
	previousConfigured := s.runtime.RoutingRulesConfigured
	s.runtime.BlacklistRules = cloneStrings(blacklist)
	s.runtime.WhitelistRules = cloneStrings(whitelist)
	s.runtime.RoutingRulesConfigured = true
	s.runtimeMu.Unlock()
	restoreRules := func() {
		s.runtimeMu.Lock()
		s.runtime.BlacklistRules = cloneStrings(previousBlacklist)
		s.runtime.WhitelistRules = cloneStrings(previousWhitelist)
		s.runtime.RoutingRulesConfigured = previousConfigured
		s.runtimeMu.Unlock()
	}
	if err := s.applyRuntimeConfig(ctx, outbounds, "", ""); err != nil {
		restoreRules()
		return errorResponse(requestID, "RUNTIME_RULES_APPLY_FAILED", err)
	}
	verification := RuntimeRoutingVerification{}
	if wasRunning {
		verification, err = s.verifyActiveRuntimeRouting(ctx)
		if err == nil {
			err = s.commitHealthyRuntime(ctx)
		}
		if err != nil {
			restoreRules()
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			rollbackOutbounds, rollbackErr := s.currentOutboundsWithTUNPin(rollbackCtx)
			if rollbackErr == nil {
				rollbackErr = s.applyRuntimeConfig(rollbackCtx, rollbackOutbounds, "", "")
			}
			if rollbackErr == nil {
				rollbackErr = s.commitHealthyRuntime(rollbackCtx)
			}
			return errorResponse(requestID, "RUNTIME_RULES_VERIFY_FAILED", errors.Join(err, rollbackErr))
		}
	}
	return response(requestID, map[string]interface{}{
		"blacklist": blacklist, "whitelist": whitelist,
		"verified": verification.Verified, "sites": verification.Sites,
	})
}

func (s *Service) handleRuntimeListModeSet(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	mode, _ := msg["mode"].(string)
	if !validRoutingListMode(mode) {
		return errorResponse(requestID, "INVALID", fmt.Errorf("list mode must be off, blacklist, or whitelist"))
	}

	s.runtimeMu.Lock()
	currentMode := s.runtime.RoutingListMode
	s.runtimeMu.Unlock()
	if currentMode == mode {
		return response(requestID, map[string]interface{}{
			"mode": mode, "verified": true, "changed": false,
		})
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "RUNTIME_BUSY", err)
	}
	defer s.captureMu.Unlock()
	wasRunning, err := s.waitForRuntimeCoreState(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", err)
	}
	s.sup.SetRestartSuppressed(true)
	defer s.sup.SetRestartSuppressed(false)
	if wasRunning && s.sup.State() != supervisor.StateRunning {
		return errorResponse(requestID, "RUNTIME_CORE_NOT_READY", fmt.Errorf("proxy core left running state before list-mode update"))
	}
	outbounds, err := s.currentOutboundsWithTUNPin(ctx)
	if err != nil {
		return errorResponse(requestID, "RUNTIME_LIST_MODE_APPLY_FAILED", err)
	}
	s.runtimeMu.Lock()
	previousMode := s.runtime.RoutingListMode
	s.runtime.RoutingListMode = mode
	s.runtimeMu.Unlock()
	restoreMode := func() {
		s.runtimeMu.Lock()
		s.runtime.RoutingListMode = previousMode
		s.runtimeMu.Unlock()
	}
	if err := s.applyRuntimeConfig(ctx, outbounds, "", ""); err != nil {
		restoreMode()
		return errorResponse(requestID, "RUNTIME_LIST_MODE_APPLY_FAILED", err)
	}
	verification := RuntimeRoutingVerification{}
	if wasRunning {
		verification, err = s.verifyActiveRuntimeRouting(ctx)
		if err == nil {
			err = s.commitHealthyRuntime(ctx)
		}
		if err != nil {
			restoreMode()
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			rollbackOutbounds, rollbackErr := s.currentOutboundsWithTUNPin(rollbackCtx)
			if rollbackErr == nil {
				rollbackErr = s.applyRuntimeConfig(rollbackCtx, rollbackOutbounds, "", "")
			}
			if rollbackErr == nil {
				rollbackErr = s.commitHealthyRuntime(rollbackCtx)
			}
			return errorResponse(requestID, "RUNTIME_LIST_MODE_VERIFY_FAILED", errors.Join(err, rollbackErr))
		}
	}
	return response(requestID, map[string]interface{}{
		"mode": mode, "verified": verification.Verified, "sites": verification.Sites, "changed": true,
	})
}

func (s *Service) waitForRuntimeCoreState(ctx context.Context) (bool, error) {
	if s.sup == nil {
		return false, fmt.Errorf("proxy supervisor is unavailable")
	}
	initial := s.sup.State()
	if initial == supervisor.StateRunning {
		return true, nil
	}
	if initial == supervisor.StateStopped {
		return false, nil
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("wait for proxy core running from %s: %w", initial, ctx.Err())
		case <-ticker.C:
			if s.sup.State() == supervisor.StateRunning {
				return true, nil
			}
		}
	}
}

func (s *Service) verifyActiveRuntimeRouting(ctx context.Context) (RuntimeRoutingVerification, error) {
	s.runtimeMu.Lock()
	tunEnabled := s.runtime.TUNEnabled
	selectedID := s.runtime.SelectedOutbound
	runtimeMode := s.runtime.Mode
	s.runtimeMu.Unlock()
	if !tunEnabled {
		return verifyRuntimeRouting(ctx, s.cfg.ProxyPort, runtimeMode)
	}

	s.tunPlanMu.RLock()
	plan := s.tunPlan
	s.tunPlanMu.RUnlock()
	s.tunRuntimeMu.RLock()
	directIP := s.tunVerification.DirectIP
	adapter := s.tunAdapter
	s.tunRuntimeMu.RUnlock()
	if plan.SessionID == "" || directIP == "" {
		return RuntimeRoutingVerification{}, fmt.Errorf("active TUN verification baseline is unavailable")
	}

	var selected *compiler.Outbound
	for _, outbound := range s.currentOutbounds(ctx) {
		if outbound.ID == selectedID {
			copy := outbound
			selected = &copy
			break
		}
	}
	directMode := isDirectRuntime(runtimeMode, selectedID, selected)
	verification, err := s.tunVerifier.Verify(ctx, VerifyRequest{
		SessionID: plan.SessionID, DirectIP: directIP, DirectMode: directMode,
		ProxyPort: s.cfg.ProxyPort, TUNDNSIPv4: plan.TUNDNSIPv4,
		UDPRequired: directMode || outboundRequiresUDP(selected),
	})
	result := RuntimeRoutingVerification{Verified: err == nil, Sites: verification.Sites, VerifiedAt: verification.VerifiedAt}
	if err != nil {
		return result, err
	}
	s.setTUNRuntimeResult(network.TUNStageHealthCommitted, plan.SessionID, adapter, verification)
	return result, nil
}

func (s *Service) handleRuntimeVerify(ctx context.Context, requestID string) map[string]interface{} {
	verification, err := s.verifyActiveRuntimeRouting(ctx)
	if err != nil {
		return errorResponse(requestID, "APPLICATION_READINESS_FAILED", err)
	}
	s.runtimeMu.Lock()
	mode, listMode := s.runtime.Mode, s.runtime.RoutingListMode
	s.runtimeMu.Unlock()
	return response(requestID, map[string]interface{}{
		"verification": verification,
		"mode":         mode,
		"list_mode":    listMode,
	})
}

func (s *Service) handleRuntimeStatus(requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	mode := s.runtime.Mode
	selectedID := s.runtime.SelectedOutbound
	activeID := activeOutboundID(s.runtime)
	candidateID := candidateOutboundID(s.runtime)
	tunEnabled := s.runtime.TUNEnabled
	blacklist := cloneStrings(s.runtime.BlacklistRules)
	whitelist := cloneStrings(s.runtime.WhitelistRules)
	listMode := s.runtime.RoutingListMode
	s.runtimeMu.Unlock()
	s.diagnosticMu.RLock()
	exitIP := s.exitIP
	exitCountry := s.exitCountry
	s.diagnosticMu.RUnlock()
	return response(requestID, map[string]interface{}{
		"mode":         mode,
		"selected_id":  selectedID,
		"active_id":    activeID,
		"candidate_id": candidateID,
		"tun_enabled":  tunEnabled,
		"blacklist":    blacklist,
		"whitelist":    whitelist,
		"list_mode":    listMode,
		"exit_ip":      exitIP,
		"exit_country": exitCountry,
	})
}

func (s *Service) applyRuntimeConfig(
	ctx context.Context,
	outbounds []compiler.Outbound,
	selectedID string,
	mode string,
) error {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.applyRuntimeConfigLocked(ctx, outbounds, selectedID, mode)
}

func (s *Service) currentOutbounds(ctx context.Context) []compiler.Outbound {
	outbounds := s.subMgr.Outbounds()
	for _, upstream := range s.upstreamMgr.List() {
		outbound, err := s.compileUpstream(ctx, upstream)
		if err != nil {
			log.Printf("[service] upstream proxy %q unavailable: %v", upstream.ID, err)
			continue
		}
		outbounds = append(outbounds, outbound)
	}
	return outbounds
}

func (s *Service) currentOutboundsWithTUNPin(ctx context.Context) ([]compiler.Outbound, error) {
	outbounds := s.currentOutbounds(ctx)
	s.tunPlanMu.RLock()
	plan := s.tunPlan
	s.tunPlanMu.RUnlock()
	if plan.SelectedOutboundID == "" || plan.OriginalServerHost == "" {
		return outbounds, nil
	}
	pinned, err := pinSelectedOutbound(outbounds, plan)
	if err != nil {
		return nil, fmt.Errorf("preserve active TUN endpoint pin: %w", err)
	}
	return pinned, nil
}

func (s *Service) compileUpstream(
	ctx context.Context,
	upstream endpoint.UpstreamProxy,
) (compiler.Outbound, error) {
	if err := upstream.Validate(); err != nil {
		return compiler.Outbound{}, err
	}
	outboundType := compiler.OutboundSOCKS
	if upstream.Protocol == endpoint.UpstreamHTTP || upstream.Protocol == endpoint.UpstreamHTTPS {
		outboundType = compiler.OutboundHTTP
	}
	resolve := func(reference *string) (string, error) {
		if reference == nil {
			return "", nil
		}
		value, err := s.credentialStore.Resolve(ctx, *reference)
		if err != nil {
			return "", err
		}
		defer clear(value)
		return string(value), nil
	}
	username, err := resolve(upstream.UsernameRef)
	if err != nil {
		return compiler.Outbound{}, fmt.Errorf("resolve username credential: %w", err)
	}
	password, err := resolve(upstream.PasswordRef)
	if err != nil {
		return compiler.Outbound{}, fmt.Errorf("resolve password credential: %w", err)
	}
	return compiler.Outbound{
		ID: upstream.ID, Name: upstream.Name, Type: outboundType,
		Server: upstream.Server, Port: int(upstream.Port),
		Username: username, Password: password, TLS: upstream.TLS,
		Enabled: upstream.Enabled, ProviderID: "upstream_proxy",
		CreatedAt: upstream.CreatedAt, UpdatedAt: upstream.UpdatedAt,
	}, nil
}

func (s *Service) applyRuntimeConfigLocked(
	ctx context.Context,
	outbounds []compiler.Outbound,
	selectedID string,
	mode string,
) error {
	next := s.runtime
	if mode != "" {
		next.Mode = mode
		next.RoutingModeConfigured = true
	}
	if !validRuntimeMode(next.Mode) {
		next.Mode = runtimeModeBypassMainland
		next.RoutingModeConfigured = false
	}
	if selectedID != "" {
		next.SelectedOutbound = selectedID
	}

	enabled := make([]compiler.Outbound, 0, len(outbounds)+1)
	enabled = append(enabled, compiler.Outbound{
		ID:      "direct",
		Name:    "Direct",
		Type:    compiler.OutboundDirect,
		Enabled: true,
	})
	selectedFound := false
	var selectedOutbound *compiler.Outbound
	for _, outbound := range outbounds {
		if !outbound.Enabled {
			continue
		}
		enabled = append(enabled, outbound)
		if outbound.ID == next.SelectedOutbound {
			selectedFound = true
			copy := outbound
			selectedOutbound = &copy
		}
	}
	if !selectedFound {
		if selectedID != "" {
			return fmt.Errorf("outbound %q was not found", selectedID)
		}
		if len(enabled) > 1 {
			next.SelectedOutbound = enabled[1].ID
			copy := enabled[1]
			selectedOutbound = &copy
		} else {
			next.SelectedOutbound = ""
		}
	}

	selectedTag := next.SelectedOutbound
	if selectedTag == "" {
		selectedTag = "direct"
	}
	finalOutbound, rules := runtimeRoutingPolicy(
		next.Mode, next.RoutingListMode, selectedTag, next.BlacklistRules, next.WhitelistRules,
	)

	configDir := s.cfg.ConfigDir
	if configDir == "" {
		configDir = filepath.Dir(s.cfg.ConfigPath)
	}
	inbounds := []compiler.InboundConfig{{
		Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1",
		ListenPort: s.cfg.ProxyPort, Sniff: true,
	}}
	var tunConfig *compiler.TUNConfig
	if next.TUNEnabled {
		inbounds = append(inbounds, compiler.InboundConfig{
			Type: "tun", Tag: "tun-in", Sniff: true,
		})
		tunConfig = &compiler.TUNConfig{
			Enabled: true, InterfaceName: next.TUNName, MTU: next.TUNMTU,
			Address:           []string{"172.19.0.1/30"},
			OutboundInterface: next.TUNOutboundInterface,
			// The capture coordinator installs owned routes only after the core
			// and adapter pass readiness checks.
			AutoRoute: false, StrictRoute: false,
		}
	}
	outboundInterface := next.TUNOutboundInterface
	if !next.TUNEnabled {
		physicalRoute, routeErr := network.FindPhysicalRoute(ctx, "1.1.1.1", next.TUNName)
		if routeErr == nil {
			outboundInterface = physicalRoute.InterfaceAlias
			next.TUNOutboundInterface = outboundInterface
		} else {
			// Runtime verification remains the fail-closed boundary. Keeping this
			// discovery best-effort lets capture-off recovery and tests proceed on
			// restricted Windows identities where route CIM reads are denied.
			log.Printf("[service] physical interface discovery unavailable: %v", routeErr)
		}
	}
	config := &compiler.Config{
		SchemaVersion: 1,
		Log: compiler.LogConfig{
			Level: "info", Output: filepath.Join(configDir, "sing-box.log"),
			Timestamp: true,
		},
		Inbounds:          inbounds,
		Outbounds:         enabled,
		RoutingRules:      rules,
		FinalOutbound:     finalOutbound,
		OutboundInterface: outboundInterface,
		TUN:               tunConfig,
	}
	if tunConfig != nil && selectedTag != "direct" && finalOutbound == "direct" {
		// Blacklist mode still needs trustworthy resolution for proxied targets.
		// Route read-only DoH through the selected node while mutations remain
		// single-shot and owned by the capture transaction.
		config.DNS = proxiedRuntimeDNS(selectedTag)
	}
	for _, outbound := range enabled {
		if !compiler.Compatible(next.CoreID, outbound) {
			return fmt.Errorf("core %s does not support %s outbound %q", next.CoreID, outbound.Type, outbound.Name)
		}
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	runtimeID := time.Now().UnixNano()
	coreType := core.Type(next.CoreID)
	adapter, err := s.coreAdapters.Get(coreType)
	if err != nil {
		return err
	}
	activeSelection, disconnected := runtimeSelection(next, selectedOutbound)
	compiled, err := adapter.Compile(ctx, coreadapter.CompileRequest{
		Selection: activeSelection, Config: config,
		PortPlan:   coreadapter.PortPlan{MixedPort: s.cfg.ProxyPort},
		RuntimeDir: configDir, RevisionID: fmt.Sprintf("runtime.%d", runtimeID),
		Disconnected: disconnected,
	})
	if err != nil {
		return fmt.Errorf("generate runtime config: %w", err)
	}
	candidatePath := compiled.MainConfigPath
	if err := s.host.ValidateConfig(ctx, candidatePath); err != nil {
		_ = os.Remove(candidatePath)
		return fmt.Errorf("validate runtime config: %w", err)
	}
	candidateRevision := runtimeRevision(compiled, activeSelection, disconnected)
	if s.revisionRepo != nil {
		if err := s.revisionRepo.SaveCandidate(ctx, candidateRevision); err != nil {
			_ = os.Remove(candidatePath)
			return fmt.Errorf("persist candidate revision: %w", err)
		}
	}

	previousPath := s.cfg.ConfigPath
	previousRuntime := s.runtime
	wasRunning := s.sup.State() == supervisor.StateRunning
	if wasRunning {
		if err := s.sup.SwapConfig(ctx, candidatePath); err != nil {
			// Supervisor owns replacement rollback and reports only after the old
			// configuration is restored or restoration itself has failed.
			log.Printf("[service] swap config failed: %v", err)
			_ = os.Remove(candidatePath)
			if s.revisionRepo != nil {
				markCtx, markCancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = s.revisionRepo.MarkFailed(markCtx, compiled.RevisionID, "startup")
				markCancel()
			}
			return fmt.Errorf("activate candidate revision: %w", err)
		}
	}
	var reboundAdapter network.AdapterSnapshot
	if wasRunning && next.TUNEnabled && s.networkManager != nil {
		var err error
		reboundAdapter, err = s.networkManager.Rebind(ctx)
		if err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			var rollbackErr error
			if previousPath != "" {
				rollbackErr = s.sup.SwapConfig(rollbackCtx, previousPath)
				if rollbackErr == nil {
					_, rollbackErr = s.networkManager.Rebind(rollbackCtx)
				}
			}
			_ = os.Remove(candidatePath)
			return fmt.Errorf("rebind TUN network ownership after core reload: %w", errors.Join(err, rollbackErr))
		}
	}
	next.RevisionID = compiled.RevisionID
	next.ConfigHash = compiled.ContentHash
	next.RevisionStatus = "candidate"
	next.ActiveOutbound = activeOutboundID(previousRuntime)
	next.CandidateOutbound = ""
	if next.SelectedOutbound != next.ActiveOutbound {
		next.CandidateOutbound = next.SelectedOutbound
	}
	next.LastKnownGood = previousRuntime.LastKnownGood
	s.cfg.ConfigPath = candidatePath
	s.runtime = next
	if reboundAdapter.InterfaceGUID != "" || reboundAdapter.InterfaceIndex != 0 {
		s.tunRuntimeMu.Lock()
		s.tunAdapter = reboundAdapter
		s.tunRuntimeMu.Unlock()
	}
	s.ipDetector.ClearCache()
	s.directIPDetector.ClearCache()
	rollback := func(cause error) error {
		rollbackSucceeded := !wasRunning || previousPath == ""
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rollbackCancel()
		var rollbackErr error
		if wasRunning && previousPath != "" {
			rollbackErr = s.sup.SwapConfig(rollbackCtx, previousPath)
			if rollbackErr == nil && previousRuntime.TUNEnabled && s.networkManager != nil {
				var adapter network.AdapterSnapshot
				adapter, rollbackErr = s.networkManager.Rebind(rollbackCtx)
				if rollbackErr == nil {
					s.tunRuntimeMu.Lock()
					s.tunAdapter = adapter
					s.tunRuntimeMu.Unlock()
				}
			}
			rollbackSucceeded = rollbackErr == nil
		}
		if rollbackSucceeded {
			s.cfg.ConfigPath = previousPath
			s.runtime = previousRuntime
			_ = os.Remove(candidatePath)
		}
		if rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("restore previous runtime: %w", rollbackErr))
		}
		return cause
	}
	if err := s.saveRuntimeStateLocked(configDir); err != nil {
		return rollback(err)
	}
	if wasRunning {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath, previousPath, previousRuntime.LastKnownGood)
	} else if previousRuntime.RevisionStatus == "active" {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath, previousPath)
	} else {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath)
	}
	return nil
}

func runtimeRoutingPolicy(
	mode, listMode, selectedTag string,
	blacklist, whitelist []string,
) (string, []compiler.RoutingRule) {
	if selectedTag == "" {
		selectedTag = "direct"
	}

	rules := []compiler.RoutingRule{routingRule(
		"private-networks", "Private networks", compiler.RuleIP,
		privateRouteCIDRs, "direct",
	)}
	if listMode == routingListModeWhitelist {
		rules = appendRuntimeListRules(rules, "direct-whitelist", "Direct whitelist", whitelist, "direct")
	}
	if listMode == routingListModeBlacklist && selectedTag != "direct" {
		rules = appendRuntimeListRules(rules, "proxy-blacklist", "Proxy blacklist", blacklist, selectedTag)
	}
	switch mode {
	case runtimeModeDirect:
		return "direct", rules
	case runtimeModeGlobal:
		return selectedTag, rules
	default:
		rules = append(rules, routingRule(
			"mainland-domains", "Mainland domains", compiler.RuleDomainSuffix,
			mainlandDirectDomainSuffixes, "direct",
		))
		return selectedTag, rules
	}
}

func appendRuntimeListRules(
	rules []compiler.RoutingRule,
	id, name string,
	entries []string,
	outbound string,
) []compiler.RoutingRule {
	domains := make([]string, 0, len(entries))
	cidrs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(entry, "/") {
			cidrs = append(cidrs, entry)
		} else {
			domains = append(domains, entry)
		}
	}
	if len(domains) > 0 {
		rules = append(rules, routingRule(id, name, compiler.RuleDomainSuffix, domains, outbound))
	}
	if len(cidrs) > 0 {
		rules = append(rules, routingRule(id+"-ip", name+" IP", compiler.RuleIP, cidrs, outbound))
	}
	return rules
}

func routingRule(id, name string, ruleType compiler.RuleType, values []string, outbound string) compiler.RoutingRule {
	return compiler.RoutingRule{
		ID: id, Name: name, RuleType: ruleType,
		Values:     append([]string(nil), values...),
		OutboundID: outbound, OutboundTag: outbound, Enabled: true,
	}
}

func proxiedRuntimeDNS(outbound string) *compiler.DNSConfig {
	return &compiler.DNSConfig{
		Enabled: true, Strategy: compiler.DNSStrategyIPv4Only,
		Servers: []compiler.DNSServer{{
			Type: "https", Tag: "dns-proxy", Server: "1.1.1.1", ServerPort: 443,
			Path: "/dns-query", TLSServerName: "cloudflare-dns.com", Detour: outbound,
		}},
		Final: "dns-proxy",
	}
}

func runtimeSelection(
	state runtimeState,
	selected *compiler.Outbound,
) (selection.ActiveSelection, bool) {
	if selected == nil {
		return selection.ActiveSelection{}, true
	}
	result := selection.ActiveSelection{
		CoreType: core.Type(state.CoreID), CaptureMode: capture.ModeSystemProxy,
		UpdatedAt: time.Now().UTC(),
	}
	if state.TUNEnabled {
		result.CaptureMode = capture.ModeTUN
	}
	if selected.ProviderID == "upstream_proxy" {
		result.SourceType = source.TypeUpstreamProxy
		id := selected.ID
		result.UpstreamProxyID = &id
		return result, false
	}
	result.SourceType = source.TypeAirportSubscription
	subscriptionID := selected.ProviderID
	if subscriptionID == "" {
		subscriptionID = "legacy"
	}
	endpointID := selected.ID
	result.SubscriptionID = &subscriptionID
	result.EndpointID = &endpointID
	return result, false
}

func runtimeRevision(
	compiled coreadapter.CompiledConfig,
	activeSelection selection.ActiveSelection,
	disconnected bool,
) revision.Revision {
	sourceType := activeSelection.SourceType
	endpointReference := "disconnected"
	if !disconnected {
		switch activeSelection.SourceType {
		case source.TypeUpstreamProxy:
			if activeSelection.UpstreamProxyID != nil {
				endpointReference = *activeSelection.UpstreamProxyID
			}
		case source.TypeAirportSubscription:
			if activeSelection.EndpointID != nil {
				endpointReference = *activeSelection.EndpointID
			}
		}
	} else {
		sourceType = source.TypeAirportSubscription
	}
	return revision.Revision{
		ID: compiled.RevisionID, CoreType: compiled.CoreType,
		SourceType: sourceType, EndpointReference: endpointReference,
		ConfigHash: compiled.ContentHash, ConfigPath: compiled.MainConfigPath,
		CreatedAt: time.Now().UTC(), ValidationStatus: revision.StatusActive,
		StartupStatus: revision.StatusCandidate, HealthStatus: revision.StatusCandidate,
	}
}

func (s *Service) commitHealthyRuntime(ctx context.Context) error {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.commitHealthyRuntimeLocked(ctx)
}

func activeOutboundID(state runtimeState) string {
	if state.ActiveOutbound != "" {
		return state.ActiveOutbound
	}
	// Empty status is the pre-V1 persisted/in-memory format and represents the
	// last selected runtime. Only an explicit candidate is uncommitted.
	if state.RevisionStatus != "candidate" {
		return state.SelectedOutbound
	}
	return ""
}

func candidateOutboundID(state runtimeState) string {
	if state.CandidateOutbound != "" && state.CandidateOutbound != activeOutboundID(state) {
		return state.CandidateOutbound
	}
	if state.RevisionStatus == "candidate" && state.SelectedOutbound != activeOutboundID(state) {
		return state.SelectedOutbound
	}
	return ""
}

func (s *Service) commitHealthyRuntimeLocked(ctx context.Context) error {
	var selected *compiler.Outbound
	for _, outbound := range s.currentOutbounds(ctx) {
		if outbound.ID == s.runtime.SelectedOutbound {
			copy := outbound
			selected = &copy
			break
		}
	}
	activeSelection, disconnected := runtimeSelection(s.runtime, selected)
	if !disconnected && s.selectionRepo != nil {
		if err := s.selectionRepo.Save(ctx, activeSelection); err != nil {
			return fmt.Errorf("commit active selection: %w", err)
		}
	}
	if s.revisionRepo != nil {
		if err := s.revisionRepo.MarkActive(ctx, s.runtime.RevisionID); err != nil {
			return fmt.Errorf("commit active revision: %w", err)
		}
	}
	previous := s.runtime
	s.runtime.RevisionStatus = "active"
	s.runtime.ActiveOutbound = s.runtime.SelectedOutbound
	s.runtime.CandidateOutbound = ""
	s.runtime.LastKnownGood = s.cfg.ConfigPath
	if err := s.saveRuntimeStateLocked(s.cfg.ConfigDir); err != nil {
		s.runtime = previous
		return err
	}
	cleanupGeneratedRuntimeFiles(s.cfg.ConfigDir, s.cfg.ConfigPath)
	return nil
}

func cleanupGeneratedRuntimeFiles(configDir string, retainedPaths ...string) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return
	}
	retained := make(map[string]struct{}, len(retainedPaths))
	for _, path := range retainedPaths {
		if strings.TrimSpace(path) != "" {
			retained[filepath.Base(path)] = struct{}{}
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, keep := retained[name]; keep {
			continue
		}
		if name == "runtime.active.json" {
			_ = os.Remove(filepath.Join(configDir, name))
			continue
		}
		if !strings.HasPrefix(name, "runtime.") ||
			(!strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		nanos := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(name, "runtime."), ".json"), ".yaml")
		if _, err := strconv.ParseInt(nanos, 10, 64); err == nil {
			_ = os.Remove(filepath.Join(configDir, name))
		}
	}
}

func (s *Service) saveRuntimeStateLocked(configDir string) error {
	data, err := json.MarshalIndent(s.runtime, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state: %w", err)
	}
	finalPath := filepath.Join(configDir, "runtime_state.json")
	if err := fsatomic.WriteFile(finalPath, data, 0600); err != nil {
		return fmt.Errorf("activate runtime state: %w", err)
	}
	return nil
}

func (s *Service) handleOutboundCreate(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	name, _ := msg["name"].(string)
	typ, _ := msg["proto"].(string)
	if typ == "" {
		typ, _ = msg["type"].(string)
	}
	server, _ := msg["server"].(string)
	port := 0
	switch v := msg["port"].(type) {
	case float64:
		port = int(v)
	case int:
		port = v
	}
	if name == "" || typ == "" || server == "" || port == 0 {
		return errorResponse(requestID, "INVALID", fmt.Errorf("name, protocol, server and port are required"))
	}
	protocol := endpoint.UpstreamProtocol(strings.ToLower(typ))
	if protocol == "socks" {
		protocol = endpoint.UpstreamSOCKS5
	}
	if protocol != endpoint.UpstreamHTTP && protocol != endpoint.UpstreamHTTPS && protocol != endpoint.UpstreamSOCKS5 {
		return errorResponse(requestID, "INVALID", fmt.Errorf("upstream protocol must be http, https or socks5"))
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "OUTBOUND_BUSY", err)
	}
	defer s.captureMu.Unlock()
	createdRefs := make([]string, 0, 2)
	storeCredential := func(value string) (*string, error) {
		if value == "" {
			return nil, nil
		}
		reference, err := s.credentialStore.Put(ctx, []byte(value))
		if err != nil {
			return nil, err
		}
		createdRefs = append(createdRefs, reference)
		return &reference, nil
	}
	usernameRef, err := storeCredential(strField(msg, "username"))
	if err != nil {
		return errorResponse(requestID, "CREDENTIAL_STORE_FAILED", err)
	}
	passwordRef, err := storeCredential(strField(msg, "password"))
	if err != nil {
		for _, reference := range createdRefs {
			_ = s.credentialStore.Delete(ctx, reference)
		}
		return errorResponse(requestID, "CREDENTIAL_STORE_FAILED", err)
	}
	udpPolicy := endpoint.UDPPolicyDisabled
	if value := strField(msg, "udp_policy"); value != "" {
		udpPolicy = endpoint.UDPPolicy(value)
	}
	now := time.Now().UTC()
	upstream := endpoint.UpstreamProxy{
		ID: msgID(msg, name), Name: name, Protocol: protocol,
		Server: server, Port: uint16(port), UsernameRef: usernameRef,
		PasswordRef: passwordRef, TLS: protocol == endpoint.UpstreamHTTPS,
		UDPPolicy: udpPolicy, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.upstreamMgr.Add(upstream); err != nil {
		for _, reference := range createdRefs {
			_ = s.credentialStore.Delete(ctx, reference)
		}
		return errorResponse(requestID, "UPSTREAM_PROXY_CREATE_FAILED", err)
	}
	if err := s.applyRuntimeConfig(
		ctx,
		s.currentOutbounds(ctx),
		upstream.ID,
		"",
	); err != nil {
		_, _, _ = s.upstreamMgr.Remove(upstream.ID)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, reference := range createdRefs {
			_ = s.credentialStore.Delete(cleanupCtx, reference)
		}
		return errorResponse(requestID, "OUTBOUND_APPLY_FAILED", err)
	}
	return response(requestID, map[string]interface{}{
		"id": upstream.ID, "source_type": "upstream_proxy", "status": "created",
	})
}

func (s *Service) handleOutboundDelete(parent context.Context, requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if id == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("id required"))
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := s.lockCapture(ctx); err != nil {
		return errorResponse(requestID, "OUTBOUND_BUSY", err)
	}
	defer s.captureMu.Unlock()
	removed, found, err := s.upstreamMgr.Remove(id)
	if err != nil {
		return errorResponse(requestID, "UPSTREAM_PROXY_DELETE_FAILED", err)
	}
	if !found {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("outbound %q not found", id))
	}
	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		if restoreErr := s.upstreamMgr.Add(removed); restoreErr != nil {
			return errorResponse(requestID, "OUTBOUND_ROLLBACK_FAILED", errors.Join(err, restoreErr))
		}
		return errorResponse(requestID, "OUTBOUND_APPLY_FAILED", err)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()
	for _, reference := range []*string{removed.UsernameRef, removed.PasswordRef} {
		if reference != nil {
			_ = s.credentialStore.Delete(cleanupCtx, *reference)
		}
	}
	return response(requestID, map[string]interface{}{"status": "deleted"})
}

func strField(msg map[string]interface{}, key string) string {
	s, _ := msg[key].(string)
	return s
}

func boolField(msg map[string]interface{}, key string) bool {
	b, _ := msg[key].(bool)
	return b
}

func msgID(msg map[string]interface{}, name string) string {
	if id, _ := msg["id"].(string); id != "" {
		return id
	}
	return sanitizeID(name) + "-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Service) handleOutboundUpdate(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if id == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("id required"))
	}
	outs := s.currentOutbounds(context.Background())
	found := false
	for i, o := range outs {
		if o.ID == id {
			if v, ok := msg["name"].(string); ok && v != "" {
				outs[i].Name = v
			}
			if v, ok := msg["server"].(string); ok && v != "" {
				outs[i].Server = v
			}
			if v, ok := msg["port"].(float64); ok {
				outs[i].Port = int(v)
			}
			if v, ok := msg["username"].(string); ok {
				outs[i].Username = v
			}
			if v, ok := msg["password"].(string); ok {
				outs[i].Password = v
			}
			// Extended fields for outbound.update
			if v, ok := msg["type"].(string); ok && v != "" {
				outs[i].Type = compiler.OutboundType(v)
			} else if v, ok := msg["proto"].(string); ok && v != "" {
				outs[i].Type = compiler.OutboundType(v)
			}
			if v, ok := msg["method"].(string); ok {
				outs[i].Method = v
			} else if v, ok := msg["cipher"].(string); ok {
				outs[i].Method = v
			}
			if v, ok := msg["uuid"].(string); ok {
				outs[i].UUID = v
			}
			if v, ok := msg["password2"].(string); ok {
				outs[i].Password2 = v
			}
			if v, ok := msg["network"].(string); ok {
				outs[i].Network = v
			}
			if v, ok := msg["tls"].(bool); ok {
				outs[i].TLS = v
			}
			if v, ok := msg["sni"].(string); ok {
				outs[i].SNI = v
			}
			if v, ok := msg["transport_host"].(string); ok {
				outs[i].TransportHost = v
			}
			if v, ok := msg["transport_path"].(string); ok {
				outs[i].TransportPath = v
			}
			if v, ok := msg["security"].(string); ok {
				outs[i].Security = v
			}
			if v, ok := msg["fingerprint"].(string); ok {
				outs[i].Fingerprint = v
			}
			if v, ok := msg["reality_public_key"].(string); ok {
				outs[i].RealityPublicKey = v
			}
			if v, ok := msg["reality_short_id"].(string); ok {
				outs[i].RealityShortID = v
			}
			if v, ok := msg["obfs_type"].(string); ok {
				outs[i].ObfsType = v
			}
			if v, ok := msg["obfs_password"].(string); ok {
				outs[i].ObfsPassword = v
			}
			if v, ok := msg["congestion_control"].(string); ok {
				outs[i].CongestionControl = v
			}
			outs[i].UpdatedAt = time.Now()
			found = true
			break
		}
	}
	if !found {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("outbound %q not found", id))
	}
	return errorResponse(requestID, "UNSUPPORTED", fmt.Errorf("use the typed upstream proxy update API"))
}

func (s *Service) handleOutboundTest(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if id == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("id required"))
	}
	outs := s.currentOutbounds(context.Background())
	var target *compiler.Outbound
	for i := range outs {
		if outs[i].ID == id {
			target = &outs[i]
			break
		}
	}
	if target == nil {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("outbound %q not found", id))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe := s.prober.ProbeTCP(ctx, target.ID, target.Server, target.Port)
	s.recordEndpointProbe(target.ID, probe.Healthy, probe.Error, probe.Latency)
	return response(requestID, map[string]interface{}{
		"id": id, "reachable": probe.Healthy,
		"latency_ms": probe.Latency.Milliseconds(), "error": probe.Error,
	})
}

func (s *Service) handleOutboundTestAll(requestID string) map[string]interface{} {
	outs := s.currentOutbounds(context.Background())
	results := make([]map[string]interface{}, 0, len(outs))
	for i := range outs {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		probe := s.prober.ProbeTCP(ctx, outs[i].ID, outs[i].Server, outs[i].Port)
		cancel()
		s.recordEndpointProbe(outs[i].ID, probe.Healthy, probe.Error, probe.Latency)
		results = append(results, map[string]interface{}{
			"id": outs[i].ID, "reachable": probe.Healthy,
			"latency_ms": probe.Latency.Milliseconds(), "error": probe.Error,
		})
	}
	return response(requestID, map[string]interface{}{"results": results})
}

func sanitizeID(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		} else if r == ' ' || r == '_' {
			result += "-"
		}
	}
	if result == "" {
		return "node"
	}
	return result
}

func errorResponse(requestID, code string, err error) map[string]interface{} {
	_ = logstore.Emit(logstore.LevelError, "Service", "IPC", "request failed", map[string]any{
		"request_id": requestID, "error_code": code,
	})
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "ERROR",
		"timestamp":  time.Now().UnixMilli(),
		"payload": map[string]interface{}{
			"code": code, "message": err.Error(),
		},
	}
}
