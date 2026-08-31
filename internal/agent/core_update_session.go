package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"navo/internal/connection"
	"navo/internal/domain/capture"
)

type coreUpdateSession struct {
	id             string
	coreID         string
	previousMode   capture.Mode
	activeRunning  bool
	captureStopped bool
	done           chan struct{}
	expiresAt      time.Time
	transaction    *connection.Transaction
	timer          *time.Timer
	stepMu         sync.Mutex
	finished       bool
}

func (a *Agent) handleCoreUpdateBegin(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	coreID, _ := msg["core_id"].(string)
	coreID = strings.TrimSpace(coreID)
	if coreID == "" {
		return agentError(requestID, "INVALID", fmt.Errorf("core_id is required"))
	}
	a.coreUpdateMu.Lock()
	activeSession := a.coreUpdateSession
	a.coreUpdateMu.Unlock()
	if activeSession != nil {
		return agentError(
			requestID,
			"CORE_UPDATE_BUSY",
			fmt.Errorf("core update session %s is already active", activeSession.id),
		)
	}

	transaction, err := a.beginConnection(
		ctx,
		requestID,
		connection.OperationCoreUpdate,
		connection.OriginUser,
		"core",
	)
	if err != nil {
		return connectionAdmissionResponse(requestID, "CONNECTION_BUSY", err)
	}
	fail := func(code string, cause error) map[string]interface{} {
		transaction.Finish(cause)
		return agentError(requestID, code, cause)
	}
	if err := transaction.SetPhase(connection.PhaseApplying); err != nil {
		return fail("CONNECTION_STATE_FAILED", err)
	}

	activeCoreID, coreState, err := a.coreUpdateTargetState(ctx, requestID)
	if err != nil {
		return fail("CORE_UPDATE_STATUS_FAILED", err)
	}
	previousMode := a.captureSnapshot().CommittedMode
	activeRunning := activeCoreID == coreID && coreState == "running"
	captureStopped := false
	if activeRunning && previousMode != capture.ModeOff {
		if err := a.transitionCaptureModeLocked(ctx, capture.ModeOff); err != nil {
			return fail("CORE_UPDATE_CAPTURE_STOP_FAILED", err)
		}
		captureStopped = true
	}
	if activeRunning {
		if err := a.executeCoreUpdateServiceStep(ctx, requestID, "core.update.stop", coreID); err != nil {
			recoveryCtx, cancel := context.WithTimeout(context.Background(), coreSwitchIPCRequestTimeout)
			restartErr := a.executeCoreUpdateServiceStep(
				recoveryCtx,
				requestID+"-restore-start",
				"core.update.start",
				coreID,
			)
			var restoreErr error
			if captureStopped && restartErr == nil {
				restoreErr = a.transitionCaptureModeLocked(recoveryCtx, previousMode)
			}
			cancel()
			return fail(
				"CORE_UPDATE_STOP_FAILED",
				errors.Join(err, restartErr, restoreErr),
			)
		}
	}

	timeout := a.cfg.CoreUpdateSessionTimeout
	session := &coreUpdateSession{
		id: requestID, coreID: coreID, previousMode: previousMode,
		activeRunning: activeRunning, captureStopped: captureStopped,
		done: make(chan struct{}), expiresAt: time.Now().Add(timeout),
		transaction: transaction,
	}
	a.coreUpdateMu.Lock()
	a.coreUpdateSession = session
	a.coreUpdateMu.Unlock()
	session.timer = time.NewTimer(timeout)
	go func() {
		select {
		case <-session.timer.C:
			a.expireCoreUpdateSession(session)
		case <-session.done:
		}
	}()

	return agentResponse(requestID, map[string]interface{}{
		"status": "prepared", "session_id": session.id, "core_id": coreID,
		"previous_mode": previousMode.String(), "active_running": activeRunning,
		"expires_at": session.expiresAt.UTC(),
	})
}

func (a *Agent) handleCoreUpdateCommit(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	session, code, err := a.coreUpdateSessionForMessage(msg)
	if err != nil {
		return agentError(requestID, code, err)
	}
	session.stepMu.Lock()
	defer session.stepMu.Unlock()
	if session.finished {
		return agentError(requestID, "CORE_UPDATE_SESSION_CLOSED", fmt.Errorf("core update session is closed"))
	}
	if err := ctx.Err(); err != nil {
		return agentError(requestID, "CORE_UPDATE_COMMIT_FAILED", err)
	}
	if err := session.transaction.SetPhase(connection.PhaseApplying); err != nil {
		return agentError(requestID, "CONNECTION_STATE_FAILED", err)
	}
	if session.activeRunning {
		if err := a.executeCoreUpdateServiceStep(ctx, requestID, "core.update.start", session.coreID); err != nil {
			return a.failCoreUpdateCommitLocked(session, requestID, err)
		}
	}
	if err := session.transaction.SetPhase(connection.PhaseVerifying); err != nil {
		return a.failCoreUpdateCommitLocked(session, requestID, err)
	}
	if session.captureStopped {
		if err := a.transitionCaptureModeLocked(ctx, session.previousMode); err != nil {
			return a.failCoreUpdateCommitLocked(session, requestID, err)
		}
	} else if session.activeRunning {
		if err := a.verifyCoreUpdateRuntime(ctx); err != nil {
			return a.failCoreUpdateCommitLocked(session, requestID, err)
		}
	}
	if err := session.transaction.SetPhase(connection.PhaseCommitting); err != nil {
		return a.failCoreUpdateCommitLocked(session, requestID, err)
	}
	a.finishCoreUpdateSessionLocked(session, nil)
	return agentResponse(requestID, map[string]interface{}{
		"status": "committed", "session_id": session.id, "core_id": session.coreID,
	})
}

func (a *Agent) failCoreUpdateCommitLocked(
	session *coreUpdateSession,
	requestID string,
	cause error,
) map[string]interface{} {
	_ = session.transaction.SetPhase(connection.PhaseRollingBack)
	recoveryCtx, cancel := context.WithTimeout(context.Background(), coreSwitchIPCRequestTimeout)
	defer cancel()
	var stopErr error
	if session.activeRunning {
		stopErr = a.executeCoreUpdateServiceStep(
			recoveryCtx,
			requestID+"-fail-closed",
			"core.update.stop",
			session.coreID,
		)
	}
	return agentError(
		requestID,
		"CORE_UPDATE_VERIFY_FAILED",
		errors.Join(cause, stopErr),
	)
}

func (a *Agent) handleCoreUpdateRollback(
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	session, code, err := a.coreUpdateSessionForMessage(msg)
	if err != nil {
		return agentError(requestID, code, err)
	}
	session.stepMu.Lock()
	defer session.stepMu.Unlock()
	if session.finished {
		return agentError(requestID, "CORE_UPDATE_SESSION_CLOSED", fmt.Errorf("core update session is closed"))
	}
	_ = session.transaction.SetPhase(connection.PhaseRollingBack)
	recoveryCtx, cancel := context.WithTimeout(context.Background(), coreSwitchIPCRequestTimeout)
	recoveryErr := a.recoverCoreUpdateSessionLocked(recoveryCtx, session, true)
	cancel()
	reason, _ := msg["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "core update rolled back"
	}
	terminalErr := errors.Join(errors.New(reason), recoveryErr)
	a.finishCoreUpdateSessionLocked(session, terminalErr)
	if recoveryErr != nil {
		return agentError(requestID, "CORE_UPDATE_ROLLBACK_FAILED", recoveryErr)
	}
	return agentResponse(requestID, map[string]interface{}{
		"status": "rolled_back", "session_id": session.id, "core_id": session.coreID,
	})
}

func (a *Agent) coreUpdateTargetState(
	ctx context.Context,
	requestID string,
) (string, string, error) {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID + "-status", "method": "core.status",
	})
	if err != nil {
		return "", "", fmt.Errorf("query core before update: %w", err)
	}
	if isErrorResponse(response) {
		return "", "", fmt.Errorf("query core before update: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("core status returned an invalid payload")
	}
	coreID, _ := payload["core_id"].(string)
	state, _ := payload["state"].(string)
	coreID = strings.TrimSpace(coreID)
	state = strings.TrimSpace(state)
	if coreID == "" || state == "" {
		return "", "", fmt.Errorf("core status omitted core_id or state")
	}
	return coreID, state, nil
}

func (a *Agent) executeCoreUpdateServiceStep(
	ctx context.Context,
	requestID string,
	method string,
	coreID string,
) error {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID, "method": method, "core_id": coreID,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if isErrorResponse(response) {
		return fmt.Errorf("%s: %s", method, responseMessage(response))
	}
	return nil
}

func (a *Agent) verifyCoreUpdateRuntime(ctx context.Context) error {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": fmt.Sprintf("core-update-verify-%d", time.Now().UnixNano()),
		"method":     "runtime.verify",
	})
	if err != nil {
		return fmt.Errorf("verify updated core: %w", err)
	}
	if isErrorResponse(response) {
		return fmt.Errorf("verify updated core: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("updated core verification returned an invalid payload")
	}
	readiness := captureReadinessFromServicePayload(payload, capture.ModeSystemProxy)
	if readiness.State != "ready" {
		return fmt.Errorf("updated core data plane is not ready: %s", readiness.Error)
	}
	return nil
}

func (a *Agent) coreUpdateSessionForMessage(
	msg map[string]interface{},
) (*coreUpdateSession, string, error) {
	sessionID, _ := msg["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, "CORE_UPDATE_SESSION_REQUIRED", fmt.Errorf("session_id is required")
	}
	a.coreUpdateMu.Lock()
	session := a.coreUpdateSession
	a.coreUpdateMu.Unlock()
	if session == nil {
		return nil, "CORE_UPDATE_SESSION_REQUIRED", fmt.Errorf("no core update session is active")
	}
	if session.id != sessionID {
		return nil, "CORE_UPDATE_SESSION_MISMATCH", fmt.Errorf("core update session does not own the Coordinator")
	}
	if coreID, _ := msg["core_id"].(string); strings.TrimSpace(coreID) != "" && strings.TrimSpace(coreID) != session.coreID {
		return nil, "CORE_UPDATE_TARGET_MISMATCH", fmt.Errorf("core update session target is %s", session.coreID)
	}
	return session, "", nil
}

func (a *Agent) recoverCoreUpdateSessionLocked(
	ctx context.Context,
	session *coreUpdateSession,
	restoreCapture bool,
) error {
	var recoveryErr error
	if session.activeRunning {
		startErr := a.executeCoreUpdateServiceStep(
			ctx,
			session.id+"-recovery-start",
			"core.update.start",
			session.coreID,
		)
		recoveryErr = errors.Join(recoveryErr, startErr)
	}
	if restoreCapture && session.captureStopped && recoveryErr == nil {
		recoveryErr = errors.Join(
			recoveryErr,
			a.transitionCaptureModeLocked(ctx, session.previousMode),
		)
	}
	return recoveryErr
}

func (a *Agent) finishCoreUpdateSessionLocked(
	session *coreUpdateSession,
	result error,
) {
	if session.finished {
		return
	}
	session.finished = true
	if session.timer != nil {
		session.timer.Stop()
	}
	session.transaction.Finish(result)
	close(session.done)
	a.coreUpdateMu.Lock()
	if a.coreUpdateSession == session {
		a.coreUpdateSession = nil
	}
	a.coreUpdateMu.Unlock()
}

func (a *Agent) expireCoreUpdateSession(session *coreUpdateSession) {
	session.stepMu.Lock()
	defer session.stepMu.Unlock()
	if session.finished {
		return
	}
	_ = session.transaction.SetPhase(connection.PhaseRollingBack)
	ctx, cancel := context.WithTimeout(context.Background(), coreSwitchIPCRequestTimeout)
	recoveryErr := a.recoverCoreUpdateSessionLocked(ctx, session, true)
	cancel()
	terminalErr := errors.Join(fmt.Errorf("core update session expired"), recoveryErr)
	a.finishCoreUpdateSessionLocked(session, terminalErr)
	log.Printf("[agent] core update session %s expired: %v", session.id, terminalErr)
}

func (a *Agent) abortCoreUpdateForShutdown() {
	a.coreUpdateMu.Lock()
	session := a.coreUpdateSession
	a.coreUpdateMu.Unlock()
	if session == nil {
		return
	}
	session.stepMu.Lock()
	defer session.stepMu.Unlock()
	if session.finished {
		return
	}
	_ = session.transaction.SetPhase(connection.PhaseRollingBack)
	ctx, cancel := context.WithTimeout(context.Background(), coreSwitchIPCRequestTimeout)
	recoveryErr := a.recoverCoreUpdateSessionLocked(ctx, session, false)
	cancel()
	terminalErr := errors.Join(fmt.Errorf("agent shut down during core update"), recoveryErr)
	a.finishCoreUpdateSessionLocked(session, terminalErr)
	if terminalErr != nil {
		log.Printf("[agent] core update shutdown recovery: %v", terminalErr)
	}
}
