package network

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestObserveMissingJournalIsCleanAndReadOnly(t *testing.T) {
	cfg := testConfig(t)
	executor := &fakeExecutor{}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if observation.Journal.Present || observation.Journal.Dirty || !observation.Routes.Coherent {
		t.Fatalf("observation = %+v", observation)
	}
	if len(executor.commands) != 0 {
		t.Fatalf("missing journal executed commands: %+v", executor.commands)
	}
}

func TestObserveExactOwnedResourceReportsCoherentWithoutMutation(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, true, actionApplied)
	executor := &fakeExecutor{inspectionState: "EXACT"}
	manager, err := NewManager(cfg, executor, fakePlatform{})
	if err != nil {
		t.Fatal(err)
	}

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !observation.Journal.Present || observation.Journal.Dirty ||
		observation.Routes.OwnedCount != 1 || observation.Routes.ExistingCount != 1 ||
		!observation.Routes.Coherent {
		t.Fatalf("observation = %+v", observation)
	}
	assertObservationCommandsReadOnly(t, executor.commands)
}

func TestObserveMissingOwnedResourceProducesRecoverableEvidenceOnly(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, true, actionApplied)
	executor := &fakeExecutor{inspectionState: "MISSING"}
	manager, _ := NewManager(cfg, executor, fakePlatform{})

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !observation.Journal.Dirty || observation.Journal.MissingResources != 1 ||
		observation.Journal.ConflictingResources != 0 || observation.Routes.MissingCount != 1 ||
		observation.Routes.Coherent {
		t.Fatalf("observation = %+v", observation)
	}
	assertObservationCommandsReadOnly(t, executor.commands)
}

func TestObserveChangedPreexistingResourceFailsClosed(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, false, actionApplied)
	executor := &fakeExecutor{inspectionState: "MISSING"}
	manager, _ := NewManager(cfg, executor, fakePlatform{})

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if observation.Journal.OwnedResources != 0 || observation.Journal.PreexistingResources != 1 ||
		observation.Journal.ConflictingResources != 1 || observation.Routes.MissingCount != 0 {
		t.Fatalf("pre-existing state was attributed to Navo: %+v", observation)
	}
	assertObservationCommandsReadOnly(t, executor.commands)
}

func TestObserveBatchesJournalResourcesIntoOneReadOnlyCommand(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, true, actionApplied)
	value, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"128.0.0.0/1", "203.0.113.1/32", "198.51.100.1/32"} {
		action := value.Actions[0]
		action.Name = "IPv4 route " + prefix
		action.Resource.DestinationPrefix = prefix
		value.Actions = append(value.Actions, action)
	}
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{inspectionStates: []string{"EXACT", "EXACT", "EXACT", "MISSING"}}
	manager, _ := NewManager(cfg, executor, fakePlatform{})

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.commands) != 1 {
		t.Fatalf("four resources executed %d commands, want one batch: %+v", len(executor.commands), executor.commands)
	}
	if observation.Routes.ExistingCount != 3 || observation.Routes.MissingCount != 1 ||
		observation.Journal.MissingResources != 1 || observation.Routes.Coherent {
		t.Fatalf("mixed batch observation = %+v", observation)
	}
	script := executor.commands[0].Args[len(executor.commands[0].Args)-1]
	if strings.Count(script, "NAVO_NETWORK_OBSERVATION_ITEM") != 4 {
		t.Fatalf("batch item count mismatch: %s", script)
	}
	assertObservationCommandsReadOnly(t, executor.commands)
}

func TestObserveBatchItemErrorFailsClosedWithoutDroppingHealthyResult(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, true, actionApplied)
	value, err := readJournal(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	second := value.Actions[0]
	second.Name = "IPv4 route 128.0.0.0/1"
	second.Resource.DestinationPrefix = "128.0.0.0/1"
	value.Actions = append(value.Actions, second)
	if err := writeJournal(cfg.JournalPath, value); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{observationOutput: `{"results":[{"index":0,"state":"EXACT"},{"index":1,"error":"query failed"}]}`}
	manager, _ := NewManager(cfg, executor, fakePlatform{})

	observation, err := manager.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.commands) != 1 || observation.Routes.ExistingCount != 1 ||
		observation.Routes.ConflictCount != 1 || observation.Journal.ConflictingResources != 1 ||
		len(observation.Errors) != 1 || !strings.Contains(observation.Errors[0], "query failed") {
		t.Fatalf("partial batch error was not attributed exactly: %+v", observation)
	}
}

func TestDecodeObservationBatchRejectsAmbiguousResults(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "not json", output: `not-json`},
		{name: "missing result", output: `{"results":[{"index":0,"state":"EXACT"}]}`},
		{name: "duplicate index", output: `{"results":[{"index":0,"state":"EXACT"},{"index":0,"state":"MISSING"}]}`},
		{name: "invalid index", output: `{"results":[{"index":0,"state":"EXACT"},{"index":2,"state":"MISSING"}]}`},
		{name: "invalid state", output: `{"results":[{"index":0,"state":"UNKNOWN"},{"index":1,"state":"EXACT"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeObservationBatch(test.output, 2); err == nil {
				t.Fatalf("ambiguous batch output was accepted: %s", test.output)
			}
		})
	}
}

func TestObserveHonorsCancellation(t *testing.T) {
	cfg := testConfig(t)
	writeObservationJournal(t, cfg.JournalPath, true, actionApplied)
	manager, _ := NewManager(cfg, &fakeExecutor{}, fakePlatform{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Observe(ctx); err == nil {
		t.Fatal("canceled observation unexpectedly succeeded")
	}
}

func writeObservationJournal(t *testing.T, path string, createdByNavo bool, status actionStatus) {
	t.Helper()
	value := &journal{
		Version: 2, AdapterName: OwnedTUNAdapterName, SessionID: "observation-session",
		CreatedAt: time.Now().UTC(), Adapter: testAdapter(),
		Actions: []journalAction{{
			Name: "IPv4 route 0.0.0.0/1", Status: status,
			Resource: journalResource{
				Kind: resourceSplitRoute, DestinationPrefix: "0.0.0.0/1", AddressFamily: "IPv4",
				InterfaceIndex: testAdapter().InterfaceIndex, InterfaceGUID: testAdapter().InterfaceGUID,
				InterfaceAlias: OwnedTUNAdapterName, NextHop: "172.19.0.2", RouteMetric: 1,
				CreatedByNavo: createdByNavo,
			},
		}},
	}
	if err := writeJournal(path, value); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func assertObservationCommandsReadOnly(t *testing.T, commands []Command) {
	t.Helper()
	if len(commands) == 0 {
		t.Fatal("resource observation did not execute a query")
	}
	for _, command := range commands {
		script := command.Args[len(command.Args)-1]
		for _, mutation := range []string{"New-NetRoute", "Remove-NetRoute", "Add-DnsClientNrptRule", "Remove-DnsClientNrptRule", "New-NetFirewallRule", "Remove-NetFirewallRule"} {
			if strings.Contains(script, mutation) {
				t.Fatalf("observation executed mutation %q: %s", mutation, script)
			}
		}
	}
}
