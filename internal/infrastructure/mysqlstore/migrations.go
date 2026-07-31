package mysqlstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const migrationLockName = "navo_schema_migrations"

type Migration struct {
	Version    int64
	Name       string
	Statements []string
	Down       []string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_multi_core_schema",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS core_installations (
				core_type VARCHAR(32) NOT NULL PRIMARY KEY,
				version VARCHAR(64) NOT NULL,
				binary_path VARCHAR(1024) NOT NULL,
				sha256 CHAR(64) NOT NULL,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				verified_at DATETIME(6) NOT NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS subscriptions (
				id VARCHAR(128) NOT NULL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				url_credential_ref VARCHAR(255) NOT NULL,
				etag VARCHAR(255) NULL,
				last_modified VARCHAR(255) NULL,
				last_status SMALLINT NULL,
				content_type VARCHAR(255) NULL,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS providers (
				id VARCHAR(128) NOT NULL PRIMARY KEY,
				subscription_id VARCHAR(128) NOT NULL,
				name VARCHAR(255) NOT NULL,
				format VARCHAR(64) NOT NULL,
				raw_hash CHAR(64) NULL,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				CONSTRAINT fk_providers_subscription FOREIGN KEY (subscription_id)
					REFERENCES subscriptions(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS endpoints (
				id VARCHAR(128) NOT NULL PRIMARY KEY,
				provider_id VARCHAR(128) NOT NULL,
				name VARCHAR(255) NOT NULL,
				protocol VARCHAR(32) NOT NULL,
				server VARCHAR(255) NOT NULL,
				port SMALLINT UNSIGNED NOT NULL,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				spec_version INT UNSIGNED NOT NULL,
				raw_format VARCHAR(64) NULL,
				raw_hash CHAR(64) NULL,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				INDEX idx_endpoints_provider (provider_id),
				CONSTRAINT fk_endpoints_provider FOREIGN KEY (provider_id)
					REFERENCES providers(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS endpoint_specs (
				endpoint_id VARCHAR(128) NOT NULL,
				spec_version INT UNSIGNED NOT NULL,
				protocol VARCHAR(32) NOT NULL,
				spec_json JSON NOT NULL,
				created_at DATETIME(6) NOT NULL,
				PRIMARY KEY (endpoint_id, spec_version),
				CONSTRAINT fk_endpoint_specs_endpoint FOREIGN KEY (endpoint_id)
					REFERENCES endpoints(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS upstream_proxies (
				id VARCHAR(128) NOT NULL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				protocol VARCHAR(16) NOT NULL,
				server VARCHAR(255) NOT NULL,
				port SMALLINT UNSIGNED NOT NULL,
				username_credential_ref VARCHAR(255) NULL,
				password_credential_ref VARCHAR(255) NULL,
				tls_enabled BOOLEAN NOT NULL,
				udp_policy VARCHAR(16) NOT NULL,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				created_at DATETIME(6) NOT NULL,
				updated_at DATETIME(6) NOT NULL,
				CONSTRAINT chk_upstream_protocol CHECK (protocol IN ('http','https','socks5')),
				CONSTRAINT chk_upstream_udp CHECK (
					(protocol = 'socks5') OR (udp_policy = 'disabled')
				)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS active_selection (
				id VARCHAR(16) NOT NULL PRIMARY KEY,
				core_type VARCHAR(32) NOT NULL,
				source_type VARCHAR(32) NOT NULL,
				capture_mode VARCHAR(32) NOT NULL,
				subscription_id VARCHAR(128) NULL,
				endpoint_id VARCHAR(128) NULL,
				upstream_proxy_id VARCHAR(128) NULL,
				updated_at DATETIME(6) NOT NULL,
				CONSTRAINT chk_active_singleton CHECK (id = 'singleton'),
				CONSTRAINT chk_active_core CHECK (core_type IN ('mihomo','xray','sing-box')),
				CONSTRAINT chk_active_source CHECK (source_type IN ('airport_subscription','upstream_proxy')),
				CONSTRAINT chk_active_capture CHECK (capture_mode IN ('off','system_proxy','tun')),
				CONSTRAINT chk_active_exclusive CHECK (
					(source_type = 'airport_subscription' AND subscription_id IS NOT NULL
						AND endpoint_id IS NOT NULL AND upstream_proxy_id IS NULL)
					OR
					(source_type = 'upstream_proxy' AND upstream_proxy_id IS NOT NULL
						AND subscription_id IS NULL AND endpoint_id IS NULL)
				),
				CONSTRAINT fk_active_subscription FOREIGN KEY (subscription_id)
					REFERENCES subscriptions(id),
				CONSTRAINT fk_active_endpoint FOREIGN KEY (endpoint_id)
					REFERENCES endpoints(id),
				CONSTRAINT fk_active_upstream FOREIGN KEY (upstream_proxy_id)
					REFERENCES upstream_proxies(id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS core_revisions (
				revision_id VARCHAR(128) NOT NULL PRIMARY KEY,
				core_type VARCHAR(32) NOT NULL,
				source_type VARCHAR(32) NOT NULL,
				endpoint_reference VARCHAR(128) NOT NULL,
				config_hash CHAR(64) NOT NULL,
				config_path VARCHAR(1024) NOT NULL,
				created_at DATETIME(6) NOT NULL,
				validation_status VARCHAR(32) NOT NULL,
				startup_status VARCHAR(32) NOT NULL,
				health_status VARCHAR(32) NOT NULL,
				is_active BOOLEAN NOT NULL DEFAULT FALSE,
				is_last_known_good BOOLEAN NOT NULL DEFAULT FALSE,
				INDEX idx_core_revisions_active (is_active),
				INDEX idx_core_revisions_lkg (is_last_known_good)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS core_runtime_events (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				revision_id VARCHAR(128) NULL,
				event_type VARCHAR(64) NOT NULL,
				error_code VARCHAR(64) NULL,
				message VARCHAR(1024) NOT NULL,
				created_at DATETIME(6) NOT NULL,
				INDEX idx_runtime_events_revision (revision_id),
				CONSTRAINT fk_runtime_events_revision FOREIGN KEY (revision_id)
					REFERENCES core_revisions(revision_id) ON DELETE SET NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS compatibility_cache (
				core_type VARCHAR(32) NOT NULL,
				core_version VARCHAR(64) NOT NULL,
				endpoint_id VARCHAR(128) NOT NULL,
				capture_mode VARCHAR(32) NOT NULL,
				supported BOOLEAN NOT NULL,
				level VARCHAR(32) NOT NULL,
				result_json JSON NOT NULL,
				checked_at DATETIME(6) NOT NULL,
				PRIMARY KEY (core_type, core_version, endpoint_id, capture_mode),
				CONSTRAINT fk_compatibility_endpoint FOREIGN KEY (endpoint_id)
					REFERENCES endpoints(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS system_proxy_snapshots (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				revision_id VARCHAR(128) NULL,
				snapshot_json JSON NOT NULL,
				created_at DATETIME(6) NOT NULL,
				restored_at DATETIME(6) NULL,
				CONSTRAINT fk_proxy_snapshots_revision FOREIGN KEY (revision_id)
					REFERENCES core_revisions(revision_id) ON DELETE SET NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`CREATE TABLE IF NOT EXISTS network_recovery_records (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				revision_id VARCHAR(128) NULL,
				state_json JSON NOT NULL,
				recovery_status VARCHAR(32) NOT NULL,
				created_at DATETIME(6) NOT NULL,
				recovered_at DATETIME(6) NULL,
				CONSTRAINT fk_recovery_revision FOREIGN KEY (revision_id)
					REFERENCES core_revisions(revision_id) ON DELETE SET NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
		Down: []string{
			"DROP TABLE IF EXISTS network_recovery_records",
			"DROP TABLE IF EXISTS system_proxy_snapshots",
			"DROP TABLE IF EXISTS compatibility_cache",
			"DROP TABLE IF EXISTS core_runtime_events",
			"DROP TABLE IF EXISTS core_revisions",
			"DROP TABLE IF EXISTS active_selection",
			"DROP TABLE IF EXISTS upstream_proxies",
			"DROP TABLE IF EXISTS endpoint_specs",
			"DROP TABLE IF EXISTS endpoints",
			"DROP TABLE IF EXISTS providers",
			"DROP TABLE IF EXISTS subscriptions",
			"DROP TABLE IF EXISTS core_installations",
		},
	},
}

func Migrate(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	if db == nil {
		return fmt.Errorf("MySQL database is nil")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := db.ExecContext(queryCtx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT NOT NULL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		checksum CHAR(64) NOT NULL,
		applied_at DATETIME(6) NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var acquired int
	if err := db.QueryRowContext(queryCtx, "SELECT GET_LOCK(?, ?)", migrationLockName, timeoutSeconds(timeout)).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("migration lock was not acquired")
	}
	defer releaseMigrationLock(db)

	applied, err := appliedMigrations(queryCtx, db)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		checksum := migrationChecksum(migration)
		if existing, ok := applied[migration.Version]; ok {
			if existing != checksum {
				return fmt.Errorf("migration %d checksum mismatch", migration.Version)
			}
			continue
		}
		for index, statement := range migration.Statements {
			if _, err := db.ExecContext(queryCtx, statement); err != nil {
				return fmt.Errorf("apply migration %d statement %d: %w", migration.Version, index+1, err)
			}
		}
		if _, err := db.ExecContext(
			queryCtx,
			"INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))",
			migration.Version, migration.Name, checksum,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
	}
	return nil
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[int64]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read schema migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan schema migration: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migrations: %w", err)
	}
	return applied, nil
}

func migrationChecksum(migration Migration) string {
	sum := sha256.Sum256([]byte(strings.Join(migration.Statements, "\x00")))
	return hex.EncodeToString(sum[:])
}

func timeoutSeconds(timeout time.Duration) int {
	seconds := int(timeout.Round(time.Second) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func releaseMigrationLock(db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = db.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", migrationLockName)
}
