package logstore

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreQueryFiltersAndPaginates(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "structured.jsonl"), 20)
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	for _, entry := range []Entry{
		{Timestamp: base, Level: LevelInfo, Service: "Service", Message: "started"},
		{Timestamp: base.Add(time.Minute), Level: LevelWarn, Service: "TUN", Message: "retry"},
		{Timestamp: base.Add(2 * time.Minute), Level: LevelError, Service: "TUN", Message: "failed"},
	} {
		if err := store.Append(entry); err != nil {
			t.Fatal(err)
		}
	}
	result := store.Query(Query{
		Levels: []Level{LevelWarn, LevelError}, Services: []string{"tun"},
		From: base.Add(30 * time.Second), To: base.Add(3 * time.Minute), Limit: 1,
	})
	if len(result.Entries) != 1 || !result.HasMore || result.Entries[0].Message != "retry" {
		t.Fatalf("result = %#v", result)
	}
	next := store.Query(Query{AfterID: result.NextCursor, Services: []string{"TUN"}, Limit: 10})
	if len(next.Entries) != 1 || next.Entries[0].Message != "failed" {
		t.Fatalf("next = %#v", next)
	}
}

func TestStoreRedactsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "structured.jsonl")
	store := New(path, 20)
	if err := store.Append(Entry{
		Level: LevelError, Service: "Subscription",
		Message: "url=https://example.test/sub?token=secret authorization=BearerSecret uuid=550e8400-e29b-41d4-a716-446655440000",
		Fields:  map[string]any{"password": "secret", "endpoint": "https://e.test/?key=secret"},
	}); err != nil {
		t.Fatal(err)
	}
	loaded := New(path, 20)
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	entry := loaded.Query(Query{}).Entries[0]
	encoded := entry.Message + entry.Fields["password"].(string) + entry.Fields["endpoint"].(string)
	if strings.Contains(encoded, "secret") || strings.Contains(encoded, "550e8400-e29b-41d4-a716-446655440000") {
		t.Fatalf("sensitive data persisted: %s", encoded)
	}
}

func TestStoreClearKeepsWritableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "structured.jsonl")
	store := New(path, 20)
	if err := store.Append(Entry{Service: "Service", Message: "before"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if result := store.Query(Query{}); len(result.Entries) != 0 {
		t.Fatalf("entries remain: %#v", result)
	}
	if err := store.Append(Entry{Service: "Service", Message: "after"}); err != nil {
		t.Fatal(err)
	}
}
