package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"navo/internal/ai"
)

func TestAIConfigPath_EmptyDir(t *testing.T) {
	path := aiConfigPath("")
	if path != "" {
		t.Fatalf("aiConfigPath = %q, want empty string", path)
	}
}

func TestAIConfigPath_ReturnsFile(t *testing.T) {
	path := aiConfigPath("/tmp/navo")
	if !strings.HasSuffix(path, "ai-settings.json") {
		t.Fatalf("aiConfigPath = %q", path)
	}
}

func TestLoadAIConfig_EmptyDir(t *testing.T) {
	_, err := loadAIConfig("")
	if err == nil {
		t.Fatal("expected error for empty config dir")
	}
}

func TestLoadAIConfig_MissingFile(t *testing.T) {
	_, err := loadAIConfig(t.TempDir())
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestSaveAndLoadAIConfig_RoundTrip(t *testing.T) {
	configDir := t.TempDir()
	cfg := ai.Config{
		BaseURL:        "https://api.deepseek.com/v1",
		Model:          "deepseek-chat",
		APIKey:         "sk-test-key-12345",
		TimeoutSeconds: 60,
	}

	if err := saveAIConfig(configDir, cfg); err != nil {
		t.Fatalf("saveAIConfig: %v", err)
	}

	loaded, err := loadAIConfig(configDir)
	if err != nil {
		t.Fatalf("loadAIConfig: %v", err)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL = %q, want %q", loaded.BaseURL, cfg.BaseURL)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Errorf("APIKey mismatch")
	}
	if loaded.TimeoutSeconds != cfg.TimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, want %d", loaded.TimeoutSeconds, cfg.TimeoutSeconds)
	}
}

func TestSaveAIConfig_EmptyDir(t *testing.T) {
	err := saveAIConfig("", ai.Config{BaseURL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error for empty config dir")
	}
}

func TestLoadAIConfig_InvalidJSON(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(configDir, "ai-settings.json")
	if err := os.WriteFile(path, []byte("garbage"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadAIConfig(configDir)
	if err == nil {
		t.Fatal("expected JSON decode error")
	}
}

func TestLoadAIConfig_InvalidProtectedKey(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(configDir, "ai-settings.json")
	stored := persistedAIConfig{
		BaseURL:        "https://example.com",
		Model:          "test",
		TimeoutSeconds: 60,
		ProtectedKey:   "!!!not-valid-base64!!!",
	}
	data, _ := json.MarshalIndent(stored, "", "  ")
	os.WriteFile(path, data, 0600)
	_, err := loadAIConfig(configDir)
	if err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestHandleAIConfigGet(t *testing.T) {
	svc := &Service{
		aiConfig: ai.Config{
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4.1-mini",
			APIKey:         "sk-test",
			TimeoutSeconds: 30,
		},
	}
	resp := svc.handleAIConfigGet("req-1")
	if resp["type"] != "RESPONSE" {
		t.Fatalf("type = %v", resp["type"])
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["base_url"] != "https://api.openai.com/v1" {
		t.Errorf("base_url = %v", payload["base_url"])
	}
	if payload["has_api_key"] != true {
		t.Error("has_api_key should be true")
	}
}

func TestHandleAIConfigGet_NoKey(t *testing.T) {
	svc := &Service{
		aiConfig: ai.Config{
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4.1-mini",
			APIKey:         "",
			TimeoutSeconds: 30,
		},
	}
	resp := svc.handleAIConfigGet("req-1")
	payload := resp["payload"].(map[string]interface{})
	if payload["has_api_key"] != false {
		t.Error("has_api_key should be false when APIKey is empty")
	}
}

func TestHandleAIConfigSet_Validation(t *testing.T) {
	configDir := t.TempDir()
	svc := &Service{
		cfg:      Config{ConfigDir: configDir},
		aiConfig: ai.Config{BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-test", TimeoutSeconds: 30},
		aiAssistant: ai.NewAssistant(ai.NewHTTPBackend(ai.Config{
			BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-test", TimeoutSeconds: 30,
		})),
	}

	resp := svc.handleAIConfigSet("req-1", map[string]interface{}{
		"timeout_seconds": float64(3), // below minimum
	})
	if resp["type"] != "ERROR" {
		t.Fatalf("expected error for timeout < 5: %v", resp)
	}
}

func TestHandleAIConfigSet_TimeoutAboveMax(t *testing.T) {
	configDir := t.TempDir()
	svc := &Service{
		cfg:      Config{ConfigDir: configDir},
		aiConfig: ai.Config{BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-test", TimeoutSeconds: 30},
		aiAssistant: ai.NewAssistant(ai.NewHTTPBackend(ai.Config{
			BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini", APIKey: "sk-test", TimeoutSeconds: 30,
		})),
	}

	resp := svc.handleAIConfigSet("req-1", map[string]interface{}{
		"timeout_seconds": float64(400),
	})
	if resp["type"] != "ERROR" {
		t.Fatalf("expected error for timeout > 300: %v", resp)
	}
}

func TestResponse(t *testing.T) {
	resp := response("id-1", map[string]interface{}{"key": "value"})
	if resp["type"] != "RESPONSE" {
		t.Errorf("type = %v", resp["type"])
	}
	if resp["request_id"] != "id-1" {
		t.Errorf("request_id = %v", resp["request_id"])
	}
}

func TestFailure(t *testing.T) {
	resp := failure("id-1", "ERR_001", "something went wrong")
	if resp["type"] != "ERROR" {
		t.Errorf("type = %v", resp["type"])
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["code"] != "ERR_001" {
		t.Errorf("code = %v", payload["code"])
	}
}

func TestErrorResponse(t *testing.T) {
	resp := errorResponse("id-1", "ERR", os.ErrNotExist)
	payload := resp["payload"].(map[string]interface{})
	if payload["code"] != "ERR" {
		t.Errorf("code = %v", payload["code"])
	}
}
