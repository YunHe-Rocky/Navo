package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"navo/internal/supervisor"
)

const (
	runtimeModeRule   = "rule"
	runtimeModeGlobal = "global"
	runtimeModeDirect = "direct"
)

type runtimeState struct {
	CoreID           string `json:"core_id"`
	SelectedOutbound string `json:"selected_outbound"`
	Mode             string `json:"mode"`
	RevisionID       string `json:"revision_id,omitempty"`
	ConfigHash       string `json:"config_hash,omitempty"`
	RevisionStatus   string `json:"revision_status,omitempty"`
	LastKnownGood    string `json:"last_known_good_path,omitempty"`
	TUNEnabled       bool   `json:"-"`
	TUNName          string `json:"tun_name,omitempty"`
	TUNMTU           int    `json:"tun_mtu,omitempty"`
}

func loadRuntimeState(configDir string) runtimeState {
	state := runtimeState{
		CoreID: "sing-box", Mode: runtimeModeGlobal, TUNName: "Navo", TUNMTU: 1500,
	}
	if configDir == "" {
		return state
	}
	data, err := os.ReadFile(filepath.Join(configDir, "runtime_state.json"))
	if err == nil {
		_ = json.Unmarshal(data, &state)
	}
	// Always reset to global mode on fresh start so traffic goes through the selected node.
	state.Mode = runtimeModeGlobal
	state.TUNEnabled = false
	if state.TUNName == "" {
		state.TUNName = "Navo"
	}
	if state.TUNMTU <= 0 {
		state.TUNMTU = 1500
	}
	if state.CoreID == "" {
		state.CoreID = "sing-box"
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
	s.runtimeMu.Lock()
	previous := s.runtime
	s.runtime.TUNEnabled = enabled
	s.runtime.TUNName = strings.TrimSpace(name)
	s.runtime.TUNMTU = mtu
	s.runtimeMu.Unlock()

	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", ""); err != nil {
		s.runtimeMu.Lock()
		s.runtime = previous
		s.runtimeMu.Unlock()
		return fmt.Errorf("apply TUN configuration: %w", err)
	}
	return nil
}

func validRuntimeMode(mode string) bool {
	switch mode {
	case runtimeModeRule, runtimeModeGlobal, runtimeModeDirect:
		return true
	default:
		return false
	}
}

func (s *Service) handleOutboundSelect(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if strings.TrimSpace(id) == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("outbound id is required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), id, ""); err != nil {
		return errorResponse(requestID, "SELECTION_APPLY_FAILED", err)
	}
	return response(requestID, map[string]interface{}{"active_id": id})
}

func (s *Service) handleRuntimeModeSet(requestID string, msg map[string]interface{}) map[string]interface{} {
	mode, _ := msg["mode"].(string)
	if !validRuntimeMode(mode) {
		return errorResponse(requestID, "INVALID", fmt.Errorf("mode must be rule, global, or direct"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.applyRuntimeConfig(ctx, s.currentOutbounds(ctx), "", mode); err != nil {
		return errorResponse(requestID, "RUNTIME_MODE_APPLY_FAILED", err)
	}
	return response(requestID, map[string]interface{}{"mode": mode})
}

func (s *Service) handleRuntimeStatus(requestID string) map[string]interface{} {
	s.runtimeMu.Lock()
	mode := s.runtime.Mode
	activeID := s.runtime.SelectedOutbound
	tunEnabled := s.runtime.TUNEnabled
	s.runtimeMu.Unlock()
	s.diagnosticMu.RLock()
	exitIP := s.exitIP
	exitCountry := s.exitCountry
	s.diagnosticMu.RUnlock()
	return response(requestID, map[string]interface{}{
		"mode":         mode,
		"active_id":    activeID,
		"tun_enabled":  tunEnabled,
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
	}
	if !validRuntimeMode(next.Mode) {
		next.Mode = runtimeModeRule
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

	finalOutbound := "direct"
	if next.Mode != runtimeModeDirect && next.SelectedOutbound != "" {
		finalOutbound = next.SelectedOutbound
	}
	rules := make([]compiler.RoutingRule, 0)
	if next.Mode == runtimeModeRule && finalOutbound != "direct" {
		rules = append(rules,
			compiler.RoutingRule{
				ID: "private-networks", Name: "Private networks",
				RuleType: compiler.RuleIP,
				Values: []string{
					"10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
					"169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16",
				},
				OutboundID: "direct", OutboundTag: "direct", Enabled: true,
			},
			compiler.RoutingRule{
				ID: "cn-domains", Name: "CN domains",
				RuleType: compiler.RuleDomainSuffix, Values: []string{"cn"},
				OutboundID: "direct", OutboundTag: "direct", Enabled: true,
			},
		)
	}

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
			Address: []string{"172.19.0.1/30"},
			// The capture coordinator installs owned routes only after the core
			// and adapter pass readiness checks.
			AutoRoute: false, StrictRoute: false,
		}
	}
	config := &compiler.Config{
		SchemaVersion: 1,
		Log: compiler.LogConfig{
			Level: "info", Output: filepath.Join(configDir, "sing-box.log"),
			Timestamp: true,
		},
		Inbounds:      inbounds,
		Outbounds:     enabled,
		RoutingRules:  rules,
		FinalOutbound: finalOutbound,
		TUN:           tunConfig,
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
			log.Printf("[service] swap config failed, restoring previous: %v", err)
			if previousPath != "" {
				if startErr := s.sup.Start(context.Background(), previousPath); startErr != nil {
					log.Printf("[service] restore previous config also failed: %v", startErr)
					return fmt.Errorf("swap failed and restore failed: %w", startErr)
				}
			}
			_ = os.Remove(candidatePath)
			if s.revisionRepo != nil {
				_ = s.revisionRepo.MarkFailed(context.Background(), compiled.RevisionID, "startup")
			}
			return fmt.Errorf("activate candidate revision: %w", err)
		}
	}
	next.RevisionID = compiled.RevisionID
	next.ConfigHash = compiled.ContentHash
	next.RevisionStatus = "candidate"
	next.LastKnownGood = previousRuntime.LastKnownGood
	if wasRunning {
		next.RevisionStatus = "active"
		next.LastKnownGood = candidatePath
	}
	s.cfg.ConfigPath = candidatePath
	s.runtime = next
	s.ipDetector.ClearCache()
	s.directIPDetector.ClearCache()
	rollback := func(cause error) error {
		if wasRunning && previousPath != "" {
			if rollbackErr := s.sup.SwapConfig(context.Background(), previousPath); rollbackErr != nil {
				return fmt.Errorf("%v; rollback failed: %w", cause, rollbackErr)
			}
		}
		s.cfg.ConfigPath = previousPath
		s.runtime = previousRuntime
		_ = os.Remove(candidatePath)
		return cause
	}
	if err := s.saveRuntimeStateLocked(configDir); err != nil {
		return rollback(err)
	}
	if wasRunning && !disconnected && s.selectionRepo != nil {
		if err := s.selectionRepo.Save(ctx, activeSelection); err != nil {
			return rollback(fmt.Errorf("commit active selection: %w", err))
		}
	}
	if wasRunning && s.revisionRepo != nil {
		if err := s.revisionRepo.MarkActive(ctx, compiled.RevisionID); err != nil {
			return rollback(fmt.Errorf("commit active revision: %w", err))
		}
	}
	if wasRunning {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath)
	} else if previousRuntime.RevisionStatus == "active" {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath, previousPath)
	} else {
		cleanupGeneratedRuntimeFiles(configDir, candidatePath)
	}
	return nil
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

func (s *Service) handleOutboundCreate(requestID string, msg map[string]interface{}) map[string]interface{} {
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
	createdRefs := make([]string, 0, 2)
	storeCredential := func(value string) (*string, error) {
		if value == "" {
			return nil, nil
		}
		reference, err := s.credentialStore.Put(context.Background(), []byte(value))
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
			_ = s.credentialStore.Delete(context.Background(), reference)
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
			_ = s.credentialStore.Delete(context.Background(), reference)
		}
		return errorResponse(requestID, "UPSTREAM_PROXY_CREATE_FAILED", err)
	}
	applyCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.applyRuntimeConfig(
		applyCtx,
		s.currentOutbounds(applyCtx),
		upstream.ID,
		runtimeModeGlobal,
	); err != nil {
		_, _, _ = s.upstreamMgr.Remove(upstream.ID)
		for _, reference := range createdRefs {
			_ = s.credentialStore.Delete(context.Background(), reference)
		}
		return errorResponse(requestID, "OUTBOUND_APPLY_FAILED", err)
	}
	return response(requestID, map[string]interface{}{
		"id": upstream.ID, "source_type": "upstream_proxy", "status": "created",
	})
}

func (s *Service) handleOutboundDelete(requestID string, msg map[string]interface{}) map[string]interface{} {
	id, _ := msg["id"].(string)
	if id == "" {
		return errorResponse(requestID, "INVALID", fmt.Errorf("id required"))
	}
	removed, found, err := s.upstreamMgr.Remove(id)
	if err != nil {
		return errorResponse(requestID, "UPSTREAM_PROXY_DELETE_FAILED", err)
	}
	if !found {
		return errorResponse(requestID, "NOT_FOUND", fmt.Errorf("outbound %q not found", id))
	}
	for _, reference := range []*string{removed.UsernameRef, removed.PasswordRef} {
		if reference != nil {
			_ = s.credentialStore.Delete(context.Background(), *reference)
		}
	}
	go s.applyRuntimeConfig(context.Background(), s.currentOutbounds(context.Background()), "", "")
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
