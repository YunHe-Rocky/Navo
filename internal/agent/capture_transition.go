package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/agent/systemproxy"
	"navo/internal/connection"
	"navo/internal/domain/capture"
	"navo/internal/selfheal"
)

var errCaptureBusy = errors.New("capture transition is already in progress")

const (
	captureRecoveryTimeout     = 45 * time.Second
	captureHealthInterval      = 2 * time.Second
	captureHealthFailures      = 3
	captureActiveProbeInterval = 30 * time.Second
	captureLockPollInterval    = 25 * time.Millisecond
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
	a.captureRouteProbe = a.cfg.CaptureRouteProbeFn
}

func (a *Agent) transitionCaptureMode(
	ctx context.Context,
	target capture.Mode,
) (resultErr error) {
	transaction, err := a.beginConnection(
		ctx, "", connection.OperationCaptureSwitch, connection.OriginUser, "",
	)
	if err != nil {
		return err
	}
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		transaction.Finish(err)
		return err
	}
	resultErr = a.transitionCaptureModeLocked(ctx, target)
	// Release the user transaction before SelfHeal acquires Coordinator
	// ownership. Calling recovery while this transaction is active would
	// deterministically return CONNECTION_BUSY and suppress the repair.
	transaction.Finish(resultErr)
	if resultErr == nil || target == capture.ModeOff || ctx.Err() != nil {
		return resultErr
	}
	snapshot := a.captureSnapshot()
	if snapshot.State != capture.StateFaulted ||
		snapshot.DesiredMode != target ||
		snapshot.CommittedMode != capture.ModeOff {
		return resultErr
	}
	fault := newAttributedCaptureFault(target, resultErr, "ConnectionCoordinator", map[string]any{
		"phase": "activation", "committed_mode": snapshot.CommittedMode.String(),
	})
	if !selfheal.PlanFor(fault.evidence.Domain).Controllable {
		return resultErr
	}
	if recoveryErr := a.recoverUnhealthyCapture(target, fault); recoveryErr != nil {
		return errors.Join(resultErr, fmt.Errorf("automatic activation self-heal: %w", recoveryErr))
	}
	return nil
}

// transitionCaptureModeLocked runs one capture transaction while the caller
// owns the Connection Coordinator transaction. Higher-level operations
// such as core switching keep
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
		Readiness: capture.ReadinessEvidence{State: "checking", Scope: "chatgpt"},
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
	readiness := captureReadinessFromServicePayload(servicePayload, target)
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
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if a.captureProbe != nil {
			err = a.captureProbe(probeCtx, target)
		}
		cancel()
		if err != nil {
			return a.captureFailure(target, journal, fmt.Errorf("capture data-plane check: %w", err))
		}
		if target == capture.ModeSystemProxy {
			readiness.DefaultProxy = true
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
		Adapter: adapter, Readiness: readiness, UpdatedAt: time.Now().UTC(),
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
) (result map[string]interface{}) {
	targetCore, _ := msg["core_id"].(string)
	targetCore = strings.TrimSpace(targetCore)
	if targetCore == "" {
		return agentError(requestID, "INVALID", fmt.Errorf("core_id is required"))
	}
	transaction, err := a.beginConnection(
		ctx, requestID, connection.OperationCoreSwitch, connection.OriginUser, "core",
	)
	if err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}

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
		_ = transaction.SetPhase(connection.PhaseRollingBack)
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
	_ = transaction.SetPhase(connection.PhaseVerifying)
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err == nil {
		return switchResponse
	} else {
		activationErr := err
		_ = transaction.SetPhase(connection.PhaseRollingBack)
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
) (result map[string]interface{}) {
	targetID, _ := msg["id"].(string)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return agentError(requestID, "INVALID", fmt.Errorf("outbound id is required"))
	}
	transaction, err := a.beginConnection(
		ctx, requestID, connection.OperationNodeSwitch, connection.OriginUser, "node",
	)
	if err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}

	previousID, err := a.activeOutboundID(ctx)
	if err != nil {
		return agentError(requestID, "OUTBOUND_STATUS_FAILED", err)
	}
	if previousID == targetID {
		return agentResponse(requestID, map[string]interface{}{"active_id": targetID})
	}
	previousMode := a.captureSnapshot().CommittedMode
	if previousMode != capture.ModeOff && previousID == "" {
		return agentError(requestID, "OUTBOUND_ACTIVE_REQUIRED", fmt.Errorf("capture is active but no verified Active outbound exists"))
	}
	if previousMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return agentError(requestID, "OUTBOUND_SWITCH_STOP_FAILED", err)
		}
	}

	selectionResponse := a.sendOutboundSelect(ctx, requestID, targetID)
	if isErrorResponse(selectionResponse) {
		_ = transaction.SetPhase(connection.PhaseRollingBack)
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
	_ = transaction.SetPhase(connection.PhaseVerifying)
	if err := a.transitionCaptureModeLocked(ctx, previousMode); err == nil {
		return selectionResponse
	} else {
		activationErr := err
		_ = transaction.SetPhase(connection.PhaseRollingBack)
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
) (result map[string]interface{}) {
	transaction, err := a.beginConnection(
		ctx, requestID, connection.OperationSourceMutation, connection.OriginUser, "source",
	)
	if err != nil {
		return agentError(requestID, "CAPTURE_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}

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
		_ = transaction.SetPhase(connection.PhaseRollingBack)
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
	_ = transaction.SetPhase(connection.PhaseVerifying)
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
	actualID, statusErr := a.selectedOutboundID(ctx)
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

func (a *Agent) selectedOutboundID(ctx context.Context) (string, error) {
	response := a.callServiceContext(
		ctx,
		fmt.Sprintf("runtime-selected-status-%d", time.Now().UnixNano()),
		"runtime.status",
	)
	if isErrorResponse(response) {
		return "", fmt.Errorf("query selected outbound: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("query selected outbound returned an invalid payload")
	}
	id, _ := payload["selected_id"].(string)
	if strings.TrimSpace(id) == "" {
		id, _ = payload["active_id"].(string)
	}
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

func (a *Agent) beginConnection(
	ctx context.Context,
	requestID string,
	operation connection.Operation,
	origin connection.Origin,
	faultDomain string,
) (*connection.Transaction, error) {
	return a.coordinator.Begin(ctx, connection.Request{
		ID: requestID, Operation: operation, Origin: origin, FaultDomain: faultDomain,
	})
}

func finishConnectionResponse(
	transaction *connection.Transaction,
	result map[string]interface{},
) {
	if isErrorResponse(result) {
		transaction.Finish(errors.New(responseMessage(result)))
		return
	}
	if err := transaction.SetPhase(connection.PhaseCommitting); err != nil {
		transaction.Finish(err)
		return
	}
	transaction.Close()
}

func (a *Agent) forwardConnectionMutation(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
	operation connection.Operation,
) (result map[string]interface{}) {
	transaction, err := a.beginConnection(
		ctx, requestID, operation, connection.OriginUser, "",
	)
	if err != nil {
		return agentError(requestID, "CONNECTION_BUSY", err)
	}
	defer func() { finishConnectionResponse(transaction, result) }()
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}
	activePolicyMutation := isRuntimePolicyMutation(msg) &&
		a.captureSnapshot().CommittedMode != capture.ModeOff
	var previousPolicy runtimePolicySnapshot
	if activePolicyMutation {
		previousPolicy, err = a.snapshotRuntimePolicy(ctx)
		if err != nil {
			return agentError(requestID, "POLICY_SNAPSHOT_FAILED", err)
		}
	}
	response, err := a.SendToServiceContext(ctx, msg)
	if err != nil {
		return agentError(requestID, "AGENT_001", fmt.Errorf("service unreachable: %w", err))
	}
	if isErrorResponse(response) || !activePolicyMutation || responseExplicitlyUnchanged(response) {
		return response
	}
	if err := transaction.SetPhase(connection.PhaseVerifying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}
	if _, err := a.verifyCaptureReadiness(ctx); err == nil {
		return response
	} else {
		verificationErr := err
		_ = transaction.SetPhase(connection.PhaseRollingBack)
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 2*captureRecoveryTimeout)
		rollbackErr := a.restoreRuntimePolicy(rollbackCtx, msg, previousPolicy)
		if rollbackErr == nil {
			_, rollbackErr = a.verifyCaptureReadiness(rollbackCtx)
		}
		rollbackCancel()
		if rollbackErr == nil {
			return agentError(
				requestID,
				"POLICY_READINESS_FAILED",
				fmt.Errorf("current-user capture verification failed; previous policy restored: %w", verificationErr),
			)
		}

		stopCtx, stopCancel := context.WithTimeout(context.Background(), captureRecoveryTimeout)
		stopErr := a.transitionCaptureModeLocked(stopCtx, capture.ModeOff)
		stopCancel()
		failClosedState := "capture was disabled"
		if stopErr != nil {
			failClosedState = "capture disable also failed"
		}
		return agentError(
			requestID,
			"POLICY_READINESS_ROLLBACK_FAILED",
			fmt.Errorf(
				"policy verification failed and the previous policy could not be proven; %s: %w",
				failClosedState,
				errors.Join(verificationErr, rollbackErr, stopErr),
			),
		)
	}
}

type runtimePolicySnapshot struct {
	Mode      string
	ListMode  string
	Blacklist []string
	Whitelist []string
}

func isRuntimePolicyMutation(msg map[string]interface{}) bool {
	method, _ := msg["method"].(string)
	switch method {
	case "runtime.mode.set", "runtime.rules.set", "runtime.list_mode.set":
		return true
	default:
		return false
	}
}

func responseExplicitlyUnchanged(response map[string]interface{}) bool {
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return false
	}
	changed, ok := payload["changed"].(bool)
	return ok && !changed
}
func (a *Agent) snapshotRuntimePolicy(ctx context.Context) (runtimePolicySnapshot, error) {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("policy-snapshot-%d", time.Now().UnixNano()),
		"method":     "runtime.status",
	})
	if err != nil {
		return runtimePolicySnapshot{}, fmt.Errorf("read current runtime policy: %w", err)
	}
	if isErrorResponse(response) {
		return runtimePolicySnapshot{}, fmt.Errorf("read current runtime policy: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return runtimePolicySnapshot{}, fmt.Errorf("runtime status returned an invalid payload")
	}
	mode, _ := payload["mode"].(string)
	listMode, _ := payload["list_mode"].(string)
	blacklist, err := runtimePolicyStrings(payload["blacklist"])
	if err != nil {
		return runtimePolicySnapshot{}, fmt.Errorf("decode runtime blacklist: %w", err)
	}
	whitelist, err := runtimePolicyStrings(payload["whitelist"])
	if err != nil {
		return runtimePolicySnapshot{}, fmt.Errorf("decode runtime whitelist: %w", err)
	}
	if mode == "" || listMode == "" {
		return runtimePolicySnapshot{}, fmt.Errorf("runtime status omitted mode or list_mode")
	}
	return runtimePolicySnapshot{
		Mode: mode, ListMode: listMode,
		Blacklist: blacklist, Whitelist: whitelist,
	}, nil
}

func runtimePolicyStrings(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return []string{}, nil
	case []string:
		return append([]string(nil), typed...), nil
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("entry has type %T", item)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("value has type %T", value)
	}
}

func (a *Agent) restoreRuntimePolicy(
	ctx context.Context,
	original map[string]interface{},
	previous runtimePolicySnapshot,
) error {
	method, _ := original["method"].(string)
	request := map[string]interface{}{
		"request_id": fmt.Sprintf("policy-rollback-%d", time.Now().UnixNano()),
		"method":     method,
	}
	switch method {
	case "runtime.mode.set":
		request["mode"] = previous.Mode
	case "runtime.list_mode.set":
		request["mode"] = previous.ListMode
	case "runtime.rules.set":
		request["blacklist"] = append([]string(nil), previous.Blacklist...)
		request["whitelist"] = append([]string(nil), previous.Whitelist...)
	default:
		return fmt.Errorf("unsupported runtime policy rollback for %q", method)
	}
	response, err := a.SendToServiceContext(ctx, request)
	if err != nil {
		return fmt.Errorf("restore previous runtime policy: %w", err)
	}
	if isErrorResponse(response) {
		return fmt.Errorf("restore previous runtime policy: %s", responseMessage(response))
	}
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
		Readiness: capture.ReadinessEvidence{
			State: "failed", Scope: "chatgpt", CheckedAt: time.Now().UTC(),
			Error: errors.Join(cause, rollbackErr).Error(),
		},
		UpdatedAt: time.Now().UTC(),
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

func captureReadinessFromServicePayload(payload map[string]interface{}, mode capture.Mode) capture.ReadinessEvidence {
	if mode == capture.ModeOff {
		return capture.ReadinessEvidence{}
	}
	evidence := capture.ReadinessEvidence{State: "unverified", Scope: "chatgpt"}
	raw, ok := payload["verification"]
	if !ok {
		return evidence
	}
	data, err := json.Marshal(raw)
	if err != nil {
		evidence.State, evidence.Error = "failed", fmt.Sprintf("encode readiness evidence: %v", err)
		return evidence
	}
	var parsed struct {
		Verified   bool                             `json:"verified"`
		Sites      map[string]capture.ReadinessSite `json:"sites"`
		VerifiedAt time.Time                        `json:"verified_at"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		evidence.State, evidence.Error = "failed", fmt.Sprintf("decode readiness evidence: %v", err)
		return evidence
	}
	evidence.Sites = parsed.Sites
	evidence.CheckedAt = parsed.VerifiedAt
	if evidence.CheckedAt.IsZero() {
		evidence.CheckedAt = time.Now().UTC()
	}
	ready := parsed.Verified
	if !ready && len(parsed.Sites) > 0 {
		ready = true
		for _, site := range parsed.Sites {
			if !site.DNS || !site.TCP || !site.HTTPS {
				ready = false
				break
			}
		}
	}
	if ready {
		evidence.State = "ready"
	} else {
		evidence.State = "failed"
		evidence.Error = "ChatGPT application routing verification did not pass"
	}
	return evidence
}

func (a *Agent) verifyCaptureReadiness(ctx context.Context) (capture.ReadinessEvidence, error) {
	snapshot := a.captureSnapshot()
	if snapshot.CommittedMode == capture.ModeOff {
		return capture.ReadinessEvidence{}, fmt.Errorf("capture is not enabled")
	}
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("capture-verify-%d", time.Now().UnixNano()),
		"method":     "runtime.verify",
	})
	if err == nil && isErrorResponse(response) {
		err = fmt.Errorf("%s", responseMessage(response))
	}
	var readiness capture.ReadinessEvidence
	runtimeMode := ""
	if err == nil {
		payload, ok := response["payload"].(map[string]interface{})
		if !ok {
			err = fmt.Errorf("runtime verification returned an invalid payload")
		} else {
			if value, ok := payload["mode"].(string); ok {
				runtimeMode = strings.TrimSpace(value)
			}
			readiness = captureReadinessFromServicePayload(payload, snapshot.CommittedMode)
			if runtimeMode == "direct" {
				readiness.Scope = "direct"
			}
			if readiness.State != "ready" {
				err = fmt.Errorf("%s", readiness.Error)
			}
		}
	}
	if err == nil && snapshot.CommittedMode != capture.ModeOff && (a.captureRouteProbe != nil || a.captureProbe != nil) {
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if a.captureRouteProbe != nil {
			err = a.captureRouteProbe(probeCtx, snapshot.CommittedMode, runtimeMode)
		} else {
			err = a.captureProbe(probeCtx, snapshot.CommittedMode)
		}
		cancel()
		if snapshot.CommittedMode == capture.ModeSystemProxy {
			readiness.DefaultProxy = err == nil
		}
	}
	if err != nil {
		scope := "chatgpt"
		if runtimeMode == "direct" {
			scope = "direct"
		}
		readiness = capture.ReadinessEvidence{
			State: "failed", Scope: scope, CheckedAt: time.Now().UTC(), Error: err.Error(),
		}
	}
	snapshot.Readiness = readiness
	snapshot.UpdatedAt = time.Now().UTC()
	a.setCaptureSnapshot(snapshot)
	return readiness, err
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
func (a *Agent) recoverySnapshot() selfheal.RecoveryReport {
	a.recoveryMu.RLock()
	defer a.recoveryMu.RUnlock()
	report := cloneRecoveryReport(a.recoveryReport)
	if report.State == "" {
		report.State = selfheal.RecoveryIdle
	}
	return report
}

func (a *Agent) setRecoveryReport(report selfheal.RecoveryReport) {
	if report.UpdatedAt.IsZero() {
		report.UpdatedAt = time.Now().UTC()
	}
	a.recoveryMu.Lock()
	a.recoveryReport = cloneRecoveryReport(report)
	a.recoveryMu.Unlock()
}

func cloneRecoveryReport(report selfheal.RecoveryReport) selfheal.RecoveryReport {
	report.Rounds = append([]selfheal.RoundResult(nil), report.Rounds...)
	report.Candidates = append([]selfheal.CandidateResult(nil), report.Candidates...)
	if report.Evidence.Details != nil {
		report.Evidence.Details = make(map[string]any, len(report.Evidence.Details))
		for key, value := range report.Evidence.Details {
			report.Evidence.Details[key] = value
		}
	}
	return report
}

func (a *Agent) captureStatusPayload() map[string]interface{} {
	snapshot := a.captureSnapshot()
	transaction := a.coordinator.Snapshot()
	return map[string]interface{}{
		"state": snapshot.State, "phase": snapshot.Phase,
		"desired_mode": snapshot.DesiredMode, "committed_mode": snapshot.CommittedMode,
		"transition_id": snapshot.TransitionID, "fault_id": snapshot.FaultID,
		"adapter": snapshot.Adapter, "last_error": snapshot.LastError,
		"can_retry_tun": snapshot.CanRetryTUN, "updated_at": snapshot.UpdatedAt,
		"readiness": snapshot.Readiness,
		"recovery":  a.recoverySnapshot(),
		"transaction": map[string]interface{}{
			"busy": transaction.Busy, "id": transaction.ID,
			"operation": transaction.Operation, "origin": transaction.Origin,
			"phase": transaction.Phase, "fault_domain": transaction.FaultDomain,
			"started_at": transaction.StartedAt, "queued": transaction.Queued,
			"last_id": transaction.LastID, "last_operation": transaction.LastOperation,
			"last_phase": transaction.LastPhase, "last_error": transaction.LastError,
			"completed_at": transaction.CompletedAt,
		},
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

func (a *Agent) recoverCaptureOnStartup(parent context.Context) (resultErr error) {
	transaction, err := a.beginConnection(
		parent, "", connection.OperationRecovery, connection.OriginStartup, "capture",
	)
	if err != nil {
		return err
	}
	defer func() { transaction.Finish(resultErr) }()
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

type attributedCaptureFault struct {
	evidence selfheal.FaultEvidence
	cause    error
}

func (f *attributedCaptureFault) Error() string {
	if f == nil || f.cause == nil {
		return "capture health fault"
	}
	return f.cause.Error()
}

func (f *attributedCaptureFault) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.cause
}

func (a *Agent) monitorCaptureHealth(ctx context.Context) {
	ticker := time.NewTicker(captureHealthInterval)
	defer ticker.Stop()
	failures := 0
	faultKey := ""
	lastActiveProbe := time.Time{}
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
			failures, faultKey = 0, ""
			continue
		}
		fullProbe := failures > 0 || time.Since(lastActiveProbe) >= captureActiveProbeInterval
		fault := a.captureHealthFault(snapshot.CommittedMode, fullProbe)
		if fullProbe {
			lastActiveProbe = time.Now()
		}
		if fault == nil {
			failures, faultKey = 0, ""
			continue
		}
		key := string(fault.evidence.Code) + ":" + string(fault.evidence.Domain)
		if key != faultKey {
			faultKey, failures = key, 0
		}
		failures++
		if failures < captureHealthFailures {
			continue
		}
		failures, faultKey = 0, ""
		if err := a.recoverUnhealthyCapture(snapshot.CommittedMode, fault); err != nil {
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
	fault := a.captureHealthFault(mode, false)
	if fault == nil {
		return nil
	}
	return fault
}

func (a *Agent) captureHealthFault(mode capture.Mode, includeDataPlane bool) *attributedCaptureFault {
	if mode == capture.ModeTUN {
		tunResponse := a.callService("capture-health-tun", "tun.status")
		if isErrorResponse(tunResponse) {
			return newAttributedCaptureFault(mode, fmt.Errorf(
				"TUN health unavailable: %s", responseMessage(tunResponse),
			), "Service", map[string]any{"status": "unavailable"})
		}
		tunPayload, ok := tunResponse["payload"].(map[string]interface{})
		if !ok {
			return newAttributedCaptureFault(
				mode, errors.New("TUN health returned an invalid payload"),
				"Service", map[string]any{"status": "invalid"},
			)
		}
		state, _ := tunPayload["state"].(string)
		faultID, _ := tunPayload["fault_id"].(string)
		lastError, _ := tunPayload["last_error"].(string)
		if faultID != "" || state != string(capture.AdapterEnabled) {
			message := strings.TrimSpace(lastError)
			if message == "" {
				message = fmt.Sprintf("TUN adapter is unavailable (state=%s)", state)
			}
			return newAttributedCaptureFault(
				mode, errors.New(message), "Service",
				map[string]any{"fault_id": faultID, "adapter_state": state},
			)
		}
	} else {
		proxy := a.ProxyStatus()
		if !proxy.Enabled {
			return newAttributedCaptureFault(
				mode, errors.New("Navo no longer owns the current System Proxy"),
				"Agent", map[string]any{"wininet_endpoint": proxy.ProxyServer, "owned": false},
			)
		}
		coreResponse := a.callService("capture-health-core", "core.status")
		if isErrorResponse(coreResponse) {
			return newAttributedCaptureFault(mode, fmt.Errorf(
				"core health unavailable: %s", responseMessage(coreResponse),
			), "Service", nil)
		}
		corePayload, ok := coreResponse["payload"].(map[string]interface{})
		if !ok {
			return newAttributedCaptureFault(
				mode, errors.New("core health returned an invalid payload"), "Service", nil,
			)
		}
		state, _ := corePayload["state"].(string)
		switch state {
		case "running", "degraded", "starting", "reconciling", "ready", "dirty":
		default:
			return newAttributedCaptureFault(
				mode, fmt.Errorf("core is unavailable (state=%s)", state),
				"Service", map[string]any{"core_state": state},
			)
		}
	}
	if includeDataPlane {
		readiness, err := a.verifyCaptureReadiness(context.Background())
		if err != nil {
			return newAttributedCaptureFault(
				mode, fmt.Errorf("active data-plane verification: %w", err),
				"ConnectionCoordinator", map[string]any{
					"readiness_state": readiness.State,
					"readiness_scope": readiness.Scope,
				},
			)
		}
	}
	return nil
}

func newAttributedCaptureFault(
	mode capture.Mode,
	cause error,
	sourceService string,
	details map[string]any,
) *attributedCaptureFault {
	domain, code, summary := classifyCaptureFault(mode, cause)
	plan := selfheal.PlanFor(domain)
	if details == nil {
		details = make(map[string]any)
	}
	details["error"] = cause.Error()
	return &attributedCaptureFault{
		cause: cause,
		evidence: selfheal.FaultEvidence{
			Code: code, Domain: domain, Severity: selfheal.SeverityError,
			Summary: summary, Symptom: cause.Error(), Impact: plan.Impact,
			SourceService: sourceService, CaptureMode: mode.String(),
			ObservedAt: time.Now().UTC(), Details: details,
		},
	}
}

func classifyCaptureFault(
	mode capture.Mode,
	cause error,
) (selfheal.FaultDomain, selfheal.ErrorCode, string) {
	message := strings.ToLower(cause.Error())
	switch {
	case strings.Contains(message, "nrpt"):
		return selfheal.FaultDomainNRPT, selfheal.CodeNRPTMismatch, "Navo NRPT state is inconsistent"
	case strings.Contains(message, "firewall"):
		return selfheal.FaultDomainFirewall, selfheal.CodeFirewallMismatch, "Navo firewall state is inconsistent"
	case strings.Contains(message, "dns") || strings.Contains(message, "resolve"):
		return selfheal.FaultDomainDNS, selfheal.CodeDNSMismatch, "proxy DNS validation failed"
	case strings.Contains(message, "tun adapter") || strings.Contains(message, "adapter"):
		return selfheal.FaultDomainTUN, selfheal.CodeTUNAdapterMissing, "TUN capture validation failed"
	case strings.Contains(message, "local http proxy"):
		return selfheal.FaultDomainCore, selfheal.CodeCoreCrashed, "Navo local proxy listener is unavailable"
	case strings.Contains(message, "https") || strings.Contains(message, "tcp") ||
		strings.Contains(message, "exit") || strings.Contains(message, "timeout") ||
		strings.Contains(message, "current-user") ||
		strings.Contains(message, "chatgpt"):
		return selfheal.FaultDomainNode, selfheal.CodeNodeUnavailable, "active proxy node is unavailable"
	case strings.Contains(message, "route") || strings.Contains(message, "bypass"):
		return selfheal.FaultDomainRoute, selfheal.CodeRouteBypassMissing, "Navo route validation failed"
	case strings.Contains(message, "tun"):
		return selfheal.FaultDomainTUN, selfheal.CodeCaptureDataPlaneFailed, "TUN data plane is unavailable"
	case strings.Contains(message, "core") || strings.Contains(message, "listener"):
		return selfheal.FaultDomainCore, selfheal.CodeCoreCrashed, "Navo core is unavailable"
	case strings.Contains(message, "system proxy") || strings.Contains(message, "wininet") ||
		strings.Contains(message, "preconfig") || strings.Contains(message, "default proxy"):
		return selfheal.FaultDomainSystemProxy, selfheal.CodeSystemProxyMismatch, "System Proxy validation failed"
	case strings.Contains(message, "physical") || strings.Contains(message, "gateway"):
		return selfheal.FaultDomainPhysicalNetwork, selfheal.CodePhysicalNetworkDown, "physical network is unavailable"
	case strings.Contains(message, "unavailable") && strings.Contains(message, "health"):
		return selfheal.FaultDomainDetection, selfheal.CodeDetectionFailed, "health evidence is unavailable"
	default:
		if mode == capture.ModeTUN {
			return selfheal.FaultDomainTUN, selfheal.CodeCaptureDataPlaneFailed, "TUN data plane is unavailable"
		}
		return selfheal.FaultDomainUnknown, selfheal.CodeConnectivityUnknown, "connectivity failure could not be attributed"
	}
}

func (a *Agent) recoverUnhealthyCapture(
	mode capture.Mode,
	cause error,
) (resultErr error) {
	fault := &attributedCaptureFault{}
	if !errors.As(cause, &fault) || fault == nil || fault.cause == nil {
		fault = newAttributedCaptureFault(mode, cause, "Agent", nil)
	}
	plan := selfheal.PlanFor(fault.evidence.Domain)
	transaction, err := a.coordinator.TryBegin(connection.Request{
		Operation: connection.OperationSelfHeal, Origin: connection.OriginSelfHeal,
		FaultDomain: string(fault.evidence.Domain),
	})
	if err != nil {
		if errors.Is(err, connection.ErrBusy) {
			return errCaptureBusy
		}
		return err
	}
	defer func() { transaction.Finish(resultErr) }()
	snapshot := a.captureSnapshot()
	failedActivation := snapshot.State == capture.StateFaulted &&
		snapshot.DesiredMode == mode && snapshot.CommittedMode == capture.ModeOff
	if snapshot.CommittedMode != mode && !failedActivation {
		return nil
	}

	recoveryCtx, cancel := context.WithTimeout(context.Background(), 6*captureRecoveryTimeout)
	defer cancel()
	if activeID, activeErr := a.activeOutboundID(recoveryCtx); activeErr == nil {
		fault.evidence.OutboundID = activeID
	}
	if coreID, coreErr := a.activeCoreID(recoveryCtx, fmt.Sprintf("recovery-core-%d", time.Now().UnixNano())); coreErr == nil {
		fault.evidence.CoreID = coreID
	}
	now := time.Now().UTC()
	report := selfheal.RecoveryReport{
		ID:    fmt.Sprintf("recovery-%d", now.UnixNano()),
		State: selfheal.RecoveryDetected, Evidence: fault.evidence,
		StartedAt: now, UpdatedAt: now,
	}
	a.setRecoveryReport(report)

	if !plan.Controllable || (fault.evidence.Domain == selfheal.FaultDomainSystemProxy && !a.ProxyStatus().Enabled) {
		report.State = selfheal.RecoveryFailed
		report.Exhausted = true
		report.FinalError = cause.Error()
		report.FinalImpact = plan.Impact
		report.UpdatedAt = time.Now().UTC()
		a.setRecoveryReport(report)
		return a.failClosedRecovery(recoveryCtx, mode, cause, report)
	}

	var lastErr error
	for round := 1; round <= selfheal.MaxRepairRounds; round++ {
		action := plan.Action(round)
		result := selfheal.RoundResult{
			Round: round, Action: action, StartedAt: time.Now().UTC(),
		}
		report.State = selfheal.RecoveryRepairing
		report.UpdatedAt = result.StartedAt
		a.setRecoveryReport(report)
		_ = transaction.SetPhase(connection.PhaseApplying)
		repairErr := a.executeCaptureRepair(recoveryCtx, mode, action)
		var readiness capture.ReadinessEvidence
		if repairErr == nil {
			report.State = selfheal.RecoveryVerifying
			report.UpdatedAt = time.Now().UTC()
			a.setRecoveryReport(report)
			_ = transaction.SetPhase(connection.PhaseVerifying)
			readiness, repairErr = a.verifyCaptureReadiness(recoveryCtx)
		}
		result.CompletedAt = time.Now().UTC()
		result.Recovered = repairErr == nil && readiness.State == "ready"
		result.Evidence = fmt.Sprintf("readiness=%s scope=%s", readiness.State, readiness.Scope)
		if repairErr != nil {
			result.Error = repairErr.Error()
			_ = transaction.SetPhase(connection.PhaseRollingBack)
			if rollbackErr := a.transitionCaptureModeLocked(recoveryCtx, capture.ModeOff); rollbackErr != nil {
				result.Rollback = "failed: " + rollbackErr.Error()
				repairErr = errors.Join(repairErr, rollbackErr)
			} else {
				result.Rollback = "succeeded"
			}
		}
		report.Rounds = append(report.Rounds, result)
		report.UpdatedAt = result.CompletedAt
		a.setRecoveryReport(report)
		if result.Recovered {
			report.State = selfheal.RecoveryRecovered
			report.Recovered = true
			report.FinalImpact = "real connectivity restored"
			report.UpdatedAt = time.Now().UTC()
			a.setRecoveryReport(report)
			return nil
		}
		lastErr = repairErr
	}

	if plan.AllowFailover && fault.evidence.OutboundID != "" {
		if failoverErr := a.attemptSameChannelFailover(recoveryCtx, transaction, mode, fault.evidence.OutboundID, &report); failoverErr == nil {
			return nil
		} else {
			lastErr = errors.Join(lastErr, failoverErr)
		}
	}

	report.State = selfheal.RecoveryFailed
	report.Exhausted = true
	report.FinalError = errors.Join(cause, lastErr).Error()
	report.FinalImpact = plan.Impact
	report.UpdatedAt = time.Now().UTC()
	a.setRecoveryReport(report)
	return a.failClosedRecovery(recoveryCtx, mode, errors.Join(cause, lastErr), report)
}

type failoverDiscoveryCandidate struct {
	OutboundID string `json:"outbound_id"`
	SourceType string `json:"source_type"`
	LatencyMS  int64  `json:"latency_ms"`
	Reachable  bool   `json:"reachable"`
	Error      string `json:"error"`
}

type failoverDiscovery struct {
	SourceType string                       `json:"source_type"`
	Candidates []failoverDiscoveryCandidate `json:"candidates"`
	Rejected   []failoverDiscoveryCandidate `json:"rejected"`
}

func (a *Agent) discoverSameChannelFailoverCandidates(ctx context.Context, activeID string) (failoverDiscovery, error) {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("failover-discovery-%d", time.Now().UnixNano()),
		"method":     "outbound.failover_candidates",
		"active_id":  activeID,
	})
	if err != nil {
		return failoverDiscovery{}, fmt.Errorf("discover same-channel candidates: %w", err)
	}
	if isErrorResponse(response) {
		return failoverDiscovery{}, fmt.Errorf("discover same-channel candidates: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return failoverDiscovery{}, fmt.Errorf("discover same-channel candidates returned an invalid payload")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return failoverDiscovery{}, fmt.Errorf("encode failover candidates: %w", err)
	}
	var discovery failoverDiscovery
	if err := json.Unmarshal(encoded, &discovery); err != nil {
		return failoverDiscovery{}, fmt.Errorf("decode failover candidates: %w", err)
	}
	return discovery, nil
}

func (a *Agent) attemptSameChannelFailover(
	ctx context.Context,
	transaction *connection.Transaction,
	mode capture.Mode,
	originalID string,
	report *selfheal.RecoveryReport,
) error {
	report.State = selfheal.RecoveryFailover
	report.Failover = true
	report.UpdatedAt = time.Now().UTC()
	a.setRecoveryReport(*report)
	_ = transaction.SetPhase(connection.PhaseApplying)

	discovery, err := a.discoverSameChannelFailoverCandidates(ctx, originalID)
	if err != nil {
		return err
	}
	for _, rejected := range discovery.Rejected {
		report.Candidates = append(report.Candidates, selfheal.CandidateResult{
			OutboundID: rejected.OutboundID, SourceType: rejected.SourceType,
			LatencyMS: rejected.LatencyMS, Reachable: false, Error: rejected.Error,
			CompletedAt: time.Now().UTC(),
		})
	}
	if len(discovery.Candidates) == 0 {
		report.UpdatedAt = time.Now().UTC()
		a.setRecoveryReport(*report)
		return fmt.Errorf("no reachable %s failover candidate", discovery.SourceType)
	}

	for _, candidate := range discovery.Candidates {
		result := selfheal.CandidateResult{
			OutboundID: candidate.OutboundID, SourceType: candidate.SourceType,
			LatencyMS: candidate.LatencyMS, Reachable: candidate.Reachable,
		}
		selection := a.sendOutboundSelect(ctx, fmt.Sprintf("failover-select-%d", time.Now().UnixNano()), candidate.OutboundID)
		if isErrorResponse(selection) {
			result.Error = "select candidate: " + responseMessage(selection)
		} else {
			result.Selected = true
			_ = transaction.SetPhase(connection.PhaseVerifying)
			activationErr := a.transitionCaptureModeLocked(ctx, mode)
			if activationErr == nil {
				_, activationErr = a.verifyCaptureReadiness(ctx)
			}
			if activationErr == nil {
				result.Verified = true
				result.CompletedAt = time.Now().UTC()
				report.Candidates = append(report.Candidates, result)
				report.State = selfheal.RecoveryRecovered
				report.Recovered = true
				report.FinalError = ""
				report.FinalImpact = fmt.Sprintf("real connectivity restored through %s", candidate.OutboundID)
				report.UpdatedAt = result.CompletedAt
				a.setRecoveryReport(*report)
				return nil
			}
			result.Error = activationErr.Error()
			_ = transaction.SetPhase(connection.PhaseRollingBack)
			if rollbackErr := a.transitionCaptureModeLocked(ctx, capture.ModeOff); rollbackErr != nil {
				result.Error = errors.Join(activationErr, fmt.Errorf("stop failed candidate: %w", rollbackErr)).Error()
			}
		}
		result.CompletedAt = time.Now().UTC()
		report.Candidates = append(report.Candidates, result)
		report.UpdatedAt = result.CompletedAt
		a.setRecoveryReport(*report)
		_ = transaction.SetPhase(connection.PhaseApplying)
	}

	_ = transaction.SetPhase(connection.PhaseRollingBack)
	restore := a.sendOutboundSelect(ctx, fmt.Sprintf("failover-restore-%d", time.Now().UnixNano()), originalID)
	if isErrorResponse(restore) {
		return fmt.Errorf("all same-channel candidates failed; restore original selection: %s", responseMessage(restore))
	}
	return fmt.Errorf("all same-channel candidates failed full capture verification")
}
func (a *Agent) executeCaptureRepair(
	ctx context.Context,
	mode capture.Mode,
	action selfheal.RepairActionName,
) error {
	if action == selfheal.ActionNone {
		return fmt.Errorf("fault domain is observation-only")
	}
	if a.captureSnapshot().CommittedMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return fmt.Errorf("stop capture before %s: %w", action, err)
		}
	}
	if action == selfheal.ActionRecoverOwnedNetwork || action == selfheal.ActionReconcileOwnedNetwork {
		response, err := a.SendToServiceContext(ctx, map[string]interface{}{
			"request_id": fmt.Sprintf("recovery-network-%d", time.Now().UnixNano()),
			"method":     "network.recover",
		})
		if err != nil {
			return fmt.Errorf("%s: %w", action, err)
		}
		if isErrorResponse(response) {
			return fmt.Errorf("%s: %s", action, responseMessage(response))
		}
	}
	if action == selfheal.ActionReapplyTrafficPolicy {
		status := a.callServiceContext(ctx, fmt.Sprintf("recovery-policy-%d", time.Now().UnixNano()), "runtime.status")
		if isErrorResponse(status) {
			return fmt.Errorf("read current traffic policy: %s", responseMessage(status))
		}
		payload, _ := status["payload"].(map[string]interface{})
		modeValue, _ := payload["mode"].(string)
		response, err := a.SendToServiceContext(ctx, map[string]interface{}{
			"request_id": fmt.Sprintf("recovery-policy-apply-%d", time.Now().UnixNano()),
			"method":     "runtime.mode.set", "mode": modeValue,
		})
		if err != nil || isErrorResponse(response) {
			return fmt.Errorf("reapply traffic policy: %s", responseMessage(response))
		}
	}
	return a.transitionCaptureModeLocked(ctx, mode)
}

func (a *Agent) failClosedRecovery(
	ctx context.Context,
	mode capture.Mode,
	cause error,
	report selfheal.RecoveryReport,
) error {
	var recoveryErr error
	recoveryErr = errors.Join(recoveryErr, a.DisableProxy())
	_, serviceErr := a.prepareServiceCaptureRecovery(ctx)
	recoveryErr = errors.Join(recoveryErr, serviceErr)
	if recoveryErr == nil {
		recoveryErr = a.captureJournal.Clear()
	}
	combined := errors.Join(cause, recoveryErr)
	a.setCaptureSnapshot(capture.Snapshot{
		State: capture.StateFaulted, Phase: capture.PhaseFaulted,
		DesiredMode: mode, CommittedMode: capture.ModeOff,
		FaultID:     report.ID,
		LastError:   combined.Error(),
		CanRetryTUN: mode == capture.ModeTUN,
		Readiness: capture.ReadinessEvidence{
			State: "failed", Scope: "chatgpt", CheckedAt: time.Now().UTC(), Error: combined.Error(),
		},
		Adapter:   capture.AdapterStatus{State: capture.AdapterMissing, Name: "Navo"},
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
