package subscription

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"navo/internal/compiler"
	"navo/internal/credential"
	"navo/internal/fsatomic"
	"navo/internal/securestore"
	"navo/internal/subscription/parser"
)

// Subscription represents a managed airport subscription.
type Subscription struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	URL              string    `json:"url,omitempty"` // legacy migration input; never set for new records
	URLCredentialRef string    `json:"url_credential_ref"`
	URLHash          string    `json:"url_hash"`
	Enabled          bool      `json:"enabled"`
	SkipTLSVerify    bool      `json:"skip_tls_verify,omitempty"`
	NodeCount        int       `json:"node_count"`
	LastError        string    `json:"last_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Manager orchestrates fetching, parsing, and normalizing subscriptions.
type Manager struct {
	fetcher         *Fetcher
	normalizer      *Normalizer
	mu              sync.RWMutex
	subs            []Subscription
	outbounds       []compiler.Outbound
	storePath       string
	credentialStore credential.Store
	protectData     func([]byte) ([]byte, error)
	unprotectData   func([]byte) ([]byte, error)
	loadErr         error
}

type persistedState struct {
	Subscriptions []Subscription      `json:"subscriptions"`
	Outbounds     []compiler.Outbound `json:"outbounds,omitempty"` // legacy plaintext migration only
}

type persistedMetadata struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// NewManager creates a new subscription manager.
func NewManager() *Manager {
	return NewManagerWithPath("")
}

// NewManagerWithPath creates a subscription manager with JSON persistence.
func NewManagerWithPath(storePath string) *Manager {
	manager, err := NewManagerWithCredentialStore(storePath, credential.NewMemoryStore())
	if err == nil {
		return manager
	}
	log.Printf("[subscription] state load failed: %v", err)
	return &Manager{
		fetcher: NewFetcher(), normalizer: NewNormalizer(),
		subs: make([]Subscription, 0), outbounds: make([]compiler.Outbound, 0),
		storePath: storePath, credentialStore: credential.NewMemoryStore(),
		protectData: securestore.Protect, unprotectData: securestore.Unprotect,
	}
}

func NewManagerWithCredentialStore(
	storePath string,
	credentialStore credential.Store,
) (*Manager, error) {
	return newManagerWithProtector(
		storePath, credentialStore, securestore.Protect, securestore.Unprotect,
	)
}

func newManagerWithProtector(
	storePath string,
	credentialStore credential.Store,
	protectData func([]byte) ([]byte, error),
	unprotectData func([]byte) ([]byte, error),
) (*Manager, error) {
	if credentialStore == nil {
		return nil, fmt.Errorf("credential store is required")
	}
	if protectData == nil || unprotectData == nil {
		return nil, fmt.Errorf("endpoint-cache protector is required")
	}
	manager := &Manager{
		fetcher:         NewFetcher(),
		normalizer:      NewNormalizer(),
		subs:            make([]Subscription, 0),
		outbounds:       make([]compiler.Outbound, 0),
		storePath:       storePath,
		credentialStore: credentialStore,
		protectData:     protectData,
		unprotectData:   unprotectData,
	}
	manager.load()
	if manager.loadErr != nil {
		return nil, manager.loadErr
	}
	return manager, nil
}

// Add adds a new subscription URL.
// URL validation is deferred to fetch time so the user can add and see errors later.
func (m *Manager) Add(name, rawURL string) (*Subscription, error) {
	return m.AddWithOptions(name, rawURL, false)
}

// AddWithOptions adds a subscription with provider-scoped TLS compatibility.
func (m *Manager) AddWithOptions(
	name string,
	rawURL string,
	skipTLSVerify bool,
) (*Subscription, error) {
	name = strings.TrimSpace(name)
	rawURL = strings.TrimSpace(rawURL)
	if name == "" {
		return nil, fmt.Errorf("subscription provider name is required")
	}
	if rawURL == "" {
		return nil, fmt.Errorf("subscription URL is required")
	}
	if err := validateURL(rawURL); err != nil {
		return nil, fmt.Errorf("invalid subscription URL: %w", err)
	}
	urlHash := hashURL(rawURL)
	urlRef, err := m.credentialStore.Put(context.Background(), []byte(rawURL))
	if err != nil {
		return nil, fmt.Errorf("protect subscription URL: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.subs {
		if existing.URLHash == urlHash {
			_ = m.credentialStore.Delete(context.Background(), urlRef)
			return nil, fmt.Errorf("subscription URL already exists")
		}
	}

	sub := Subscription{
		ID:               uniqueID(name, m.subs),
		Name:             name,
		URLCredentialRef: urlRef,
		URLHash:          urlHash,
		Enabled:          true,
		SkipTLSVerify:    skipTLSVerify,
		CreatedAt:        time.Now(),
	}
	m.subs = append(m.subs, sub)
	if err := m.saveLocked(); err != nil {
		m.subs = m.subs[:len(m.subs)-1]
		_ = m.credentialStore.Delete(context.Background(), urlRef)
		return nil, err
	}
	return &sub, nil
}

// UpdateTLSCompatibility changes certificate verification for one provider.
func (m *Manager) UpdateTLSCompatibility(
	id string,
	skipTLSVerify bool,
) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.subs {
		if m.subs[i].ID != id {
			continue
		}
		previous := m.subs[i]
		m.subs[i].SkipTLSVerify = skipTLSVerify
		if err := m.saveLocked(); err != nil {
			m.subs[i] = previous
			return nil, err
		}
		updated := m.subs[i]
		return &updated, nil
	}
	return nil, fmt.Errorf("subscription %q not found", id)
}

// Remove removes a subscription and every node owned by it.
func (m *Manager) Remove(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, s := range m.subs {
		if s.ID == id {
			previousSubs := append([]Subscription(nil), m.subs...)
			previousOutbounds := append([]compiler.Outbound(nil), m.outbounds...)
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			filtered := m.outbounds[:0]
			for _, outbound := range m.outbounds {
				if outbound.ProviderID != id {
					filtered = append(filtered, outbound)
				}
			}
			m.outbounds = filtered
			if err := m.saveLocked(); err != nil {
				m.subs = previousSubs
				m.outbounds = previousOutbounds
				return false, err
			}
			if s.URLCredentialRef != "" {
				if err := m.credentialStore.Delete(context.Background(), s.URLCredentialRef); err != nil {
					m.subs = previousSubs
					m.outbounds = previousOutbounds
					rollbackErr := m.saveLocked()
					return false, errors.Join(fmt.Errorf("delete subscription credential: %w", err), rollbackErr)
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// List returns all subscriptions.
func (m *Manager) List() []Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Subscription, len(m.subs))
	copy(result, m.subs)
	return result
}

// Refresh fetches and parses all enabled subscriptions, returning merged outbounds.
func (m *Manager) Refresh(ctx context.Context) ([]compiler.Outbound, error) {
	m.mu.RLock()
	subs := append([]Subscription(nil), m.subs...)
	existingOutbounds := append([]compiler.Outbound(nil), m.outbounds...)
	m.mu.RUnlock()

	allOutbounds := make([]compiler.Outbound, 0)
	updates := make(map[string]Subscription, len(subs))
	successes := 0
	failures := make([]string, 0)

	for _, sub := range subs {
		if !sub.Enabled {
			continue
		}

		urlBytes, resolveErr := m.credentialStore.Resolve(ctx, sub.URLCredentialRef)
		if resolveErr != nil {
			sub.LastError = "subscription credential is unavailable"
			failures = append(failures, sub.LastError)
			updates[sub.ID] = sub
			allOutbounds = append(allOutbounds, outboundsForProvider(existingOutbounds, sub.ID)...)
			continue
		}
		rawURL := string(urlBytes)
		clear(urlBytes)
		data, err := m.fetcher.FetchWithOptions(ctx, rawURL, FetchOptions{
			SkipTLSVerify: sub.SkipTLSVerify,
		})
		if err != nil {
			log.Printf("[subscription] fetch %q failed: %v", sub.Name, err)
			sub.LastError = err.Error()
			failures = append(failures, sub.LastError)
			updates[sub.ID] = sub
			allOutbounds = append(
				allOutbounds,
				outboundsForProvider(existingOutbounds, sub.ID)...,
			)
			continue
		}

		parsed := m.parseContent(data)
		normalized := m.normalizer.Normalize(parsed)
		validated := normalized[:0]
		for i := range normalized {
			if result := compiler.ValidateOutbound(&normalized[i]); result.Valid {
				validated = append(validated, normalized[i])
			} else {
				log.Printf("[subscription] rejected invalid node %q: %s", normalized[i].Name, result.Errors[0].Field)
			}
		}
		normalized = validated
		if len(normalized) == 0 {
			sub.LastError = "subscription contains no supported nodes"
			failures = append(failures, sub.LastError)
			updates[sub.ID] = sub
			allOutbounds = append(
				allOutbounds,
				outboundsForProvider(existingOutbounds, sub.ID)...,
			)
			continue
		}
		for i := range normalized {
			normalized[i].ProviderID = sub.ID
		}
		allOutbounds = append(allOutbounds, normalized...)

		sub.UpdatedAt = time.Now()
		sub.NodeCount = len(normalized)
		sub.LastError = ""
		updates[sub.ID] = sub
		successes++
		log.Printf("[subscription] %s: %d nodes parsed", sub.Name, len(normalized))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	previousSubs := append([]Subscription(nil), m.subs...)
	previousOutbounds := append([]compiler.Outbound(nil), m.outbounds...)
	for i := range m.subs {
		if updated, ok := updates[m.subs[i].ID]; ok {
			m.subs[i] = updated
		}
	}
	m.outbounds = m.normalizer.Merge(m.outbounds, allOutbounds)
	if err := m.saveLocked(); err != nil {
		m.subs = previousSubs
		m.outbounds = previousOutbounds
		return nil, err
	}
	if successes == 0 && len(updates) > 0 {
		message := "all enabled subscriptions failed to refresh"
		if len(failures) > 0 {
			message = failures[0]
		}
		return append([]compiler.Outbound(nil), m.outbounds...),
			fmt.Errorf("subscription refresh failed: %s", message)
	}
	return m.outbounds, nil
}

func outboundsForProvider(
	outbounds []compiler.Outbound,
	providerID string,
) []compiler.Outbound {
	result := make([]compiler.Outbound, 0)
	for _, outbound := range outbounds {
		if outbound.ProviderID == providerID || outbound.ProviderID == "" {
			result = append(result, outbound)
		}
	}
	return result
}

// parseContent tries each parser and returns the combined results.
func (m *Manager) parseContent(data []byte) []compiler.Outbound {
	data = bytes.TrimSpace(data)
	clashParser := parser.NewClashParser()
	if clashParser.Supports(data) {
		result, err := clashParser.Parse(data)
		if err != nil {
			log.Printf("[subscription] Clash parser error: %v", err)
			return nil
		}
		return result.Outbounds
	}

	data = decodeSubscriptionBody(data)
	if clashParser.Supports(data) {
		result, err := clashParser.Parse(data)
		if err != nil {
			log.Printf("[subscription] decoded Clash parser error: %v", err)
			return nil
		}
		return result.Outbounds
	}
	parsers := []parser.Parser{
		parser.NewVMessParser(),
		parser.NewVLESSParser(),
		parser.NewTrojanParser(),
		parser.NewSSParser(),
		parser.NewSOCKSParser(),
	}

	outbounds := make([]compiler.Outbound, 0)
	for _, p := range parsers {
		result, err := p.Parse(data)
		if err != nil {
			log.Printf("[subscription] parser error: %v", err)
			continue
		}
		outbounds = append(outbounds, result.Outbounds...)
	}

	return outbounds
}

func decodeSubscriptionBody(data []byte) []byte {
	if bytes.Contains(data, []byte("://")) {
		return data
	}
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, string(data))
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(compact)
		if err == nil && bytes.Contains(decoded, []byte("://")) {
			return bytes.TrimSpace(decoded)
		}
	}
	return data
}

// AddOutbound adds a manually created outbound without a subscription URL.
func (m *Manager) AddOutbound(o compiler.Outbound) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o.Enabled = true
	if o.ProviderID == "" {
		o.ProviderID = "manual"
	}
	for _, existing := range m.outbounds {
		if existing.ID == o.ID {
			return fmt.Errorf("outbound %q already exists", o.ID)
		}
	}
	m.outbounds = append(m.outbounds, o)
	if err := m.saveLocked(); err != nil {
		m.outbounds = m.outbounds[:len(m.outbounds)-1]
		return err
	}
	return nil
}

// RemoveOutbound removes an outbound by ID.
func (m *Manager) RemoveOutbound(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, o := range m.outbounds {
		if o.ID == id {
			m.outbounds = append(m.outbounds[:i], m.outbounds[i+1:]...)
			_ = m.saveLocked()
			return true
		}
	}
	return false
}

// Outbounds returns current parsed outbounds.
func (m *Manager) Outbounds() []compiler.Outbound {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]compiler.Outbound(nil), m.outbounds...)
}

// StartAutoRefresh periodically refreshes subscriptions.
func (m *Manager) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("[subscription] auto-refreshing...")
				m.Refresh(ctx)
			}
		}
	}()
}

func sanitizeID(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		} else if r == ' ' || r == '_' {
			result += "-"
		}
	}
	if result == "" {
		return fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}
	return result
}

func uniqueID(name string, existing []Subscription) string {
	base := sanitizeID(name)
	candidate := base
	for suffix := 2; ; suffix++ {
		found := false
		for _, sub := range existing {
			if sub.ID == candidate {
				found = true
				break
			}
		}
		if !found {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func (m *Manager) load() {
	if m.storePath == "" {
		return
	}
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var state persistedState
	if len(data) > 0 && data[0] == '[' {
		if err := json.Unmarshal(data, &state.Subscriptions); err != nil {
			log.Printf("[subscription] load legacy state failed: %v", err)
			return
		}
	} else if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("[subscription] load state failed: %v", err)
		return
	}
	m.subs = state.Subscriptions
	m.outbounds = state.Outbounds
	migrated := len(state.Outbounds) > 0
	cachePath := m.endpointCachePath()
	if encrypted, err := os.ReadFile(cachePath); err == nil {
		plain, decryptErr := m.unprotectData(encrypted)
		if decryptErr != nil {
			m.loadErr = fmt.Errorf("decrypt subscription endpoint cache: %w", decryptErr)
			return
		}
		if unmarshalErr := json.Unmarshal(plain, &m.outbounds); unmarshalErr != nil {
			clear(plain)
			m.loadErr = fmt.Errorf("decode subscription endpoint cache: %w", unmarshalErr)
			return
		}
		clear(plain)
	} else if !errors.Is(err, os.ErrNotExist) {
		m.loadErr = fmt.Errorf("read subscription endpoint cache: %w", err)
		return
	}
	for index := range m.subs {
		if m.subs[index].URLCredentialRef == "" && strings.TrimSpace(m.subs[index].URL) != "" {
			reference, err := m.credentialStore.Put(context.Background(), []byte(m.subs[index].URL))
			if err != nil {
				m.loadErr = fmt.Errorf("migrate legacy subscription URL: %w", err)
				return
			}
			m.subs[index].URLCredentialRef = reference
			m.subs[index].URLHash = hashURL(m.subs[index].URL)
			m.subs[index].URL = ""
			migrated = true
		}
	}
	if migrated {
		m.loadErr = m.saveLocked()
	}
}

func (m *Manager) saveLocked() error {
	if m.storePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0700); err != nil {
		return fmt.Errorf("create subscription state directory: %w", err)
	}
	if err := m.saveEndpointCacheLocked(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(persistedMetadata{
		Subscriptions: m.subs,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode subscriptions: %w", err)
	}
	if err := writeAtomic(m.storePath, data); err != nil {
		return fmt.Errorf("save subscriptions: %w", err)
	}
	return nil
}

func (m *Manager) saveEndpointCacheLocked() error {
	cachePath := m.endpointCachePath()
	if len(m.outbounds) == 0 {
		if err := os.Remove(cachePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove empty subscription endpoint cache: %w", err)
		}
		return nil
	}
	plain, err := json.Marshal(m.outbounds)
	if err != nil {
		return fmt.Errorf("encode subscription endpoint cache: %w", err)
	}
	defer clear(plain)
	encrypted, err := m.protectData(plain)
	if err != nil {
		return fmt.Errorf("encrypt subscription endpoint cache: %w", err)
	}
	defer clear(encrypted)
	if err := writeAtomic(cachePath, encrypted); err != nil {
		return fmt.Errorf("save subscription endpoint cache: %w", err)
	}
	return nil
}

func (m *Manager) endpointCachePath() string {
	return m.storePath + ".endpoints.dpapi"
}

func writeAtomic(path string, data []byte) error {
	return fsatomic.WriteFile(path, data, 0600)
}

func hashURL(value string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return fmt.Sprintf("%x", hash[:])
}
