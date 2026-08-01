package localstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"navo/internal/domain/revision"
	"navo/internal/domain/selection"
	"navo/internal/fsatomic"
)

const stateVersion = 1

type persistedState struct {
	Version        int                          `json:"version"`
	Selection      *selection.ActiveSelection   `json:"selection,omitempty"`
	Revisions      map[string]revision.Revision `json:"revisions"`
	ActiveRevision string                       `json:"active_revision,omitempty"`
}

type store struct {
	mu    sync.Mutex
	path  string
	state persistedState
}

type SelectionRepository struct{ store *store }
type RevisionRepository struct{ store *store }

var _ selection.Repository = (*SelectionRepository)(nil)
var _ revision.Repository = (*RevisionRepository)(nil)

func Open(path string) (*SelectionRepository, *RevisionRepository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil, fmt.Errorf("local repository path is empty")
	}
	s := &store{
		path: path,
		state: persistedState{
			Version:   stateVersion,
			Revisions: make(map[string]revision.Revision),
		},
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read local repository: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, nil, fmt.Errorf("decode local repository: %w", err)
		}
		if s.state.Version != stateVersion {
			return nil, nil, fmt.Errorf("unsupported local repository version %d", s.state.Version)
		}
		if s.state.Revisions == nil {
			s.state.Revisions = make(map[string]revision.Revision)
		}
		if s.state.Selection != nil {
			if err := s.state.Selection.Validate(); err != nil {
				return nil, nil, fmt.Errorf("validate stored selection: %w", err)
			}
		}
	}
	return &SelectionRepository{store: s}, &RevisionRepository{store: s}, nil
}

func (r *SelectionRepository) Load(ctx context.Context) (selection.ActiveSelection, error) {
	if err := ctx.Err(); err != nil {
		return selection.ActiveSelection{}, err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if r.store.state.Selection == nil {
		return selection.ActiveSelection{}, selection.ErrNotFound
	}
	return *r.store.state.Selection, nil
}

func (r *SelectionRepository) Save(ctx context.Context, value selection.ActiveSelection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate active selection: %w", err)
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = time.Now().UTC()
	} else {
		value.UpdatedAt = value.UpdatedAt.UTC()
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	previous := r.store.state.Selection
	r.store.state.Selection = &value
	if err := r.store.persistLocked(); err != nil {
		r.store.state.Selection = previous
		return err
	}
	return nil
}

func (r *RevisionRepository) SaveCandidate(ctx context.Context, value revision.Revision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateRevision(value); err != nil {
		return err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	previous, existed := r.store.state.Revisions[value.ID]
	r.store.state.Revisions[value.ID] = value
	if err := r.store.persistLocked(); err != nil {
		if existed {
			r.store.state.Revisions[value.ID] = previous
		} else {
			delete(r.store.state.Revisions, value.ID)
		}
		return err
	}
	return nil
}

func (r *RevisionRepository) MarkActive(ctx context.Context, revisionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	value, ok := r.store.state.Revisions[revisionID]
	if !ok {
		return fmt.Errorf("candidate revision %q not found", revisionID)
	}
	previousState := r.store.state
	previousState.Revisions = cloneRevisions(r.store.state.Revisions)
	for id, candidate := range r.store.state.Revisions {
		if id == revisionID {
			continue
		}
		if candidate.StartupStatus == revision.StatusActive || candidate.HealthStatus == revision.StatusActive {
			candidate.StartupStatus = revision.StatusCandidate
			candidate.HealthStatus = revision.StatusCandidate
			r.store.state.Revisions[id] = candidate
		}
	}
	value.StartupStatus = revision.StatusActive
	value.HealthStatus = revision.StatusActive
	r.store.state.Revisions[revisionID] = value
	r.store.state.ActiveRevision = revisionID
	if err := r.store.persistLocked(); err != nil {
		r.store.state = previousState
		return err
	}
	return nil
}

func (r *RevisionRepository) MarkFailed(ctx context.Context, revisionID, stage string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	value, ok := r.store.state.Revisions[revisionID]
	if !ok {
		return fmt.Errorf("candidate revision %q not found", revisionID)
	}
	previous := value
	switch stage {
	case "validation":
		value.ValidationStatus = revision.StatusFailed
	case "startup":
		value.StartupStatus = revision.StatusFailed
	case "health":
		value.HealthStatus = revision.StatusFailed
	default:
		return fmt.Errorf("invalid revision failure stage %q", stage)
	}
	r.store.state.Revisions[revisionID] = value
	if r.store.state.ActiveRevision == revisionID {
		r.store.state.ActiveRevision = ""
	}
	if err := r.store.persistLocked(); err != nil {
		r.store.state.Revisions[revisionID] = previous
		return err
	}
	return nil
}

func (s *store) persistLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local repository: %w", err)
	}
	if err := fsatomic.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("persist local repository: %w", err)
	}
	return nil
}

func validateRevision(value revision.Revision) error {
	if strings.TrimSpace(value.ID) == "" {
		return fmt.Errorf("revision ID is empty")
	}
	if !value.CoreType.Valid() {
		return fmt.Errorf("revision core type %q is invalid", value.CoreType)
	}
	if !value.SourceType.Valid() {
		return fmt.Errorf("revision source type %q is invalid", value.SourceType)
	}
	if value.CreatedAt.IsZero() {
		return fmt.Errorf("revision creation time is empty")
	}
	if strings.TrimSpace(value.ConfigHash) == "" || strings.TrimSpace(value.ConfigPath) == "" {
		return fmt.Errorf("revision config identity is incomplete")
	}
	return nil
}

func cloneRevisions(input map[string]revision.Revision) map[string]revision.Revision {
	result := make(map[string]revision.Revision, len(input))
	for id, value := range input {
		result[id] = value
	}
	return result
}
