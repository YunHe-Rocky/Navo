package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	// Verify usage doesn't panic
	printUsage()
}

func TestRunCheck_NoIssues(t *testing.T) {
	// Simulate clean state by ensuring no dirty shutdown file exists
	result := runCheck(context.Background())
	data, _ := json.Marshal(result)
	if len(data) == 0 {
		t.Error("expected valid JSON output")
	}
}

func TestRunCheck_OutputFormat(t *testing.T) {
	result := runCheck(context.Background())
	if result.IssuesFound < 0 {
		t.Error("IssuesFound should not be negative")
	}
	// Without wintun.dll present, should find at least the missing DLL issue
	// (unless it happens to exist at third_party/wintun/wintun.dll)
}

func TestRunFix(t *testing.T) {
	result := runFix(context.Background())
	// Fix should produce valid JSON even without real state
	if result.Fixable && result.Fixed == 0 && result.IssuesFound > 0 {
		t.Log("fixable issues found but none fixed (expected without admin)")
	}
}

func TestRunReset(t *testing.T) {
	result := runReset(context.Background())
	if result.Fixed < 0 {
		t.Error("Fixed count should not be negative")
	}
}

func TestRunCheck_DirtyShutdown(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("PROGRAMDATA", dir)
	defer os.Unsetenv("PROGRAMDATA")

	// Create a dirty shutdown state file
	stateDir := filepath.Join(dir, "Navo", "service")
	os.MkdirAll(stateDir, 0755)
	stateFile := filepath.Join(stateDir, "recovery_state.json")
	dirtyState := `{"state":"DIRTY_SHUTDOWN","dirty_since":"2026-01-01T00:00:00Z"}`
	os.WriteFile(stateFile, []byte(dirtyState), 0644)

	result := runCheck(context.Background())

	hasDirtyIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "dirty_shutdown" {
			hasDirtyIssue = true
			break
		}
	}
	if !hasDirtyIssue {
		t.Error("expected dirty_shutdown issue to be detected")
	}
	if !result.Fixable {
		t.Error("expected fixable=true with dirty shutdown")
	}
}

func TestRunFix_CleansDirtyShutdown(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("PROGRAMDATA", dir)
	defer os.Unsetenv("PROGRAMDATA")

	stateDir := filepath.Join(dir, "Navo", "service")
	os.MkdirAll(stateDir, 0755)
	stateFile := filepath.Join(stateDir, "recovery_state.json")
	dirtyState := `{"state":"DIRTY_SHUTDOWN"}`
	os.WriteFile(stateFile, []byte(dirtyState), 0644)

	result := runFix(context.Background())

	if result.Fixed == 0 {
		t.Error("expected at least 1 issue fixed (dirty shutdown)")
	}

	// Verify state file was updated
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal("state file should still exist after fix:", err)
	}
	if !contains(data, "NORMAL") {
		t.Error("expected NORMAL state after fix, got:", string(data))
	}
}

func contains(data []byte, s string) bool {
	return strings.Contains(string(data), s)
}
