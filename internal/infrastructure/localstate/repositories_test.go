package localstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/revision"
	"navo/internal/domain/selection"
	"navo/internal/domain/source"
)

func TestRepositoriesPersistSelectionAndRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.json")
	selectionRepo, revisionRepo, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	upstreamID := "proxy-1"
	selected := selection.ActiveSelection{
		CoreType: core.TypeSingBox, SourceType: source.TypeUpstreamProxy,
		CaptureMode: capture.ModeSystemProxy, UpstreamProxyID: &upstreamID,
		UpdatedAt: time.Now(),
	}
	if err := selectionRepo.Save(context.Background(), selected); err != nil {
		t.Fatal(err)
	}
	candidate := revision.Revision{
		ID: "rev-1", CoreType: core.TypeSingBox, SourceType: source.TypeUpstreamProxy,
		EndpointReference: upstreamID, ConfigHash: "abc", ConfigPath: "runtime.json",
		CreatedAt: time.Now(), ValidationStatus: revision.StatusActive,
		StartupStatus: revision.StatusCandidate, HealthStatus: revision.StatusCandidate,
	}
	if err := revisionRepo.SaveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	if err := revisionRepo.MarkActive(context.Background(), candidate.ID); err != nil {
		t.Fatal(err)
	}

	reloadedSelection, _, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloadedSelection.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.UpstreamProxyID == nil || *got.UpstreamProxyID != upstreamID {
		t.Fatalf("selection = %#v", got)
	}
}

func TestOpenRejectsCorruptStateWithoutOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.json")
	original := []byte("{not-json")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(path); err == nil {
		t.Fatal("corrupt repository accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatal("corrupt repository was overwritten")
	}
}
