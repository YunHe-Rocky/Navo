package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Explain generates natural language explanations of network state.
type Explain struct {
	assistant *Assistant
}

// NewExplain creates a new explanation assistant.
func NewExplain(assistant *Assistant) *Explain {
	return &Explain{assistant: assistant}
}

// ExplainNetwork generates a human-readable summary of network state.
func (e *Explain) ExplainNetwork(ctx context.Context, snapshot *NetworkSnapshot) (string, error) {
	data, _ := json.MarshalIndent(snapshot, "", "  ")

	systemPrompt := `You are a network assistant for Navo, a smart proxy manager.
Explain the user's current network state in simple, conversational Chinese.
Keep it under 3 sentences. Be specific about which outbounds are doing what.
Don't use technical jargon unless necessary.`

	userPrompt := fmt.Sprintf("Explain this network state to the user:\n%s", string(data))

	response, err := e.assistant.backend.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		// Fall back to rule-based explanation
		return ruleBasedExplain(snapshot), nil
	}
	return strings.TrimSpace(response), nil
}

// ruleBasedExplain generates a simple explanation without AI (no API call).
func ruleBasedExplain(snapshot *NetworkSnapshot) string {
	healthy := 0
	total := len(snapshot.Outbounds)

	for _, ob := range snapshot.Outbounds {
		if ob.Healthy {
			healthy++
		}
	}

	parts := []string{}

	if total == 0 {
		return "尚未配置任何出口节点。"
	}

	parts = append(parts, fmt.Sprintf("当前共有 %d 个出口节点，其中 %d 个正常。", total, healthy))

	// Describe each outbound
	for _, ob := range snapshot.Outbounds {
		status := "正常"
		if !ob.Healthy {
			status = "异常"
		}
		parts = append(parts, fmt.Sprintf("%s（%s）：延迟 %dms，状态%s", ob.Name, ob.Server, ob.Latency, status))
	}

	if len(snapshot.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("最近有 %d 个错误发生。", len(snapshot.Errors)))
	}

	if snapshot.Uptime > 0 {
		parts = append(parts, fmt.Sprintf("服务已运行 %.0f 分钟。", snapshot.Uptime.Minutes()))
	}

	return strings.Join(parts, " ")
}

// extractJSON attempts to extract JSON from a response that may contain surrounding text.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return s
	}
	end := strings.LastIndex(s, "}")
	if end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
}
