package mysqlstore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"os"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	runtimeconfig "navo/internal/infrastructure/config"
)

func Open(ctx context.Context, cfg runtimeconfig.MySQL) (*sql.DB, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	driverConfig, err := buildDriverConfig(cfg)
	if err != nil {
		return nil, err
	}
	connector, err := mysqlDriver.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("create MySQL connector: %w", err)
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	return db, nil
}

func buildDriverConfig(cfg runtimeconfig.MySQL) (*mysqlDriver.Config, error) {
	driverConfig := mysqlDriver.NewConfig()
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	driverConfig.DBName = cfg.Database
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC
	driverConfig.Timeout = cfg.ConnectTimeout
	driverConfig.ReadTimeout = cfg.ReadTimeout
	driverConfig.WriteTimeout = cfg.WriteTimeout
	driverConfig.Collation = "utf8mb4_unicode_ci"
	driverConfig.Params = map[string]string{
		"sql_mode": "'STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'",
	}

	switch cfg.TLSMode {
	case "required":
		driverConfig.TLSConfig = "true"
	case "verify_identity", "verify_ca":
		tlsConfig, err := secureTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		driverConfig.TLS = tlsConfig
	default:
		return nil, fmt.Errorf("unsupported MySQL TLS mode %q", cfg.TLSMode)
	}
	return driverConfig, nil
}

func secureTLSConfig(cfg runtimeconfig.MySQL) (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read MySQL CA file: %w", err)
		}
		if ok := roots.AppendCertsFromPEM(pem); !ok {
			return nil, fmt.Errorf("MySQL CA file contains no valid certificates")
		}
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
		ServerName: cfg.Host,
	}
	if cfg.TLSMode == "verify_ca" {
		// Go has no CA-only mode. Disable the built-in hostname check and
		// reproduce chain verification explicitly without a DNSName.
		tlsConfig.InsecureSkipVerify = true //nolint:gosec
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("MySQL server sent no certificate")
			}
			intermediates := x509.NewCertPool()
			for _, certificate := range state.PeerCertificates[1:] {
				intermediates.AddCert(certificate)
			}
			_, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{
				Roots:         roots,
				Intermediates: intermediates,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			})
			return err
		}
	}
	return tlsConfig, nil
}
