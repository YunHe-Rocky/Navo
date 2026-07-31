package config

import "testing"

func TestLoadMySQLDefaultsToDisabled(t *testing.T) {
	clearMySQLEnv(t)
	cfg, err := LoadMySQL()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("MySQL must be opt-in")
	}
	if cfg.Port != 3306 || cfg.Database != "navo" {
		t.Fatalf("unexpected defaults: port=%d database=%q", cfg.Port, cfg.Database)
	}
}

func TestLoadMySQLValidatesEnabledConfiguration(t *testing.T) {
	clearMySQLEnv(t)
	t.Setenv("NAVO_MYSQL_ENABLED", "true")
	if _, err := LoadMySQL(); err == nil {
		t.Fatal("enabled incomplete MySQL configuration accepted")
	}

	t.Setenv("NAVO_MYSQL_HOST", "mysql.example.com")
	t.Setenv("NAVO_MYSQL_USER", "navo")
	t.Setenv("NAVO_MYSQL_MAX_OPEN_CONNS", "5")
	t.Setenv("NAVO_MYSQL_MAX_IDLE_CONNS", "6")
	if _, err := LoadMySQL(); err == nil {
		t.Fatal("invalid pool configuration accepted")
	}

	t.Setenv("NAVO_MYSQL_MAX_IDLE_CONNS", "5")
	cfg, err := LoadMySQL()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.TLSMode != "verify_identity" {
		t.Fatalf("unexpected config: enabled=%v tls=%q", cfg.Enabled, cfg.TLSMode)
	}
}

func TestLoadMySQLRejectsInsecureTLSMode(t *testing.T) {
	clearMySQLEnv(t)
	t.Setenv("NAVO_MYSQL_ENABLED", "true")
	t.Setenv("NAVO_MYSQL_HOST", "mysql.example.com")
	t.Setenv("NAVO_MYSQL_USER", "navo")
	t.Setenv("NAVO_MYSQL_TLS_MODE", "disabled")
	if _, err := LoadMySQL(); err == nil {
		t.Fatal("insecure TLS mode accepted")
	}
}

func clearMySQLEnv(t *testing.T) {
	t.Helper()
	names := []string{
		"NAVO_MYSQL_ENABLED", "NAVO_MYSQL_REQUIRED", "NAVO_MYSQL_HOST",
		"NAVO_MYSQL_PORT", "NAVO_MYSQL_DATABASE", "NAVO_MYSQL_USER",
		"NAVO_MYSQL_PASSWORD", "NAVO_MYSQL_TLS_MODE", "NAVO_MYSQL_CA_FILE",
		"NAVO_MYSQL_CONNECT_TIMEOUT", "NAVO_MYSQL_READ_TIMEOUT",
		"NAVO_MYSQL_WRITE_TIMEOUT", "NAVO_MYSQL_QUERY_TIMEOUT",
		"NAVO_MYSQL_MAX_OPEN_CONNS", "NAVO_MYSQL_MAX_IDLE_CONNS",
		"NAVO_MYSQL_CONN_MAX_LIFETIME", "NAVO_MYSQL_CONN_MAX_IDLE_TIME",
	}
	for _, name := range names {
		t.Setenv(name, "")
	}
}
