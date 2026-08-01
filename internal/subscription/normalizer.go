package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"navo/internal/compiler"
)

// Normalizer deduplicates and standardizes parsed outbounds.
type Normalizer struct{}

func NewNormalizer() *Normalizer { return &Normalizer{} }

// Normalize deduplicates by canonical connectivity identity. Credentials are
// represented only by a digest and are never emitted in logs or IDs.
func (n *Normalizer) Normalize(outbounds []compiler.Outbound) []compiler.Outbound {
	seen := make(map[string]bool)
	usedIDs := make(map[string]bool)
	result := make([]compiler.Outbound, 0, len(outbounds))

	for _, ob := range outbounds {
		key := connectivityFingerprint(ob)
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
		key := connectivityFingerprint(ob)
		existingMap[key] = ob
	}

	normalized := n.Normalize(newOutbounds)
	usedIDs := make(map[string]bool, len(normalized))

	// Reserve existing IDs first so the selected outbound remains stable.
	for i, ob := range normalized {
		key := connectivityFingerprint(ob)
		if ex, ok := existingMap[key]; ok {
			normalized[i].Enabled = ex.Enabled
			normalized[i].ID = ex.ID
			usedIDs[ex.ID] = true
		}
	}
	for i, ob := range normalized {
		key := connectivityFingerprint(ob)
		if _, ok := existingMap[key]; ok {
			continue
		}
		normalized[i].ID = uniqueOutboundID(ob.ID, usedIDs)
		usedIDs[normalized[i].ID] = true
	}

	return normalized
}

func connectivityFingerprint(ob compiler.Outbound) string {
	credential := sha256.Sum256([]byte(strings.Join([]string{
		ob.Username, ob.Password, ob.Password2, ob.UUID, ob.Method,
	}, "\x00")))
	canonical := strings.Join([]string{
		strings.ToLower(strings.TrimSpace(ob.ProviderID)),
		string(ob.Type), strings.ToLower(strings.TrimSpace(ob.Server)), strconv.Itoa(ob.Port),
		hex.EncodeToString(credential[:]), strings.ToLower(ob.Method), strings.ToLower(ob.Network),
		ob.TransportPath, strings.ToLower(ob.TransportHost), strings.ToLower(ob.SNI),
		strconv.FormatBool(ob.TLS), ob.RealityPublicKey, ob.RealityShortID, ob.ServiceName,
	}, "\x1f")
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
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
