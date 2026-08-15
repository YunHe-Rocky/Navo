package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/domain/capture"
)

var errCaptureBusy = errors.New("capture transition is already in progress")

const (
	captureRecoveryTimeout  = 45 * time.Second
	captureHealthInterval   = 2 * time.Second
	captureHealthFailures   = 3
	captureLockPollInterval = 25 * time.Millisecond
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
}

func (a *Agent) transitionCaptureMode(ctx context.Context, target capture.Mode) error {
	if err := a.lockCapture(ctx); err != nil {
		return err
	}
	defer a.captureMu.Unlock()
	return a.transitionCaptureModeLocked(ctx, target)
}

// transitionCaptureModeLocked runs one capture transaction while the caller
// owns captureMu. It lets higher-level operations such as core switching keep
// stop/switch/re-enable atomic across the Agent boundary.
func (a *Agent) transitionCaptureModeLocked(ctx context.Context, target capture.Mode) error {
	if target == capture.ModeTUN && !a.cfg.IsElevatedFn() {
		return fmt.Errorf("TUN_REQUIRES_ADMIN: TUN requires Navo to run as administrator")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current := a.captureSnapshot()
	if current.State == capture.StateRecovering || current.State == capture.StateFaulted {
		if err := a.recoverCaptureLocked(ctx); err != nil {
			return fmt.Errorf("capture recovery must complete before enabling %s: %w", target, err)
		}
		current = a.captureSnapshot()
	}
	if current.CommittedMode == target &&
		current.DesiredMode == target &&
		current.State == capture.RunningState(target) &&
		current.FaultID == "" && current.LastError == "" {
		// A repeated request for an already healthy mode is idempotent. Tearing
		// down and rebuilding TUN here creates a needless core/listener race.
		return nil
	}

	from := current.CommittedMode
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
	servicePayload, err := a.prepareServiceCapture(ctx, target)
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
		status := a.ProxyStatus()
		expected := fmt.Sprintf("127.0.0.1:%d", a.cfg.ProxyPort)
		if !status.Enabled || status.ProxyServer != expected {
			return a.captureFailure(target, journal, fmt.Errorf(
				"system proxy ownership check failed: expected %s, got enabled=%t server=%s",
				expected, status.Enabled, status.ProxyServer,
			))
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
		if a.captureProbe != nil {
			err = a.captureProbe(probeCtx, target)
		}
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

func (a *Agent) selectCoreWithCapture(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	targetCore, _ := msg["core_id"].(string)
	targetCore = strings.TrimSpace(targetCore)
	if targetCore == "" {
		return agentError(requestID, "INVALID", fmt.Errorf("core_id is required"))
	}
	if err := a.lockCapture(ctx); err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer a.captureMu.Unlock()

	previousCore, err := a.activeCoreID(
		ctx,
		fmt.Sprintf("core-status-before-switch-%d", time.Now().UnixNano()),
	)
	if err != nil {
		return agentError(requestID, "CORE_STATUS_FAILED", err)
	}
	if previousCore == targetCore {
		return agentResponse(requestID, map[string]interface{}{"active": targetCore})
	}

	previousMode := a.captureSnapshot().CommittedMode
	if previousMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return agentError(requestID, "CORE_SWITCH_STOP_FAILED", err)
		}
	}

	switchResponse := a.sendCoreSelect(ctx, requestID, targetCore)
	if isErrorResponse(switchResponse) {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 2*captureIPCRequestTimeout)
		restoreErr := a.restoreCoreAndCapture(restoreCtx, previousCore, previousMode)
		cancel()
		if restoreErr != nil {
			return agentError(requestID, "CORE_SWITCH_ROLLBACK_FAILED", errors.Join(
				fmt.Errorf("switch to %s: %s", targetCore, responseMessage(switchResponse)),
				restoreErr,
			))
		}
		return switchResponse
	}
	if previousMode == capture.ModeOff {
		return switchResponse
	}
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err == nil {
		return switchResponse
	} else {
		activationErr := err
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 2*captureIPCRequestTimeout)
		defer cancel()
		rollbackErr := a.restoreCoreAndCapture(rollbackCtx, previousCore, previousMode)
		return agentError(requestID, "CORE_SWITCH_VERIFY_FAILED", errors.Join(
			fmt.Errorf("core %s failed capture verification: %w", targetCore, activationErr),
			rollbackErr,
		))
	}
}

func (a *Agent) selectOutboundWithCapture(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	targetID, _ := msg["id"].(string)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return agentError(requestID, "INVALID", fmt.Errorf("outbound id is required"))
	}
	if err := a.lockCapture(ctx); err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer a.captureMu.Unlock()

	previousID, err := a.activeOutboundID(ctx)
	if err != nil {
		return agentError(requestID, "OUTBOUND_STATUS_FAILED", err)
	}
	if previousID == targetID {
		return agentResponse(requestID, map[string]interface{}{"active_id": targetID})
	}
	previousMode := a.captureSnapshot().CommittedMode
	if previousMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return agentError(requestID, "OUTBOUND_SWITCH_STOP_FAILED", err)
		}
	}

	selectionResponse := a.sendOutboundSelect(ctx, requestID, targetID)
	if isErrorResponse(selectionResponse) {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 2*captureIPCRequestTimeout)
		defer cancel()
		rollbackErr := a.restoreOutboundAndCapture(rollbackCtx, previousID, previousMode)
		if rollbackErr != nil {
			return agentError(requestID, "OUTBOUND_SWITCH_ROLLBACK_FAILED", errors.Join(
				fmt.Errorf("select outbound %s: %s", targetID, responseMessage(selectionResponse)),
				rollbackErr,
			))
		}
		return selectionResponse
	}
	if previousMode == capture.ModeOff {
		return selectionResponse
	}
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err == nil {
		return selectionResponse
	} else {
		activationErr := err
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 2*captureIPCRequestTimeout)
		defer cancel()
		rollbackErr := a.restoreOutboundAndCapture(rollbackCtx, previousID, previousMode)
		return agentError(requestID, "OUTBOUND_SWITCH_VERIFY_FAILED", errors.Join(
			fmt.Errorf("outbound %s failed capture verification: %w", targetID, activationErr),
			rollbackErr,
		))
	}
}

func (a *Agent) mutateSourcesWithCapture(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	if err := a.lockCapture(ctx); err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer a.captureMu.Unlock()

	previousMode := a.captureSnapshot().CommittedMode
	if previousMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return agentError(requestID, "SOURCE_MUTATION_STOP_FAILED", err)
		}
	}
	mutationResponse, err := a.SendToServiceContext(ctx, msg)
	if err != nil {
		mutationResponse = agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	if isErrorResponse(mutationResponse) {
		if previousMode != capture.ModeOff {
			restoreCtx, cancel := context.WithTimeout(context.Background(), captureIPCRequestTimeout)
			restoreErr := a.transitionCaptureModeLocked(restoreCtx, previousMode)
			cancel()
			if restoreErr != nil {
				return agentError(requestID, "SOURCE_MUTATION_ROLLBACK_FAILED", errors.Join(
					fmt.Errorf("source mutation: %s", responseMessage(mutationResponse)),
					restoreErr,
				))
			}
		}
		return mutationResponse
	}
	if previousMode == capture.ModeOff {
		return mutationResponse
	}
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err != nil {
		return agentError(requestID, "SOURCE_MUTATION_RECONNECT_FAILED", fmt.Errorf(
			"source mutation committed but %s capture could not be restored: %w",
			previousMode,
			err,
		))
	}
	return mutationResponse
}

func (a *Agent) restoreOutboundAndCapture(
	ctx context.Context,
	previousID string,
	previousMode capture.Mode,
) error {
	actualID, statusErr := a.activeOutboundID(ctx)
	var restoreErr error
	if statusErr != nil {
		restoreErr = fmt.Errorf("query outbound during rollback: %w", statusErr)
	} else if actualID != previousID {
		if previousID == "" {
			restoreErr = fmt.Errorf("cannot restore an empty previous outbound after observing %s", actualID)
		} else {
			response := a.sendOutboundSelect(
				ctx,
				fmt.Sprintf("outbound-switch-rollback-%d", time.Now().UnixNano()),
				previousID,
			)
			if isErrorResponse(response) {
				restoreErr = fmt.Errorf("restore outbound %s: %s", previousID, responseMessage(response))
			}
		}
	}
	if restoreErr != nil || previousMode == capture.ModeOff {
		return restoreErr
	}
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err != nil {
		return fmt.Errorf("restore %s capture: %w", previousMode, err)
	}
	return nil
}

func (a *Agent) activeOutboundID(ctx context.Context) (string, error) {
	response := a.callServiceContext(
		ctx,
		fmt.Sprintf("runtime-status-%d", time.Now().UnixNano()),
		"runtime.status",
	)
	if isErrorResponse(response) {
		return "", fmt.Errorf("query active outbound: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("query active outbound returned an invalid payload")
	}
	id, _ := payload["active_id"].(string)
	return strings.TrimSpace(id), nil
}

func (a *Agent) sendOutboundSelect(ctx context.Context, requestID, outboundID string) map[string]interface{} {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID,
		"method":     "outbound.select",
		"id":         outboundID,
	})
	if err != nil {
		return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	return response
}

func (a *Agent) restoreCoreAndCapture(
	ctx context.Context,
	previousCore string,
	previousMode capture.Mode,
) error {
	if err := a.reconcileActiveCore(ctx, previousCore); err != nil {
		return err
	}
	if previousMode == capture.ModeOff {
		return nil
	}
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err != nil {
		return fmt.Errorf("restore %s capture: %w", previousMode, err)
	}
	return nil
}

// reconcileActiveCore closes the outcome-ambiguity window where Service may
// commit core.select but the Named Pipe response is lost. Capture stays off
// until the previous core is observed again.
func (a *Agent) reconcileActiveCore(ctx context.Context, expectedCore string) error {
	expectedCore = strings.TrimSpace(expectedCore)
	if expectedCore == "" {
		return fmt.Errorf("previous core identity is empty")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	actualCore, err := a.activeCoreID(
		ctx,
		fmt.Sprintf("core-status-before-rollback-%d", time.Now().UnixNano()),
	)
	if err != nil {
		return fmt.Errorf("reconcile active core: %w", err)
	}
	if actualCore == expectedCore {
		return nil
	}

	selectResponse := a.sendCoreSelect(
		ctx,
		fmt.Sprintf("core-switch-rollback-%d", time.Now().UnixNano()),
		expectedCore,
	)
	selectErr := error(nil)
	if isErrorResponse(selectResponse) {
		selectErr = fmt.Errorf("restore core %s: %s", expectedCore, responseMessage(selectResponse))
	}
	verifiedCore, verifyErr := a.activeCoreID(
		ctx,
		fmt.Sprintf("core-status-after-rollback-%d", time.Now().UnixNano()),
	)
	if verifyErr == nil && verifiedCore == expectedCore {
		return nil
	}
	if verifyErr == nil {
		verifyErr = fmt.Errorf("expected active core %s, observed %s", expectedCore, verifiedCore)
	}
	return errors.Join(selectErr, verifyErr)
}

func (a *Agent) activeCoreID(ctx context.Context, requestID string) (string, error) {
	status := a.callServiceContext(ctx, requestID, "core.status")
	if isErrorResponse(status) {
		return "", fmt.Errorf("query current core: %s", responseMessage(status))
	}
	statusPayload, ok := status["payload"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("query current core returned an invalid payload")
	}
	coreID, _ := statusPayload["core_id"].(string)
	coreID = strings.TrimSpace(coreID)
	if coreID == "" {
		return "", fmt.Errorf("query current core returned an empty core_id")
	}
	return coreID, nil
}

func (a *Agent) sendCoreSelect(ctx context.Context, requestID, coreID string) map[string]interface{} {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID,
		"method":     "core.select",
		"core_id":    coreID,
	})
	if err != nil {
		return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	return response
}

func (a *Agent) lockCapture(ctx context.Context) error {
	ticker := time.NewTicker(captureLockPollInterval)
	defer ticker.Stop()
	for {
		if a.captureMu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
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
	rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), captureRecoveryTimeout)
	defer rollbackCancel()
	_, serviceErr := a.prepareServiceCapture(rollbackCtx, capture.ModeOff)
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

func (a *Agent) prepareServiceCapture(ctx context.Context, mode capture.Mode) (map[string]interface{}, error) {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("capture-service-%d", time.Now().UnixNano()),
		"method":     "capture.prepare",
		"mode":       mode.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("service capture transaction: %w", err)
	}
	if isErrorResponse(response) {
		return nil, &serviceCaptureError{code: responseCode(response), message: responseMessage(response)}
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("service capture transaction returned an invalid payload")
	}
	return payload, nil
}

type serviceCaptureError struct {
	code    string
	message string
}

func (e *serviceCaptureError) Error() string { return e.message }

func (a *Agent) prepareServiceCaptureRecovery(
	ctx context.Context,
) (map[string]interface{}, error) {
	payload, err := a.prepareServiceCapture(ctx, capture.ModeOff)
	if err != nil && ctx.Err() != nil {
		return nil, fmt.Errorf("service capture recovery timed out: %w", ctx.Err())
	}
	return payload, err
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
	current.CommittedMode = capture.ModeOff
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

func (a *Agent) recoverCaptureOnStartup(parent context.Context) error {
	a.captureMu.Lock()
	defer a.captureMu.Unlock()
	return a.recoverCaptureLocked(parent)
}

func (a *Agent) recoverCaptureLocked(parent context.Context) error {
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
	recoveryCtx, cancel := context.WithTimeout(parent, captureRecoveryTimeout)
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
		if snapshot.CommittedMode == capture.ModeTUN {
			// Service is the only owner allowed to roll back a committed TUN
			// session. Agent mirrors its stable fault instead of initiating a
			// competing capture.prepare(off) transaction.
			a.mirrorServiceTUNFault()
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

func (a *Agent) mirrorServiceTUNFault() {
	tunResponse := a.callService("capture-health-tun", "tun.status")
	if isErrorResponse(tunResponse) {
		return
	}
	if payload, ok := tunResponse["payload"].(map[string]interface{}); ok {
		a.refreshCaptureFault(payload)
	}
}

func (a *Agent) captureHealthError(mode capture.Mode) error {
	if mode == capture.ModeTUN {
		// Service owns the TUN adapter, core lifecycle, and fail-closed recovery.
		// A transient Supervisor state must not race a verified TUN session into
		// a second Agent rollback.
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
