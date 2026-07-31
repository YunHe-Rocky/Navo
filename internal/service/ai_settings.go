package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"navo/internal/ai"
	"navo/internal/securestore"
)

type persistedAIConfig struct {
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	ProtectedKey   string `json:"protected_api_key,omitempty"`
}

func aiConfigPath(configDir string) string {
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "ai-settings.json")
}

func loadAIConfig(configDir string) (ai.Config, error) {
	path := aiConfigPath(configDir)
	if path == "" {
		return ai.Config{}, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ai.Config{}, err
	}
	var stored persistedAIConfig
	if err := json.Unmarshal(data, &stored); err != nil {
		return ai.Config{}, fmt.Errorf("decode AI settings: %w", err)
	}
	keyData, err := base64.StdEncoding.DecodeString(stored.ProtectedKey)
	if err != nil {
		return ai.Config{}, fmt.Errorf("decode protected API key: %w", err)
	}
	apiKey, err := securestore.Unprotect(keyData)
	if err != nil {
		return ai.Config{}, fmt.Errorf("unprotect API key: %w", err)
	}
	return ai.Config{
		BaseURL:        stored.BaseURL,
		Model:          stored.Model,
		APIKey:         string(apiKey),
		TimeoutSeconds: stored.TimeoutSeconds,
	}, nil
}

func saveAIConfig(configDir string, cfg ai.Config) error {
	if configDir == "" {
		return fmt.Errorf("AI settings directory is not configured")
	}
	protected, err := securestore.Protect([]byte(cfg.APIKey))
	if err != nil {
		return fmt.Errorf("protect API key: %w", err)
	}
	stored := persistedAIConfig{
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		TimeoutSeconds: cfg.TimeoutSeconds,
		ProtectedKey:   base64.StdEncoding.EncodeToString(protected),
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode AI settings: %w", err)
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create AI settings directory: %w", err)
	}
	if err := os.WriteFile(aiConfigPath(configDir), data, 0600); err != nil {
		return fmt.Errorf("write AI settings: %w", err)
	}
	return nil
}

func (s *Service) handleAIConfigGet(requestID string) map[string]interface{} {
	s.aiMu.RLock()
	cfg := s.aiConfig
	s.aiMu.RUnlock()
	return response(requestID, map[string]interface{}{
		"base_url":        cfg.BaseURL,
		"model":           cfg.Model,
		"timeout_seconds": cfg.TimeoutSeconds,
		"has_api_key":     strings.TrimSpace(cfg.APIKey) != "",
	})
}

func (s *Service) handleAIConfigSet(requestID string, msg map[string]interface{}) map[string]interface{} {
	s.aiMu.RLock()
	cfg := s.aiConfig
	s.aiMu.RUnlock()

	if value, ok := msg["base_url"].(string); ok {
		cfg.BaseURL = strings.TrimSpace(value)
	}
	if value, ok := msg["model"].(string); ok {
		cfg.Model = strings.TrimSpace(value)
	}
	if value, ok := msg["api_key"].(string); ok && strings.TrimSpace(value) != "" {
		cfg.APIKey = strings.TrimSpace(value)
	}
	if value, ok := msg["timeout_seconds"].(float64); ok {
		cfg.TimeoutSeconds = int(value)
	}
	if cfg.TimeoutSeconds < 5 || cfg.TimeoutSeconds > 300 {
		return failure(requestID, "AI_CONFIG_INVALID", "timeout must be between 5 and 300 seconds")
	}
	if err := cfg.Validate(); err != nil {
		return failure(requestID, "AI_CONFIG_INVALID", err.Error())
	}
	if err := saveAIConfig(s.cfg.ConfigDir, cfg); err != nil {
		return failure(requestID, "AI_CONFIG_SAVE", err.Error())
	}

	s.aiMu.Lock()
	s.aiConfig = cfg
	s.aiAssistant = ai.NewAssistant(ai.NewHTTPBackend(cfg))
	s.aiMu.Unlock()
	return response(requestID, map[string]interface{}{"status": "saved"})
}

func (s *Service) handleAIConfigTest(requestID string) map[string]interface{} {
	s.aiMu.RLock()
	cfg := s.aiConfig
	s.aiMu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	if err := ai.NewHTTPBackend(cfg).Test(ctx); err != nil {
		return failure(requestID, "AI_CONNECTION_FAILED", err.Error())
	}
	return response(requestID, map[string]interface{}{"status": "ok"})
}

func response(requestID string, payload map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"request_id": requestID, "type": "RESPONSE", "payload": payload}
}

func failure(requestID, code, message string) map[string]interface{} {
	return map[string]interface{}{
		"request_id": requestID,
		"type":       "ERROR",
		"payload":    map[string]interface{}{"code": code, "message": message},
	}
}
