package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// RuleGen generates routing rules from natural language descriptions.
type RuleGen struct {
	assistant *Assistant
}

// RuleSuggestion is an AI-generated routing rule suggestion.
type RuleSuggestion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RuleType    string   `json:"rule_type"`    // domain, domain_suffix, process_name, etc.
	Values      []string `json:"values"`        // domains, IPs, process names
	OutboundID  string   `json:"outbound_id"`   // suggested outbound
	Confidence  float64  `json:"confidence"`    // 0.0 - 1.0
	Reasoning   string   `json:"reasoning"`     // why this rule was suggested
}

// NewRuleGen creates a new rule generator.
func NewRuleGen(assistant *Assistant) *RuleGen {
	return &RuleGen{assistant: assistant}
}

// Generate creates routing rules from a natural language description.
func (g *RuleGen) Generate(ctx context.Context, userRequest string, availableOutbounds []string, existingRules []string) ([]RuleSuggestion, error) {
	systemPrompt := `You are a network routing expert. Generate routing rules based on the user's natural language request.
You must respond with valid JSON only, in this exact format:
{
  "rules": [
    {
      "name": "short-rule-name",
      "description": "what this rule does",
      "rule_type": "domain_suffix",
      "values": ["example.com", "other.com"],
      "outbound_id": "us-node",
      "confidence": 0.9,
      "reasoning": "why this rule matches the user's intent"
    }
  ]
}

Available rule types: domain, domain_suffix, domain_keyword, domain_regex, ip_cidr, process_name.
Available outbounds: %s
Existing rules (avoid duplicates): %s`

	userPrompt := fmt.Sprintf("User request: %s\nGenerate appropriate routing rules.", userRequest)
	sysPrompt := fmt.Sprintf(systemPrompt, formatList(availableOutbounds), formatList(existingRules))

	response, err := g.assistant.backend.Complete(ctx, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI rule generation failed: %w", err)
	}

	var result struct {
		Rules []RuleSuggestion `json:"rules"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Try to extract JSON from response if there's surrounding text
		clean := extractJSON(response)
		if err2 := json.Unmarshal([]byte(clean), &result); err2 != nil {
			return nil, fmt.Errorf("parse AI response: %w (raw: %s)", err, response)
		}
	}

	return result.Rules, nil
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "[none]"
	}
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}
