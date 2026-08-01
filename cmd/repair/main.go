// Package main provides offline diagnostics. Network mutation is deliberately
// owned by the runtime Reconciler and is never reconstructed here.
//
// Usage:
//
//	repair check       - diagnostic scan only (no changes)
//	repair fix         - read-only recovery report; runtime owns mutation
//	repair reset       - read-only full-recovery report; runtime owns mutation
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()
	cmd := os.Args[1]

	switch cmd {
	case "check":
		result := runCheck(ctx)
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	case "fix":
		result := runFix(ctx)
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	case "reset":
		result := runReset(ctx)
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	default:
		fmt.Printf("unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Navo Repair Tool")
	fmt.Println()
	fmt.Println("Usage: repair <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  check        Diagnostic scan (read-only)")
	fmt.Println("  fix          Read-only recovery report (start Navo to reconcile)")
	fmt.Println("  reset        Read-only full-recovery report (start Navo to reconcile)")
}

// DiagnosticResult is the JSON output format for repair operations.
type DiagnosticResult struct {
	IssuesFound int     `json:"issues_found"`
	Issues      []Issue `json:"issues"`
	Fixable     bool    `json:"fixable"`
	Fixed       int     `json:"fixed,omitempty"`
}

// Issue represents a single diagnostic finding.
type Issue struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Detail   string `json:"detail"`
}

func runCheck(ctx context.Context) DiagnosticResult {
	issues := []Issue{}

	// Check for wintun.dll
	if _, err := os.Stat("third_party/wintun/wintun.dll"); os.IsNotExist(err) {
		issues = append(issues, Issue{
			Severity: "error",
			Type:     "wintun_missing",
			Detail:   "wintun.dll not found at third_party/wintun/wintun.dll",
		})
	}

	// Check for sing-box.exe
	if _, err := os.Stat("third_party/sing-box/sing-box.exe"); os.IsNotExist(err) {
		issues = append(issues, Issue{
			Severity: "warning",
			Type:     "singbox_missing",
			Detail:   "sing-box.exe not found at third_party/sing-box/sing-box.exe",
		})
	}

	// Check recovery state
	programData := os.Getenv("PROGRAMDATA")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	stateFile := programData + `\Navo\service\recovery_state.json`
	if data, err := os.ReadFile(stateFile); err == nil {
		var state map[string]interface{}
		if json.Unmarshal(data, &state) == nil {
			if s, ok := state["state"].(string); ok && s == "DIRTY_SHUTDOWN" {
				issues = append(issues, Issue{
					Severity: "warning",
					Type:     "dirty_shutdown",
					Detail:   "Previous session did not shut down cleanly",
				})
			}
		}
	}

	result := DiagnosticResult{
		IssuesFound: len(issues),
		Issues:      issues,
		Fixable:     len(issues) > 0,
	}

	if result.IssuesFound == 0 {
		result.Issues = []Issue{}
	}

	return result
}

func runFix(ctx context.Context) DiagnosticResult {
	checkResult := runCheck(ctx)
	for index := range checkResult.Issues {
		if checkResult.Issues[index].Type == "dirty_shutdown" {
			checkResult.Issues[index].Detail += "; start Navo and use its verified network recovery path"
		}
	}
	checkResult.Fixed = 0
	checkResult.Fixable = false
	return checkResult
}

func runReset(ctx context.Context) DiagnosticResult {
	return runFix(ctx)
}
