package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore_InMemory(t *testing.T) {
	s := NewStore("")
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
	if len(s.Keys()) != 0 {
		t.Errorf("Keys = %v, want empty", s.Keys())
	}
}

func TestPutGet(t *testing.T) {
	s := NewStore("")

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := s.Put("key1", TestData{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}

	var result TestData
	err = s.Get("key1", &result)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if result.Name != "test" || result.Value != 42 {
		t.Errorf("result = %+v", result)
	}
}

func TestGet_MissingKey(t *testing.T) {
	s := NewStore("")
	var v interface{}
	err := s.Get("nonexistent", &v)
	if err == nil {
		t.Error("Get should error for missing key")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore("")
	s.Put("key1", "value1")
	s.Delete("key1")

	if s.Count() != 0 {
		t.Errorf("Count = %d after delete, want 0", s.Count())
	}

	var v string
	if err := s.Get("key1", &v); err == nil {
		t.Error("Get should error after delete")
	}
}

func TestKeys(t *testing.T) {
	s := NewStore("")
	s.Put("a", 1)
	s.Put("b", 2)
	s.Put("c", 3)

	keys := s.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys count = %d, want 3", len(keys))
	}
}

func TestCount(t *testing.T) {
	s := NewStore("")
	if s.Count() != 0 {
		t.Error("initial count should be 0")
	}

	s.Put("a", 1)
	s.Put("b", 2)
	if s.Count() != 2 {
		t.Errorf("Count = %d, want 2", s.Count())
	}

	s.Delete("a")
	if s.Count() != 1 {
		t.Errorf("Count = %d, want 1", s.Count())
	}
}

func TestFlushAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "store.json")

	// Create and populate
	s1 := NewStore(path)
	s1.Put("key1", map[string]string{"msg": "hello"})
	s1.Put("key2", 42)

	err := s1.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("store file not found: %v", err)
	}

	// Reload
	s2 := NewStore(path)
	if s2.Count() != 2 {
		t.Errorf("reloaded Count = %d, want 2", s2.Count())
	}

	var msg map[string]string
	s2.Get("key1", &msg)
	if msg["msg"] != "hello" {
		t.Errorf("msg = %v", msg)
	}

	var num int
	s2.Get("key2", &num)
	if num != 42 {
		t.Errorf("num = %d", num)
	}
}

func TestFlush_InMemory(t *testing.T) {
	s := NewStore("")
	s.Put("key", "value")
	err := s.Flush() // Should be no-op
	if err != nil {
		t.Errorf("Flush error on in-memory store: %v", err)
	}
}

func TestStore_JSONTypes(t *testing.T) {
	s := NewStore("")

	tests := []struct {
		key string
		val interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"array", []int{1, 2, 3}},
		{"object", map[string]int{"a": 1}},
	}

	for _, tt := range tests {
		s.Put(tt.key, tt.val)
	}

	// Round-trip each
	for _, tt := range tests {
		var raw json.RawMessage
		if err := s.Get(tt.key, &raw); err != nil {
			t.Errorf("Get(%s) error: %v", tt.key, err)
		}
	}
}

func TestStore_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(path, []byte("not valid json"), 0644)

	s := NewStore(path)
	// Should not panic, should start with empty store
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0 for corrupted file", s.Count())
	}
}
