package ai

import (
	"context"
	"testing"
	"time"
)

// mockBackend implements Backend for testing without real API calls.
type mockBackend struct {
	response string
	err      error
}

func (m *mockBackend) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestRuleGen_Generate(t *testing.T) {
	backend := &mockBackend{
		response: `{
  "rules": [
    {
      "name": "openai-to-us",
      "description": "Route OpenAI services through US node",
      "rule_type": "domain_suffix",
      "values": ["openai.com", "chatgpt.com"],
      "outbound_id": "us-node-01",
      "confidence": 0.95,
      "reasoning": "OpenAI services are commonly routed through US-based nodes for better latency"
    }
  ]
}`,
	}

	gen := NewRuleGen(NewAssistant(backend))
	rules, err := gen.Generate(context.Background(),
		"Send all OpenAI traffic through my US node",
		[]string{"us-node-01", "jp-node-02"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.RuleType != "domain_suffix" {
		t.Errorf("rule_type = %s, want domain_suffix", r.RuleType)
	}
	if r.OutboundID != "us-node-01" {
		t.Errorf("outbound_id = %s, want us-node-01", r.OutboundID)
	}
	if len(r.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(r.Values))
	}
}

func TestRuleGen_Generate_JSONExtraction(t *testing.T) {
	backend := &mockBackend{
		response: `Here is your rule: {"rules": [{"name": "test", "rule_type": "domain", "values": ["test.com"], "outbound_id": "direct", "confidence": 0.8, "reasoning": "test"}]}`,
	}

	gen := NewRuleGen(NewAssistant(backend))
	rules, err := gen.Generate(context.Background(), "test", []string{"direct"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestDiagnosis_Analyze(t *testing.T) {
	backend := &mockBackend{
		response: `{"summary": "One outbound is experiencing high latency", "issues": ["High latency on US node"], "severity": "warning", "suggestions": ["Try switching to JP node"]}`,
	}

	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US Node", Server: "1.2.3.4", Healthy: true, Latency: 500},
		},
		Uptime: time.Hour,
	}

	diag := NewDiagnosis(NewAssistant(backend))
	result, err := diag.Analyze(context.Background(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Severity != "warning" {
		t.Errorf("severity = %s, want warning", result.Severity)
	}
	if len(result.Issues) == 0 {
		t.Error("expected issues in diagnosis")
	}
}

func TestDiagnosis_QuickAnalyze(t *testing.T) {
	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US Node", Server: "1.2.3.4", Healthy: true, Latency: 100},
			{ID: "jp", Name: "JP Node", Server: "5.6.7.8", Healthy: false, Latency: 999},
		},
		Errors: []string{"Connection reset"},
	}

	result := QuickAnalyze(snapshot)
	if result.Severity == "ok" {
		t.Error("expected non-ok severity with unhealthy outbound")
	}
	if len(result.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestDiagnosis_QuickAnalyze_AllHealthy(t *testing.T) {
	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US", Server: "1.2.3.4", Healthy: true, Latency: 50},
		},
	}

	result := QuickAnalyze(snapshot)
	if result.Severity != "ok" {
		t.Errorf("severity = %s, want ok", result.Severity)
	}
	if result.Summary == "" {
		t.Error("expected summary")
	}
}

func TestDiagnosis_QuickAnalyze_PacketLoss(t *testing.T) {
	snapshot := &NetworkSnapshot{
		Metrics: []MetricState{
			{OutboundID: "us", LossRate: 0.15, Latency: 200},
		},
	}
	result := QuickAnalyze(snapshot)
	found := false
	for _, issue := range result.Issues {
		if contains(issue, "packet loss") || contains(issue, "15") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected packet loss issue, got: %v", result.Issues)
	}
}

func TestDiagnosis_QuickAnalyze_ManyErrors(t *testing.T) {
	snapshot := &NetworkSnapshot{
		Errors: []string{"e1", "e2", "e3", "e4"},
	}
	result := QuickAnalyze(snapshot)
	if result.Severity != "critical" {
		t.Errorf("severity = %s, want critical", result.Severity)
	}
}

func TestExplain_RuleBasedExplain(t *testing.T) {
	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US Node", Server: "1.2.3.4", Healthy: true, Latency: 50},
		},
		Uptime: time.Hour,
	}

	result := ruleBasedExplain(snapshot)
	if result == "" {
		t.Error("expected explanation text")
	}
	if !contains(result, "US Node") {
		t.Errorf("expected 'US Node' in explanation: %s", result)
	}
}

func TestExplain_ExplainNetwork(t *testing.T) {
	backend := &mockBackend{
		response: "当前有 1 个出口节点，US Node 运行正常，延迟为 50ms。",
	}

	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US Node", Server: "1.2.3.4", Healthy: true, Latency: 50},
		},
	}

	ex := NewExplain(NewAssistant(backend))
	result, err := ex.ExplainNetwork(context.Background(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected explanation")
	}
}

func TestExplain_ExplainNetwork_Fallback(t *testing.T) {
	backend := &mockBackend{err: fmtError("API unavailable")}

	snapshot := &NetworkSnapshot{
		Outbounds: []OutboundState{
			{ID: "us", Name: "US", Server: "1.2.3.4", Healthy: true, Latency: 50},
		},
	}

	ex := NewExplain(NewAssistant(backend))
	result, err := ex.ExplainNetwork(context.Background(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected fallback explanation when API fails")
	}
	if !contains(result, "US") {
		t.Errorf("expected 'US' in fallback: %s", result)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{`Here is: {"key": "value"} end`, `{"key": "value"}`},
		{`no json`, `no json`},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.want {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatList(t *testing.T) {
	result := formatList([]string{"a", "b", "c"})
	if result != "a, b, c" {
		t.Errorf("formatList = %q, want 'a, b, c'", result)
	}
	result = formatList(nil)
	if result != "[none]" {
		t.Errorf("formatList(nil) = %q, want [none]", result)
	}
}

func TestGenerateSuggestions(t *testing.T) {
	suggestions := generateSuggestions([]string{
		"High latency on US node",
		"DNS resolution error",
	})
	if len(suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func fmtError(msg string) error {
	return &testError{msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
