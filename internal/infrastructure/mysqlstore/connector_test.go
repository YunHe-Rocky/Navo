package mysqlstore

import (
	"testing"
	"time"

	runtimeconfig "navo/internal/infrastructure/config"
)

func TestBuildDriverConfig(t *testing.T) {
	t.Parallel()

	cfg := runtimeconfig.MySQL{
		Enabled: true, Host: "mysql.example.com", Port: 3306,
		Database: "navo", User: "navo", Password: "secret",
		TLSMode: "verify_identity", ConnectTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
	}
	driverConfig, err := buildDriverConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if driverConfig.TLS == nil || driverConfig.TLS.ServerName != cfg.Host {
		t.Fatal("identity-verifying TLS config was not installed")
	}
	if driverConfig.AllowFallbackToPlaintext {
		t.Fatal("plaintext fallback must remain disabled")
	}
	if !driverConfig.ParseTime || driverConfig.DBName != "navo" {
		t.Fatal("required driver options are missing")
	}
}

func TestBuildDriverConfigRejectsInsecureMode(t *testing.T) {
	t.Parallel()
	cfg := runtimeconfig.MySQL{TLSMode: "disabled"}
	if _, err := buildDriverConfig(cfg); err == nil {
		t.Fatal("insecure mode accepted")
	}
}
