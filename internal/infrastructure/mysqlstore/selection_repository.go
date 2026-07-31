package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"navo/internal/domain/selection"
)

type SelectionRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

var _ selection.Repository = (*SelectionRepository)(nil)

func NewSelectionRepository(db *sql.DB, queryTimeout time.Duration) (*SelectionRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("MySQL database is nil")
	}
	if queryTimeout <= 0 {
		queryTimeout = 10 * time.Second
	}
	return &SelectionRepository{db: db, queryTimeout: queryTimeout}, nil
}

func (r *SelectionRepository) Load(ctx context.Context) (selection.ActiveSelection, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var (
		value          selection.ActiveSelection
		subscriptionID sql.NullString
		endpointID     sql.NullString
		upstreamID     sql.NullString
	)
	err := r.db.QueryRowContext(queryCtx, `
		SELECT core_type, source_type, capture_mode, subscription_id,
		       endpoint_id, upstream_proxy_id, updated_at
		FROM active_selection
		WHERE id = 'singleton'
	`).Scan(
		&value.CoreType,
		&value.SourceType,
		&value.CaptureMode,
		&subscriptionID,
		&endpointID,
		&upstreamID,
		&value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return selection.ActiveSelection{}, selection.ErrNotFound
	}
	if err != nil {
		return selection.ActiveSelection{}, fmt.Errorf("load active selection: %w", err)
	}
	value.SubscriptionID = nullableString(subscriptionID)
	value.EndpointID = nullableString(endpointID)
	value.UpstreamProxyID = nullableString(upstreamID)
	if err := value.Validate(); err != nil {
		return selection.ActiveSelection{}, fmt.Errorf("stored active selection is invalid: %w", err)
	}
	return value, nil
}

func (r *SelectionRepository) Save(ctx context.Context, value selection.ActiveSelection) error {
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate active selection: %w", err)
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = time.Now().UTC()
	} else {
		value.UpdatedAt = value.UpdatedAt.UTC()
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(queryCtx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin active selection transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(queryCtx, `
		INSERT INTO active_selection (
			id, core_type, source_type, capture_mode, subscription_id,
			endpoint_id, upstream_proxy_id, updated_at
		) VALUES ('singleton', ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			core_type = VALUES(core_type),
			source_type = VALUES(source_type),
			capture_mode = VALUES(capture_mode),
			subscription_id = VALUES(subscription_id),
			endpoint_id = VALUES(endpoint_id),
			upstream_proxy_id = VALUES(upstream_proxy_id),
			updated_at = VALUES(updated_at)
	`,
		value.CoreType.String(),
		value.SourceType.String(),
		value.CaptureMode.String(),
		nullableValue(value.SubscriptionID),
		nullableValue(value.EndpointID),
		nullableValue(value.UpstreamProxyID),
		value.UpdatedAt,
	); err != nil {
		return fmt.Errorf("save active selection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit active selection: %w", err)
	}
	return nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func nullableValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
