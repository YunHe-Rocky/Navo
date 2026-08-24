package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"navo/internal/connection"
	"navo/internal/domain/capture"
	"navo/internal/startup"
)

func (a *Agent) handleStartupStatus(ctx context.Context, requestID string) map[string]interface{} {
	if a.cfg.StartupController == nil {
		return agentResponse(requestID, startupSettingsPayload(startup.Settings{
			Supported: false, Mode: startup.ModeSystemProxy,
		}))
	}
	settings, err := a.cfg.StartupController.Status(ctx)
	if err != nil {
		return agentError(requestID, "STARTUP_STATUS_FAILED", err)
	}
	return agentResponse(requestID, startupSettingsPayload(settings))
}

func (a *Agent) handleStartupSet(
	ctx context.Context,
	requestID string,
	msg map[string]interface{},
) map[string]interface{} {
	if a.cfg.StartupController == nil {
		return agentError(requestID, "STARTUP_UNSUPPORTED", fmt.Errorf("login startup is unavailable"))
	}
	enabled, _ := msg["enabled"].(bool)
	mode, _ := msg["mode"].(string)
	mode = strings.TrimSpace(mode)
	if enabled && !startup.ValidMode(mode) {
		return agentError(requestID, "STARTUP_MODE_INVALID", fmt.Errorf("startup mode must be system_proxy or tun"))
	}
	if enabled {
		if err := a.requireConfiguredStartupRoute(ctx, requestID); err != nil {
			return agentError(requestID, "OUTBOUND_REQUIRED", err)
		}
	}
	settings, err := a.cfg.StartupController.Configure(ctx, enabled, mode)
	if err != nil {
		return agentError(requestID, "STARTUP_CONFIG_FAILED", err)
	}
	return agentResponse(requestID, startupSettingsPayload(settings))
}

func (a *Agent) requireConfiguredStartupRoute(ctx context.Context, requestID string) error {
	response, err := a.SendToServiceContext(ctx, map[string]interface{}{
		"request_id": requestID, "method": "runtime.status",
	})
	if err != nil {
		return fmt.Errorf("read runtime selection: %w", err)
	}
	if isErrorResponse(response) {
		return fmt.Errorf("read runtime selection: %s", responseMessage(response))
	}
	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("runtime selection returned an invalid payload")
	}
	runtimeMode, _ := payload["mode"].(string)
	selectedID, _ := payload["selected_id"].(string)
	if runtimeMode != "direct" && strings.TrimSpace(selectedID) == "" {
		return fmt.Errorf("add and select an available proxy route before enabling login connection")
	}
	return nil
}

func (a *Agent) restoreStartupConnection(ctx context.Context) error {
	if a.cfg.StartupController == nil {
		return nil
	}
	settings, err := a.cfg.StartupController.Status(ctx)
	if err != nil {
		return fmt.Errorf("read login startup settings: %w", err)
	}
	if !settings.Supported || !settings.Enabled {
		return nil
	}
	if !settings.Registered {
		return fmt.Errorf("login task is not registered: %s", settings.LastError)
	}
	target := capture.Mode(settings.Mode)
	if !target.Valid() || target == capture.ModeOff {
		return fmt.Errorf("stored login capture mode %q is invalid", settings.Mode)
	}
	if err := a.transitionCaptureModeWithOrigin(ctx, target, connection.OriginStartup); err != nil {
		return err
	}
	return nil
}

func startupSettingsPayload(settings startup.Settings) map[string]interface{} {
	return map[string]interface{}{
		"supported": settings.Supported, "enabled": settings.Enabled,
		"mode": settings.Mode, "registered": settings.Registered,
		"last_error": settings.LastError, "checked_at": time.Now().UTC(),
	}
}
