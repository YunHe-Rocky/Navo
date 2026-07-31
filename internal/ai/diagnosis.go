package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Diagnosis analyzes network issues using AI.
type Diagnosis struct {
	assistant *Assistant
}

// NetworkSnapshot holds current network state for diagnosis.
type NetworkSnapshot struct {
	Outbounds []OutboundState `json:"outbounds"`
	Metrics   []MetricState   `json:"metrics"`
	Errors    []string        `json:"recent_errors"`
	Uptime    time.Duration   `json:"uptime_seconds"`
}

// OutboundState represents current outbound health.
type OutboundState struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Server  string `json:"server"`
	Healthy bool   `json:"healthy"`
	Latency int64  `json:"latency_ms"`
}

// MetricState holds a metric snapshot.
type MetricState struct {
	OutboundID string  `json:"outbound_id"`
	Latency    int64   `json:"latency_ms"`
	LossRate   float64 `json:"loss_rate"`
	Upload     int64   `json:"upload_bytes"`
	Download   int64   `json:"download_bytes"`
}

// DiagnosisResult is the AI's analysis of the network state.
type DiagnosisResult struct {
	Summary     string   `json:"summary"`
	Issues      []string `json:"issues"`
	Severity    string   `json:"severity"` // "ok", "warning", "critical"
	Suggestions []string `json:"suggestions"`
}

// NewDiagnosis creates a new diagnosis assistant.
func NewDiagnosis(assistant *Assistant) *Diagnosis {
	return &Diagnosis{assistant: assistant}
}

func (d *Diagnosis) Analyze(ctx context.Context, snapshot *NetworkSnapshot) (*DiagnosisResult, error) {
	data, _ := json.MarshalIndent(snapshot, "", "  ")

	systemPrompt := `You are a network diagnostic expert. Analyze the network state and provide a diagnosis.
Respond in valid JSON only:
{
  "summary": "one-line summary of network health",
  "issues": ["issue 1", "issue 2"],
  "severity": "ok",
  "suggestions": ["suggestion 1", "suggestion 2"]
}
Severity must be one of: "ok", "warning", "critical".`

	userPrompt := fmt.Sprintf("Analyze this network state:\n%s", string(data))

	response, err := d.assistant.backend.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI diagnosis failed: %w", err)
	}

	var result DiagnosisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		clean := extractJSON(response)
		if err2 := json.Unmarshal([]byte(clean), &result); err2 != nil {
			return nil, fmt.Errorf("parse diagnosis: %w", err)
		}
	}

	return &result, nil
}

// QuickAnalyze performs a lightweight analysis without AI (no API call).
// Useful when the AI backend is unavailable.
func QuickAnalyze(snapshot *NetworkSnapshot) *DiagnosisResult {
	result := &DiagnosisResult{
		Severity: "ok",
	}

	unhealthyCount := 0
	for _, ob := range snapshot.Outbounds {
		if !ob.Healthy {
			unhealthyCount++
			result.Issues = append(result.Issues,
				fmt.Sprintf("Outbound %q (%s) is unhealthy", ob.Name, ob.Server))
		}
		if ob.Latency > 500 {
			result.Issues = append(result.Issues,
				fmt.Sprintf("High latency on %q: %dms", ob.Name, ob.Latency))
		}
	}

	for _, m := range snapshot.Metrics {
		if m.LossRate > 0.1 {
			result.Issues = append(result.Issues,
				fmt.Sprintf("High packet loss on %s: %.1f%%", m.OutboundID, m.LossRate*100))
		}
	}

	if len(snapshot.Errors) > 0 {
		result.Issues = append(result.Issues, snapshot.Errors...)
	}

	if unhealthyCount > 0 {
		result.Severity = "warning"
	}
	if unhealthyCount > 0 && unhealthyCount >= len(snapshot.Outbounds) && len(snapshot.Outbounds) > 0 {
		result.Severity = "critical"
	}
	if len(snapshot.Errors) > 3 {
		result.Severity = "critical"
	}

	if len(result.Issues) == 0 {
		result.Summary = "All systems normal"
	} else {
		result.Summary = fmt.Sprintf("Found %d issue(s)", len(result.Issues))
	}

	result.Suggestions = generateSuggestions(result.Issues)
	return result
}

func generateSuggestions(issues []string) []string {
	suggestions := make([]string, 0)
	for _, issue := range issues {
		lower := strings.ToLower(issue)
		if strings.Contains(lower, "latency") || strings.Contains(lower, "high") {
			suggestions = append(suggestions, "Try switching to a different outbound with lower latency")
		}
		if strings.Contains(lower, "unhealthy") || strings.Contains(lower, "packet loss") {
			suggestions = append(suggestions, "The outbound may be unstable — consider replacing or removing it")
		}
		if strings.Contains(lower, "dns") {
			suggestions = append(suggestions, "Check DNS configuration — try using a different DNS server")
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "No specific actions required at this time")
	}
	return suggestions
}
