package logstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"navo/internal/fsatomic"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Category string

const (
	CategoryBasicService       Category = "basic_service"
	CategoryNetworkCapture     Category = "network_capture"
	CategoryCoreRuntime        Category = "core_runtime"
	CategorySubscriptionUpdate Category = "subscription_update"
	CategoryOther              Category = "other"
)

var categories = []Category{
	CategoryBasicService,
	CategoryNetworkCapture,
	CategoryCoreRuntime,
	CategorySubscriptionUpdate,
	CategoryOther,
}

type Entry struct {
	ID        uint64         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Level     Level          `json:"level"`
	Category  Category       `json:"category"`
	Service   string         `json:"service"`
	Component string         `json:"component,omitempty"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type Query struct {
	Levels     []Level
	Categories []Category
	Services   []string
	From       time.Time
	To         time.Time
	AfterID    uint64
	Limit      int
}

type Result struct {
	Entries    []Entry `json:"entries"`
	NextCursor uint64  `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type Store struct {
	mu         sync.RWMutex
	path       string
	entries    []Entry
	maxEntries int
	maxBytes   int64
	nextID     atomic.Uint64
}

var configuredStore atomic.Pointer[Store]

func init() { configuredStore.Store(New("", 2000)) }

func Default() *Store { return configuredStore.Load() }

func Configure(path string) error {
	store := New(path, 2000)
	if err := store.Load(); err != nil {
		return err
	}
	configuredStore.Store(store)
	return nil
}

func New(path string, maxEntries int) *Store {
	if maxEntries < 1 {
		maxEntries = 2000
	}
	return &Store{path: path, maxEntries: maxEntries, maxBytes: 5 * 1024 * 1024}
}

func Emit(level Level, service, component, message string, fields map[string]any) error {
	return Default().Append(Entry{
		Level: level, Service: service, Component: component,
		Message: message, Fields: fields,
	})
}

func (s *Store) Append(entry Entry) error {
	entry.Timestamp = entry.Timestamp.UTC()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.Level == "" {
		entry.Level = LevelInfo
	}
	entry.Service = strings.TrimSpace(entry.Service)
	if entry.Service == "" || strings.TrimSpace(entry.Message) == "" {
		return fmt.Errorf("structured log service and message are required")
	}
	entry.Category = normalizeCategory(entry.Category, entry.Service)
	entry.ID = s.nextID.Add(1)
	entry.Message = redact(entry.Message)
	entry.Fields = redactFields(entry.Fields)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if excess := len(s.entries) - s.maxEntries; excess > 0 {
		s.entries = append([]Entry(nil), s.entries[excess:]...)
	}
	if s.path == "" {
		return nil
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(s.path); statErr == nil && info.Size()+int64(len(encoded)+1) > s.maxBytes {
		return s.rewriteLocked()
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		if err := fsatomic.WriteFile(s.path, nil, 0600); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (s *Store) Query(query Query) Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := query.Limit
	if limit < 1 || limit > 500 {
		limit = 200
	}
	levels := make(map[Level]struct{}, len(query.Levels))
	for _, level := range query.Levels {
		levels[level] = struct{}{}
	}
	categories := make(map[Category]struct{}, len(query.Categories))
	for _, category := range query.Categories {
		categories[category] = struct{}{}
	}
	services := make(map[string]struct{}, len(query.Services))
	for _, service := range query.Services {
		services[strings.ToLower(service)] = struct{}{}
	}
	result := Result{Entries: make([]Entry, 0, limit)}
	for _, entry := range s.entries {
		if entry.ID <= query.AfterID || (!query.From.IsZero() && entry.Timestamp.Before(query.From)) ||
			(!query.To.IsZero() && entry.Timestamp.After(query.To)) {
			continue
		}
		if len(levels) > 0 {
			if _, ok := levels[entry.Level]; !ok {
				continue
			}
		}
		if len(categories) > 0 {
			if _, ok := categories[entry.Category]; !ok {
				continue
			}
		}
		if len(services) > 0 {
			if _, ok := services[strings.ToLower(entry.Service)]; !ok {
				continue
			}
		}
		if len(result.Entries) == limit {
			result.HasMore = true
			break
		}
		result.Entries = append(result.Entries, entry)
		result.NextCursor = entry.ID
	}
	return result
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	if s.path == "" {
		return nil
	}
	return fsatomic.WriteFile(s.path, nil, 0600)
}

func (s *Store) Load() error {
	if s.path == "" {
		return nil
	}
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	loaded := make([]Entry, 0, s.maxEntries)
	var maxID uint64
	for scanner.Scan() {
		var entry Entry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil || entry.ID == 0 {
			continue
		}
		entry.Service = strings.TrimSpace(entry.Service)
		if entry.Service == "" {
			continue
		}
		entry.Category = normalizeCategory(entry.Category, entry.Service)
		loaded = append(loaded, entry)
		if entry.ID > maxID {
			maxID = entry.ID
		}
		if len(loaded) > s.maxEntries {
			loaded = loaded[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.entries = loaded
	s.nextID.Store(maxID)
	s.mu.Unlock()
	return nil
}

func Categories() []Category {
	return append([]Category(nil), categories...)
}

func ParseCategory(value string) (Category, bool) {
	category := Category(strings.ToLower(strings.TrimSpace(value)))
	for _, known := range categories {
		if category == known {
			return category, true
		}
	}
	return "", false
}

func normalizeCategory(category Category, service string) Category {
	if parsed, ok := ParseCategory(string(category)); ok {
		return parsed
	}
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "launcher", "ui", "agent", "service", "selfheal":
		return CategoryBasicService
	case "capture", "tun", "systemproxy", "networkmonitor", "ipdetection":
		return CategoryNetworkCapture
	case "supervisor", "sing-box", "mihomo", "xray", "coreupdate":
		return CategoryCoreRuntime
	case "subscription":
		return CategorySubscriptionUpdate
	default:
		return CategoryOther
	}
}

func (s *Store) Services() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make(map[string]struct{})
	for _, entry := range s.entries {
		values[entry.Service] = struct{}{}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Store) rewriteLocked() error {
	data := make([]byte, 0, len(s.entries)*128)
	for _, entry := range s.entries {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data = append(data, encoded...)
		data = append(data, '\n')
	}
	return fsatomic.WriteFile(s.path, data, 0600)
}

var (
	secretPattern      = regexp.MustCompile(`(?i)(authorization|cookie|password|passwd|token|api[_-]?key)(\s*[:=]\s*)[^\s,;&]+`)
	querySecretPattern = regexp.MustCompile(`(?i)([?&](?:token|key|auth|password)=)[^&\s]+`)
	uuidPattern        = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
)

func redact(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1$2[REDACTED]")
	value = querySecretPattern.ReplaceAllString(value, "$1[REDACTED]")
	return uuidPattern.ReplaceAllStringFunc(value, func(uuid string) string {
		return uuid[:8] + "-[REDACTED]"
	})
}

func redactFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	result := make(map[string]any, len(fields))
	for key, value := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") ||
			strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") ||
			strings.Contains(lower, "api_key") {
			result[key] = "[REDACTED]"
			continue
		}
		if text, ok := value.(string); ok {
			result[key] = redact(text)
		} else {
			result[key] = value
		}
	}
	return result
}
