// Package ai provides AI-powered network management features.
// It wraps LLM API calls for rule generation, diagnostics, and natural language explanations.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Backend defines an AI backend for completing prompts.
type Backend interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Config holds AI backend configuration.
type Config struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"` // e.g. "https://api.deepseek.com/v1"
	Model          string `json:"model"`    // e.g. "deepseek-chat"
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// HTTPBackend is an OpenAI-compatible HTTP API backend.
type HTTPBackend struct {
	client *http.Client
	cfg    Config
}

// NewHTTPBackend creates a new HTTP AI backend.
func NewHTTPBackend(cfg Config) *HTTPBackend {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &HTTPBackend{
		client: &http.Client{Timeout: timeout},
		cfg:    cfg,
	}
}

func (b *HTTPBackend) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if err := b.cfg.Validate(); err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"model": b.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
		"max_tokens":  1024,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(b.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.cfg.APIKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respData, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return result.Choices[0].Message.Content, nil
}

// Validate ensures a user-provided OpenAI-compatible endpoint is usable.
func (c Config) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("AI API key is not configured")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("AI model is not configured")
	}
	u, err := url.Parse(strings.TrimSpace(c.BaseURL))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("AI base URL must be a valid HTTPS URL")
	}
	if u.User != nil {
		return fmt.Errorf("AI base URL must not contain credentials")
	}
	return nil
}

// Test performs a minimal completion to verify credentials and compatibility.
func (b *HTTPBackend) Test(ctx context.Context) error {
	_, err := b.Complete(ctx, "Reply with OK only.", "ping")
	return err
}

// Assistant is the top-level AI assistant for Navo.
type Assistant struct {
	backend Backend
}

// NewAssistant creates a new AI assistant with the given backend.
func NewAssistant(backend Backend) *Assistant {
	return &Assistant{backend: backend}
}
