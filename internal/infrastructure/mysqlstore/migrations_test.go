package mysqlstore

import (
	"strings"
	"testing"
)

func TestMigrationsAreVersionedAndSecretSafe(t *testing.T) {
	t.Parallel()

	if len(migrations) == 0 {
		t.Fatal("no migrations defined")
	}
	var previous int64
	for _, migration := range migrations {
		if migration.Version <= previous {
			t.Fatalf("migration versions are not increasing: %d", migration.Version)
		}
		if migration.Name == "" || len(migration.Statements) == 0 || len(migration.Down) == 0 {
			t.Fatalf("migration %d is incomplete", migration.Version)
		}
		if len(migrationChecksum(migration)) != 64 {
			t.Fatalf("migration %d checksum is invalid", migration.Version)
		}
		for _, statement := range migration.Statements {
			lower := strings.ToLower(statement)
			if strings.Contains(lower, " password ") || strings.Contains(lower, " subscription_url ") {
				t.Fatalf("migration %d stores plaintext secret fields", migration.Version)
			}
		}
		previous = migration.Version
	}
}

func TestInitialMigrationContainsRequiredTables(t *testing.T) {
	t.Parallel()

	all := strings.ToLower(strings.Join(migrations[0].Statements, "\n"))
	required := []string{
		"core_installations", "subscriptions", "providers", "endpoints",
		"endpoint_specs", "upstream_proxies", "active_selection",
		"core_revisions", "core_runtime_events", "compatibility_cache",
		"system_proxy_snapshots", "network_recovery_records",
	}
	for _, table := range required {
		if !strings.Contains(all, "table if not exists "+table) {
			t.Errorf("required table %s is missing", table)
		}
	}
}
