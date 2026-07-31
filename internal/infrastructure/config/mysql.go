package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type MySQL struct {
	Enabled         bool
	Required        bool
	Host            string
	Port            uint16
	Database        string
	User            string
	Password        string
	TLSMode         string
	CAFile          string
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	QueryTimeout    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func LoadMySQL() (MySQL, error) {
	cfg := MySQL{
		Host:            strings.TrimSpace(os.Getenv("NAVO_MYSQL_HOST")),
		Database:        strings.TrimSpace(os.Getenv("NAVO_MYSQL_DATABASE")),
		User:            strings.TrimSpace(os.Getenv("NAVO_MYSQL_USER")),
		Password:        os.Getenv("NAVO_MYSQL_PASSWORD"),
		TLSMode:         strings.ToLower(strings.TrimSpace(os.Getenv("NAVO_MYSQL_TLS_MODE"))),
		CAFile:          strings.TrimSpace(os.Getenv("NAVO_MYSQL_CA_FILE")),
		Port:            3306,
		ConnectTimeout:  5 * time.Second,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		QueryTimeout:    10 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3 * time.Minute,
		ConnMaxIdleTime: time.Minute,
	}
	if cfg.Database == "" {
		cfg.Database = "navo"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = "verify_identity"
	}

	var err error
	if cfg.Enabled, err = envBool("NAVO_MYSQL_ENABLED", false); err != nil {
		return MySQL{}, err
	}
	if cfg.Required, err = envBool("NAVO_MYSQL_REQUIRED", false); err != nil {
		return MySQL{}, err
	}
	if cfg.Required {
		cfg.Enabled = true
	}
	if cfg.Port, err = envUint16("NAVO_MYSQL_PORT", cfg.Port); err != nil {
		return MySQL{}, err
	}
	if cfg.ConnectTimeout, err = envDuration("NAVO_MYSQL_CONNECT_TIMEOUT", cfg.ConnectTimeout); err != nil {
		return MySQL{}, err
	}
	if cfg.ReadTimeout, err = envDuration("NAVO_MYSQL_READ_TIMEOUT", cfg.ReadTimeout); err != nil {
		return MySQL{}, err
	}
	if cfg.WriteTimeout, err = envDuration("NAVO_MYSQL_WRITE_TIMEOUT", cfg.WriteTimeout); err != nil {
		return MySQL{}, err
	}
	if cfg.QueryTimeout, err = envDuration("NAVO_MYSQL_QUERY_TIMEOUT", cfg.QueryTimeout); err != nil {
		return MySQL{}, err
	}
	if cfg.MaxOpenConns, err = envPositiveInt("NAVO_MYSQL_MAX_OPEN_CONNS", cfg.MaxOpenConns); err != nil {
		return MySQL{}, err
	}
	if cfg.MaxIdleConns, err = envNonNegativeInt("NAVO_MYSQL_MAX_IDLE_CONNS", cfg.MaxIdleConns); err != nil {
		return MySQL{}, err
	}
	if cfg.ConnMaxLifetime, err = envDuration("NAVO_MYSQL_CONN_MAX_LIFETIME", cfg.ConnMaxLifetime); err != nil {
		return MySQL{}, err
	}
	if cfg.ConnMaxIdleTime, err = envDuration("NAVO_MYSQL_CONN_MAX_IDLE_TIME", cfg.ConnMaxIdleTime); err != nil {
		return MySQL{}, err
	}
	if err := cfg.Validate(); err != nil {
		return MySQL{}, err
	}
	return cfg, nil
}

func (c MySQL) Validate() error {
	if !c.Enabled {
		return nil
	}
	var missing []string
	if c.Host == "" {
		missing = append(missing, "NAVO_MYSQL_HOST")
	}
	if c.Database == "" {
		missing = append(missing, "NAVO_MYSQL_DATABASE")
	}
	if c.User == "" {
		missing = append(missing, "NAVO_MYSQL_USER")
	}
	if len(missing) > 0 {
		return fmt.Errorf("MySQL is enabled but required variables are missing: %s", strings.Join(missing, ", "))
	}
	switch c.TLSMode {
	case "required", "verify_ca", "verify_identity":
	default:
		return fmt.Errorf("NAVO_MYSQL_TLS_MODE must be required, verify_ca or verify_identity")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("NAVO_MYSQL_MAX_IDLE_CONNS cannot exceed NAVO_MYSQL_MAX_OPEN_CONNS")
	}
	return nil
}

func envBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func envUint16(name string, fallback uint16) (uint16, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return uint16(value), nil
}

func envPositiveInt(name string, fallback int) (int, error) {
	value, err := envNonNegativeInt(name, fallback)
	if err != nil {
		return 0, err
	}
	if value == 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}

func envNonNegativeInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be zero or greater", name)
	}
	return value, nil
}
