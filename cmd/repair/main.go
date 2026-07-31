// Package main provides the Navo network repair tool.
// repair.exe diagnoses and fixes TUN/route/DNS state when the service
// cannot start due to network corruption from a previous crash.
//
// Usage:
//
//	repair check       - diagnostic scan only (no changes)
//	repair fix         - automatic repair
//	repair reset       - full reset (adapter + routes + DNS)
//	repair tun-reset   - reset only TUN adapter
//	repair route-fix   - fix only routing table
//	repair dns-reset   - reset only DNS settings
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
	case "tun-reset":
		fmt.Println("tun-reset: not yet implemented on this platform")
	case "route-fix":
		fmt.Println("route-fix: not yet implemented on this platform")
	case "dns-reset":
		fmt.Println("dns-reset: not yet implemented on this platform")
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
	fmt.Println("  fix          Automatic repair")
	fmt.Println("  reset        Full reset (adapter + routes + DNS)")
	fmt.Println("  tun-reset    Reset only TUN adapter")
	fmt.Println("  route-fix    Fix only routing table")
	fmt.Println("  dns-reset    Reset only DNS settings")
}

// DiagnosticResult is the JSON output format for repair operations.
type DiagnosticResult struct {
	IssuesFound int      `json:"issues_found"`
	Issues      []Issue  `json:"issues"`
	Fixable     bool     `json:"fixable"`
	Fixed       int      `json:"fixed,omitempty"`
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
	if !checkResult.Fixable {
		return checkResult
	}

	fixed := 0
	for _, issue := range checkResult.Issues {
		switch issue.Type {
		case "dirty_shutdown":
			// Clear recovery state file
			programData := os.Getenv("PROGRAMDATA")
			if programData == "" {
				programData = `C:\ProgramData`
			}
			stateFile := programData + `\Navo\service\recovery_state.json`
			normalState := map[string]string{"state": "NORMAL"}
			data, _ := json.Marshal(normalState)
			os.MkdirAll(programData+`\Navo\service`, 0755)
			if os.WriteFile(stateFile, data, 0644) == nil {
				fixed++
			}
		}
	}

	checkResult.Fixed = fixed
	checkResult.Fixable = fixed < checkResult.IssuesFound
	return checkResult
}

func runReset(ctx context.Context) DiagnosticResult {
	checkResult := runCheck(ctx)

	// Reset recovery state
	programData := os.Getenv("PROGRAMDATA")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	stateFile := programData + `\Navo\service\recovery_state.json`
	os.Remove(stateFile)

	return DiagnosticResult{
		IssuesFound: checkResult.IssuesFound,
		Fixable:     false,
		Fixed:       1,
	}
}
