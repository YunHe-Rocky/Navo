package subscription

import (
	"fmt"
	"time"

	"navo/internal/compiler"
)

// Normalizer deduplicates and standardizes parsed outbounds.
type Normalizer struct{}

func NewNormalizer() *Normalizer { return &Normalizer{} }

// Normalize deduplicates outbounds by server+port+protocol and fills defaults.
func (n *Normalizer) Normalize(outbounds []compiler.Outbound) []compiler.Outbound {
	seen := make(map[string]bool)
	usedIDs := make(map[string]bool)
	result := make([]compiler.Outbound, 0, len(outbounds))

	for _, ob := range outbounds {
		key := fmt.Sprintf("%s:%d:%s", ob.Server, ob.Port, string(ob.Type))
		if seen[key] {
			continue
		}
		seen[key] = true

		// Fill defaults
		if ob.Name == "" {
			ob.Name = ob.Server
		}
		if ob.ID == "" {
			ob.ID = sanitizeID(ob.Name)
		}
		ob.ID = uniqueOutboundID(ob.ID, usedIDs)
		usedIDs[ob.ID] = true
		if ob.CreatedAt.IsZero() {
			ob.CreatedAt = time.Now()
		}
		ob.UpdatedAt = time.Now()
		ob.Enabled = true

		result = append(result, ob)
	}

	return result
}

// Merge merges new outbounds with existing ones, keeping existing Enabled state.
func (n *Normalizer) Merge(existing, newOutbounds []compiler.Outbound) []compiler.Outbound {
	existingMap := make(map[string]compiler.Outbound)
	for _, ob := range existing {
		key := fmt.Sprintf("%s:%d:%s", ob.Server, ob.Port, string(ob.Type))
		existingMap[key] = ob
	}

	normalized := n.Normalize(newOutbounds)
	usedIDs := make(map[string]bool, len(normalized))

	// Reserve existing IDs first so the selected outbound remains stable.
	for i, ob := range normalized {
		key := fmt.Sprintf("%s:%d:%s", ob.Server, ob.Port, string(ob.Type))
		if ex, ok := existingMap[key]; ok {
			normalized[i].Enabled = ex.Enabled
			normalized[i].ID = ex.ID
			usedIDs[ex.ID] = true
		}
	}
	for i, ob := range normalized {
		key := fmt.Sprintf("%s:%d:%s", ob.Server, ob.Port, string(ob.Type))
		if _, ok := existingMap[key]; ok {
			continue
		}
		normalized[i].ID = uniqueOutboundID(ob.ID, usedIDs)
		usedIDs[normalized[i].ID] = true
	}

	return normalized
}

func uniqueOutboundID(base string, used map[string]bool) string {
	if base == "" {
		base = "outbound"
	}
	if !used[base] {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}
