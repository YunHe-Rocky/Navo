package mysqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"navo/internal/domain/revision"
)

type RevisionRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

var _ revision.Repository = (*RevisionRepository)(nil)

func NewRevisionRepository(db *sql.DB, queryTimeout time.Duration) (*RevisionRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("MySQL database is nil")
	}
	if queryTimeout <= 0 {
		queryTimeout = 10 * time.Second
	}
	return &RevisionRepository{db: db, queryTimeout: queryTimeout}, nil
}

func (r *RevisionRepository) SaveCandidate(ctx context.Context, value revision.Revision) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	_, err := r.db.ExecContext(queryCtx, `
		INSERT INTO core_revisions (
			revision_id, core_type, source_type, endpoint_reference,
			config_hash, config_path, created_at, validation_status,
			startup_status, health_status, is_active, is_last_known_good
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE, FALSE)
		ON DUPLICATE KEY UPDATE
			config_hash = VALUES(config_hash),
			config_path = VALUES(config_path),
			validation_status = VALUES(validation_status),
			startup_status = VALUES(startup_status),
			health_status = VALUES(health_status)
	`,
		value.ID, value.CoreType.String(), value.SourceType.String(),
		value.EndpointReference, value.ConfigHash, value.ConfigPath,
		value.CreatedAt.UTC(), value.ValidationStatus, value.StartupStatus,
		value.HealthStatus,
	)
	if err != nil {
		return fmt.Errorf("save candidate revision: %w", err)
	}
	return nil
}

func (r *RevisionRepository) MarkActive(ctx context.Context, revisionID string) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(queryCtx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin revision activation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(queryCtx, `
		UPDATE core_revisions
		SET is_active = FALSE, is_last_known_good = FALSE
		WHERE is_active = TRUE OR is_last_known_good = TRUE
	`); err != nil {
		return fmt.Errorf("clear previous active revision: %w", err)
	}
	result, err := tx.ExecContext(queryCtx, `
		UPDATE core_revisions
		SET startup_status = 'active', health_status = 'active',
		    is_active = TRUE, is_last_known_good = TRUE
		WHERE revision_id = ?
	`, revisionID)
	if err != nil {
		return fmt.Errorf("activate revision: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read revision activation result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("candidate revision %q not found", revisionID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit revision activation: %w", err)
	}
	return nil
}

func (r *RevisionRepository) MarkFailed(ctx context.Context, revisionID, stage string) error {
	if stage != "validation" && stage != "startup" && stage != "health" {
		return fmt.Errorf("invalid revision failure stage %q", stage)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query := fmt.Sprintf(
		"UPDATE core_revisions SET %s_status = 'failed', is_active = FALSE WHERE revision_id = ?",
		stage,
	)
	if _, err := r.db.ExecContext(queryCtx, query, revisionID); err != nil {
		return fmt.Errorf("mark revision failed: %w", err)
	}
	return nil
}
