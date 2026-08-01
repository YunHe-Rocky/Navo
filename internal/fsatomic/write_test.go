package fsatomic

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "value.json")
	if err := WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want second", data)
	}
}

func TestWriteFileReplaceFailurePreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("committed"), 0600); err != nil {
		t.Fatal(err)
	}
	originalReplace := replace
	replace = func(string, string) error { return errors.New("injected replacement failure") }
	t.Cleanup(func() { replace = originalReplace })

	if err := WriteFile(path, []byte("candidate"), 0600); err == nil {
		t.Fatal("expected replacement failure")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "committed" {
		t.Fatalf("original content lost: %q", data)
	}
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	if err := WriteFile("", nil, 0600); err == nil {
		t.Fatal("empty path accepted")
	}
}
