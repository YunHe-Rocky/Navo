//go:build windows

package network

import (
	"context"
	"strings"
	"testing"
)

func TestObservationBatchPowerShellContract(t *testing.T) {
	items := []preparedResourceObservation{
		{script: "'EXACT'"},
		{script: "throw 'injected read failure'"},
	}
	command := observationBatchCommand(items)
	script := command.Args[len(command.Args)-1]
	for _, mutation := range []string{
		"New-NetRoute", "Remove-NetRoute", "Add-DnsClientNrptRule", "Remove-DnsClientNrptRule",
		"New-NetFirewallRule", "Remove-NetFirewallRule",
	} {
		if strings.Contains(script, mutation) {
			t.Fatalf("batch contract contains mutation %q: %s", mutation, script)
		}
	}

	output, err := NewSystemExecutor().RunOutput(context.Background(), command)
	if err != nil {
		t.Fatalf("execute batch contract: %v", err)
	}
	results, err := decodeObservationBatch(output, len(items))
	if err != nil {
		t.Fatalf("decode batch contract %q: %v", output, err)
	}
	if results[0].State != "EXACT" || results[0].Error != "" {
		t.Fatalf("healthy result = %+v", results[0])
	}
	if results[1].State != "" || !strings.Contains(results[1].Error, "injected read failure") {
		t.Fatalf("failed result = %+v", results[1])
	}
}
