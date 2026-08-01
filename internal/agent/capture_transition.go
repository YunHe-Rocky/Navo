package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
)

var errCaptureBusy = errors.New("capture transition is already in progress")

const (
	captureRecoveryTimeout = 20 * time.Second
	captureHealthInterval  = 2 * time.Second
	captureHealthFailures  = 3
)

func (a *Agent) initializeCaptureState() {
	path := a.cfg.CaptureJournalPath
	if path == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.TempDir()
		}
		path = filepath.Join(base, "Navo", "agent", "capture_transition.json")
	}
	a.captureState = capture.InitialSnapshot()
	a.captureJournal = capture.NewJournalStore(path)
	a.captureProbe = a.cfg.CaptureProbeFn
	if a.captureProbe == nil {
		a.captureProbe = probeCaptureDataPlane
	}
}

func (a *Agent) transitionCaptureMode(ctx context.Context, target capture.Mode) error {
	if target == capture.ModeTUN && !a.cfg.IsElevatedFn() {
		return fmt.Errorf("TUN_REQUIRES_ADMIN: TUN requires Navo to run as administrator")
	}
	if !a.captureMu.TryLock() {
		return errCaptureBusy
	}
	defer a.captureMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	from := a.captureSnapshot().CommittedMode
	startedAt := time.Now()
	id := fmt.Sprintf("capture-%d", time.Now().UnixNano())
	journal := capture.TransitionJournal{
		ID: id, From: from, To: target,
		CurrentStep:       capture.PhaseStoppingOld,
		StartedAt:         time.Now().UTC(),
		SystemProxyBackup: proxyBackupMap(a.ProxyStatus()),
	}
	if err := a.captureJournal.Save(journal); err != nil {
		return err
	}
	log.Printf(
		"[agent] capture transition: transition_id=%s from=%s to=%s phase=%s",
		id, from, target, capture.PhaseStoppingOld,
	)
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StartingState(target), Phase: capture.PhaseStoppingOld,
		DesiredMode: target, CommittedMode: from, TransitionID: id,
		UpdatedAt: time.Now().UTC(),
	})

	setStep := func(phase capture.Phase) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		journal.CurrentStep = phase
		if err := a.captureJournal.Save(journal); err != nil {
			return err
		}
		snapshot := a.captureSnapshot()
		snapshot.Phase = phase
		snapshot.UpdatedAt = time.Now().UTC()
		a.setCaptureSnapshot(snapshot)
		log.Printf(
			"[agent] capture transition: transition_id=%s from=%s to=%s phase=%s elapsed_ms=%d",
			id, from, target, phase, time.Since(startedAt).Milliseconds(),
		)
		return nil
	}

	if target == capture.ModeTUN {
		if err := setStep(capture.PhaseRecovering); err != nil {
			return a.captureFailure(target, journal, err)
		}
	}
	if target != capture.ModeOff {
		if err := setStep(capture.PhaseStartingCore); err != nil {
			return a.captureFailure(target, journal, err)
		}
	}
	servicePayload, err := a.prepareServiceCapture(target)
	if err != nil {
		return a.captureFailure(target, journal, err)
	}
	if pid, ok := numberAsInt(servicePayload["pid"]); ok {
		journal.CorePID = pid
	}
	if adapter, ok := servicePayload["adapter"].(map[string]interface{}); ok {
		journal.CreatedAdapterID, _ = adapter["interface_guid"].(string)
	}

	if target == capture.ModeSystemProxy {
		if err := a.EnableProxy(); err != nil {
			return a.captureFailure(target, journal, err)
		}
	} else if err := a.DisableProxy(); err != nil {
		return a.captureFailure(target, journal, err)
	}

	if target == capture.ModeTUN {
		if err := setStep(capture.PhaseConfiguringRoute); err != nil {
			return a.captureFailure(target, journal, err)
		}
	}
	if target != capture.ModeOff {
		if err := setStep(capture.PhaseChecking); err != nil {
			return a.captureFailure(target, journal, err)
		}
		probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		err = a.captureProbe(probeCtx, target)
		cancel()
		if err != nil {
			return a.captureFailure(target, journal, fmt.Errorf("capture data-plane check: %w", err))
		}
	}

	journal.CurrentStep = capture.PhaseRunning
	journal.Committed = true
	if err := a.captureJournal.Save(journal); err != nil {
		return a.captureFailure(target, journal, err)
	}
	if err := a.captureJournal.Clear(); err != nil {
		return a.captureFailure(target, journal, err)
	}
	adapter := a.serviceAdapterStatus()
	phase := capture.PhaseRunning
	if target == capture.ModeOff {
		phase = capture.PhaseStopped
	}
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.RunningState(target), Phase: phase,
		DesiredMode: target, CommittedMode: target,
		Adapter: adapter, UpdatedAt: time.Now().UTC(),
	})
	log.Printf(
		"[agent] capture transition: transition_id=%s from=%s to=%s phase=%s result=success elapsed_ms=%d",
		id, from, target, phase, time.Since(startedAt).Milliseconds(),
	)
	return nil
}

func (a *Agent) captureFailure(
	target capture.Mode,
	journal capture.TransitionJournal,
	cause error,
) error {
	log.Printf(
		"[agent] capture transition: transition_id=%s from=%s to=%s phase=%s result=failure error=%q",
		journal.ID, journal.From, journal.To, journal.CurrentStep, cause,
	)
	snapshot := a.captureSnapshot()
	snapshot.State = capture.StateRecovering
	snapshot.Phase = capture.PhaseRollingBack
	snapshot.LastError = cause.Error()
	snapshot.UpdatedAt = time.Now().UTC()
	a.setCaptureSnapshot(snapshot)
	journal.CurrentStep = capture.PhaseRollingBack
	_ = a.captureJournal.Save(journal)

	var rollbackErr error
	rollbackErr = errors.Join(rollbackErr, a.DisableProxy())
	_, serviceErr := a.prepareServiceCapture(capture.ModeOff)
	rollbackErr = errors.Join(rollbackErr, serviceErr)
	if rollbackErr == nil {
		rollbackErr = a.captureJournal.Clear()
	}
	committedMode := journal.From
	adapter := capture.AdapterStatus{State: capture.AdapterMissing, Name: "Navo"}
	if rollbackErr == nil {
		committedMode = capture.ModeOff
	} else {
		adapter = a.serviceAdapterStatus()
	}
	faultID := fmt.Sprintf("capture-fault-%d", time.Now().UnixNano())
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateFaulted, Phase: capture.PhaseFaulted,
		DesiredMode: target, CommittedMode: committedMode,
		FaultID: faultID, LastError: errors.Join(cause, rollbackErr).Error(),
		CanRetryTUN: target == capture.ModeTUN,
		Adapter:     adapter,
		UpdatedAt:   time.Now().UTC(),
	})
	return errors.Join(cause, rollbackErr)
}

func (a *Agent) prepareServiceCapture(mode capture.Mode) (map[string]interface{}, error) {
	response, err := a.SendToService(map[string]interface{}{
		"request_id": fmt.Sprintf("capture-service-%d", time.Now().UnixNano()),
		"method":     "capture.prepare",
		"mode":       mode.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("service capture transaction: %w", err)
	}
	if isErrorResponse(response) {
		return nil, fmt.Errorf("%s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("service capture transaction returned an invalid payload")
	}
	return payload, nil
}

func (a *Agent) prepareServiceCaptureRecovery(
	ctx context.Context,
) (map[string]interface{}, error) {
	type result struct {
		payload map[string]interface{}
		err     error
	}
	done := make(chan result, 1)
	go func() {
		payload, err := a.prepareServiceCapture(capture.ModeOff)
		done <- result{payload: payload, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("service capture recovery timed out: %w", ctx.Err())
	case value := <-done:
		return value.payload, value.err
	}
}

func (a *Agent) captureSnapshot() capture.Snapshot {
	a.captureStateMu.RLock()
	defer a.captureStateMu.RUnlock()
	return a.captureState
}

func (a *Agent) setCaptureSnapshot(snapshot capture.Snapshot) {
	a.captureStateMu.Lock()
	a.captureState = snapshot
	a.captureStateMu.Unlock()
}

func (a *Agent) captureStatusPayload() map[string]interface{} {
	snapshot := a.captureSnapshot()
	return map[string]interface{}{
		"state": snapshot.State, "phase": snapshot.Phase,
		"desired_mode": snapshot.DesiredMode, "committed_mode": snapshot.CommittedMode,
		"transition_id": snapshot.TransitionID, "fault_id": snapshot.FaultID,
		"adapter": snapshot.Adapter, "last_error": snapshot.LastError,
		"can_retry_tun": snapshot.CanRetryTUN, "updated_at": snapshot.UpdatedAt,
	}
}

func (a *Agent) refreshCaptureFault(tunStatus map[string]interface{}) {
	faultID, _ := tunStatus["fault_id"].(string)
	if faultID == "" {
		return
	}
	message, _ := tunStatus["last_error"].(string)
	current := a.captureSnapshot()
	if current.FaultID == faultID {
		return
	}
	current.State = capture.StateFaulted
	current.Phase = capture.PhaseFaulted
	current.DesiredMode = capture.ModeTUN
	current.FaultID = faultID
	current.LastError = message
	current.CanRetryTUN = true
	current.UpdatedAt = time.Now().UTC()
	current.Adapter = adapterStatusFromMap(tunStatus)
	a.setCaptureSnapshot(current)
}

func (a *Agent) serviceAdapterStatus() capture.AdapterStatus {
	response := a.callService("capture-adapter-status", "tun.status")
	if isErrorResponse(response) {
		return capture.AdapterStatus{Name: "Navo", State: capture.AdapterUnknown, Error: responseMessage(response)}
	}
	payload, _ := response["payload"].(map[string]interface{})
	return adapterStatusFromMap(payload)
}

func adapterStatusFromMap(payload map[string]interface{}) capture.AdapterStatus {
	name, _ := payload["name"].(string)
	state, _ := payload["state"].(string)
	identifier, _ := payload["identifier"].(string)
	index, _ := numberAsInt(payload["interface_index"])
	return capture.AdapterStatus{
		Name: name, State: capture.AdapterState(state),
		InterfaceGUID: identifier, InterfaceIndex: index,
	}
}

func (a *Agent) recoverCaptureOnStartup(_ context.Context) error {
	a.captureMu.Lock()
	defer a.captureMu.Unlock()
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRecovering, Phase: capture.PhaseRollingBack,
		DesiredMode: capture.ModeOff, CommittedMode: capture.ModeOff,
		UpdatedAt: time.Now().UTC(),
	})
	journal, journalErr := a.captureJournal.Load()
	if journalErr == nil && !journal.Committed {
		a.setCaptureSnapshot(capture.Snapshot{
			State: capture.StateRecovering, Phase: capture.PhaseRollingBack,
			DesiredMode: journal.To, CommittedMode: capture.ModeOff,
			TransitionID: journal.ID, UpdatedAt: time.Now().UTC(),
		})
	} else if journalErr != nil && !errors.Is(journalErr, os.ErrNotExist) {
		// A malformed journal is evidence, not executable input. Continue the
		// ownership-aware cleanup and clear it only after every resource is safe.
		log.Printf("[agent] invalid capture recovery journal: %v", journalErr)
	}
	recoveryCtx, cancel := context.WithTimeout(context.Background(), captureRecoveryTimeout)
	defer cancel()
	var recoveryErr error
	recoveryErr = errors.Join(recoveryErr, a.DisableProxy())
	_, serviceErr := a.prepareServiceCaptureRecovery(recoveryCtx)
	recoveryErr = errors.Join(recoveryErr, serviceErr)
	if recoveryErr == nil {
		recoveryErr = a.captureJournal.Clear()
	}
	if recoveryErr != nil {
		target := capture.ModeOff
		transitionID := ""
		if journal != nil {
			target, transitionID = journal.To, journal.ID
		}
		fault := errors.Join(journalErr, recoveryCtx.Err(), recoveryErr)
		a.setCaptureSnapshot(capture.Snapshot{
			State: capture.StateFaulted, Phase: capture.PhaseFaulted,
			DesiredMode: target, CommittedMode: capture.ModeOff,
			TransitionID: transitionID,
			FaultID:      fmt.Sprintf("capture-recovery-fault-%d", time.Now().UnixNano()),
			LastError:    fault.Error(),
			CanRetryTUN:  target == capture.ModeTUN,
			Adapter: capture.AdapterStatus{
				State: capture.AdapterUnknown, Name: "Navo", Error: recoveryErr.Error(),
			},
			UpdatedAt: time.Now().UTC(),
		})
		return fault
	}
	a.setCaptureSnapshot(capture.InitialSnapshot())
	return nil
}

func (a *Agent) monitorCaptureHealth(ctx context.Context) {
	ticker := time.NewTicker(captureHealthInterval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
		}
		snapshot := a.captureSnapshot()
		if snapshot.CommittedMode == capture.ModeOff {
			failures = 0
			continue
		}
		healthErr := a.captureHealthError(snapshot.CommittedMode)
		if healthErr == nil {
			failures = 0
			continue
		}
		failures++
		if failures < captureHealthFailures {
			continue
		}
		failures = 0
		if err := a.recoverUnhealthyCapture(snapshot.CommittedMode, healthErr); err != nil {
			log.Printf("[agent] capture health recovery: %v", err)
		}
	}
}

func (a *Agent) captureHealthError(mode capture.Mode) error {
	coreResponse := a.callService("capture-health-core", "core.status")
	if isErrorResponse(coreResponse) {
		return fmt.Errorf("core health unavailable: %s", responseMessage(coreResponse))
	}
	corePayload, ok := coreResponse["payload"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("core health returned an invalid payload")
	}
	state, _ := corePayload["state"].(string)
	switch state {
	case "running", "degraded":
	case "starting", "reconciling", "ready", "dirty":
		return nil
	default:
		return fmt.Errorf("core is unavailable (state=%s)", state)
	}
	if mode != capture.ModeTUN {
		return nil
	}
	tunResponse := a.callService("capture-health-tun", "tun.status")
	if isErrorResponse(tunResponse) {
		return fmt.Errorf("TUN health unavailable: %s", responseMessage(tunResponse))
	}
	tunPayload, ok := tunResponse["payload"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("TUN health returned an invalid payload")
	}
	if state, _ := tunPayload["state"].(string); state != string(capture.AdapterEnabled) {
		return fmt.Errorf("TUN adapter is unavailable (state=%s)", state)
	}
	return nil
}

func (a *Agent) recoverUnhealthyCapture(mode capture.Mode, cause error) error {
	if !a.captureMu.TryLock() {
		return errCaptureBusy
	}
	defer a.captureMu.Unlock()
	if a.captureSnapshot().CommittedMode != mode {
		return nil
	}
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateRecovering, Phase: capture.PhaseRollingBack,
		DesiredMode: mode, CommittedMode: mode,
		LastError: cause.Error(), UpdatedAt: time.Now().UTC(),
	})
	recoveryCtx, cancel := context.WithTimeout(context.Background(), captureRecoveryTimeout)
	defer cancel()
	var recoveryErr error
	recoveryErr = errors.Join(recoveryErr, a.DisableProxy())
	_, serviceErr := a.prepareServiceCaptureRecovery(recoveryCtx)
	recoveryErr = errors.Join(recoveryErr, serviceErr)
	if recoveryErr == nil {
		recoveryErr = a.captureJournal.Clear()
	}
	combined := errors.Join(cause, recoveryErr)
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateFaulted, Phase: capture.PhaseFaulted,
		DesiredMode: mode, CommittedMode: capture.ModeOff,
		FaultID:     fmt.Sprintf("capture-health-fault-%d", time.Now().UnixNano()),
		LastError:   combined.Error(),
		CanRetryTUN: mode == capture.ModeTUN,
		Adapter: capture.AdapterStatus{
			State: capture.AdapterMissing, Name: "Navo",
		},
		UpdatedAt: time.Now().UTC(),
	})
	return combined
}

func probeCaptureDataPlane(ctx context.Context, mode capture.Mode) error {
	if mode != capture.ModeTUN {
		return nil
	}
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true,
		ResponseHeaderTimeout: 8 * time.Second,
	}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		"http://www.msftconnecttest.com/connecttest.txt", nil,
	)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return nil
}

func proxyBackupMap(status systemproxy.ProxyConfig) map[string]interface{} {
	// ProxyConfig has JSON-compatible exported fields; spell them out to keep
	// the transition journal independent from the WinINet implementation.
	return map[string]interface{}{
		"enabled": status.Enabled, "proxy_server": status.ProxyServer,
		"bypass_list": status.BypassList, "auto_config_url": status.AutoConfigURL,
		"auto_detect": status.AutoDetect,
	}
}

func numberAsInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}
