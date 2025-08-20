// Package tlsmigrator provides a wrapper around the SignOz schema migrator
// with enhanced TLS support and additional functionality for use as a library.
package schemamigrator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MigrateArgs represents the arguments needed for running sync migrations
type MigrateArgs struct {
	DSN                string
	ClusterName        string
	ReplicationEnabled bool
	Development        bool
	UpVersions         []uint64
	DownVersions       []uint64
	CertDir            string
	CertName           string
	KeyName            string
	CAName             string
}

// TLSMigrator wraps the RunSyncMigrate functionality with additional features
type TLSMigrator struct {
	logger *zap.Logger
}

// NewTLSMigrator creates a new TLS migrator instance
func NewTLSMigrator() *TLSMigrator {
	return &TLSMigrator{
		logger: getLogger(),
	}
}

// getLogger creates a zap logger instance
func getLogger() *zap.Logger {
	// Always verbose logging for schema migrator
	config := zap.NewDevelopmentConfig()
	config.Encoding = "json"
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger %v", err)
	}
	return logger
}

// createTLSConfig creates a TLS configuration from the provided arguments
func (tm *TLSMigrator) createTLSConfig(args *MigrateArgs) (*tls.Config, error) {
	// custom tls config for full mtls enabled clickhouse
	dir := args.CertDir
	certName := args.CertName
	keyName := args.KeyName
	caName := args.CAName
	certFile := fmt.Sprintf("%s/%s", dir, certName)
	privateKeyFile := fmt.Sprintf("%s/%s", dir, keyName)
	caFile := fmt.Sprintf("%s/%s", dir, caName)

	tm.logger.Info("Loading cert/key",
		zap.String("cert", certFile),
		zap.String("key", privateKeyFile))
	cert, err := tls.LoadX509KeyPair(certFile, privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client key pair: %w", err)
	}

	tm.logger.Info("Loading CA cert", zap.String("ca", caFile))
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca certificate: %w", err)
	}

	tm.logger.Info("Creating cert pool")
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tm.logger.Info("Making TLS config")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	tm.logger.Info("TLS config created successfully")
	return tlsConfig, nil
}

// RunSyncMigrate wraps the original RunSyncMigrate function with additional logging and error handling
func (tm *TLSMigrator) RunSyncMigrate(args *MigrateArgs) error {
	tm.logger.Info("Starting TLS migrator",
		zap.String("dsn", args.DSN),
		zap.Bool("replication", args.ReplicationEnabled),
		zap.String("cluster-name", args.ClusterName))

	if len(args.UpVersions) != 0 && len(args.DownVersions) != 0 {
		return fmt.Errorf("cannot provide both up and down migrations")
	}

	opts, err := clickhouse.ParseDSN(args.DSN)
	if err != nil {
		return fmt.Errorf("failed to parse dsn: %w", err)
	}
	tm.logger.Info("Parsed DSN", zap.Any("opts", opts))

	tlsConfig, err := tm.createTLSConfig(args)
	if err != nil {
		return fmt.Errorf("failed to get tls config: %w", err)
	}

	opts.TLS = tlsConfig
	// end of custom tls config for full mtls enabled clickhouse

	tm.logger.Info("Opening connection")
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	tm.logger.Info("Opened connection successfully")

	manager, err := NewMigrationManager(
		WithClusterName(args.ClusterName),
		WithReplicationEnabled(args.ReplicationEnabled),
		WithConn(conn),
		WithConnOptions(*opts),
		WithLogger(tm.logger),
		WithDevelopment(args.Development),
	)
	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}

	err = manager.Bootstrap()
	if err != nil {
		return fmt.Errorf("failed to bootstrap migrations: %w", err)
	}
	tm.logger.Info("Bootstrapped migrations")

	err = manager.RunSquashedMigrations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to run squashed migrations: %w", err)
	}
	tm.logger.Info("Ran squashed migrations")

	if len(args.DownVersions) != 0 {
		tm.logger.Info("Migrating down")
		return manager.MigrateDownSync(context.Background(), args.DownVersions)
	}
	tm.logger.Info("Migrating up")
	return manager.MigrateUpSync(context.Background(), args.UpVersions)
}

// RunSyncMigrateWithDefaults runs the migration with commonly used default values
func (tm *TLSMigrator) RunSyncMigrateWithDefaults(dsn, clusterName, certDir string) error {
	args := &MigrateArgs{
		DSN:                dsn,
		ClusterName:        clusterName,
		ReplicationEnabled: false,
		Development:        false,
		UpVersions:         []uint64{}, // empty means run all
		DownVersions:       []uint64{},
		CertDir:            certDir,
		CertName:           "fullchain.crt",
		KeyName:            "private_migration.key",
		CAName:             "partialchain.crt",
	}

	return tm.RunSyncMigrate(args)
}

// RunUpMigrations runs specific up migrations
func (tm *TLSMigrator) RunUpMigrations(dsn, clusterName, certDir string, versions []uint64) error {
	args := &MigrateArgs{
		DSN:                dsn,
		ClusterName:        clusterName,
		ReplicationEnabled: false,
		Development:        false,
		UpVersions:         versions,
		DownVersions:       []uint64{},
		CertDir:            certDir,
		CertName:           "fullchain.crt",
		KeyName:            "private_migration.key",
		CAName:             "partialchain.crt",
	}

	return tm.RunSyncMigrate(args)
}

// RunDownMigrations runs specific down migrations
func (tm *TLSMigrator) RunDownMigrations(dsn, clusterName, certDir string, versions []uint64) error {
	args := &MigrateArgs{
		DSN:                dsn,
		ClusterName:        clusterName,
		ReplicationEnabled: false,
		Development:        false,
		UpVersions:         []uint64{},
		DownVersions:       versions,
		CertDir:            certDir,
		CertName:           "fullchain.crt",
		KeyName:            "private_migration.key",
		CAName:             "partialchain.crt",
	}

	return tm.RunSyncMigrate(args)
}

// ValidateTLSConfig validates that the TLS certificate files exist and are readable
func (tm *TLSMigrator) ValidateTLSConfig(args *MigrateArgs) error {
	certFile := fmt.Sprintf("%s/%s", args.CertDir, args.CertName)
	keyFile := fmt.Sprintf("%s/%s", args.CertDir, args.KeyName)
	caFile := fmt.Sprintf("%s/%s", args.CertDir, args.CAName)

	// Check if certificate file exists and is readable
	if _, err := os.Stat(certFile); err != nil {
		return fmt.Errorf("certificate file not accessible: %w", err)
	}

	// Check if key file exists and is readable
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("key file not accessible: %w", err)
	}

	// Check if CA file exists and is readable
	if _, err := os.Stat(caFile); err != nil {
		return fmt.Errorf("CA file not accessible: %w", err)
	}

	// Try to load the certificate pair to validate
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate pair: %w", err)
	}

	// Try to read and parse the CA certificate
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse CA certificate")
	}

	tm.logger.Info("TLS configuration validation successful")
	return nil
}

// TestConnection tests the database connection without running migrations
func (tm *TLSMigrator) TestConnection(args *MigrateArgs) error {
	opts, err := clickhouse.ParseDSN(args.DSN)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", err)
	}

	tlsConfig, err := tm.createTLSConfig(args)
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	opts.TLS = tlsConfig

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer conn.Close()

	// Test the connection with a simple ping
	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	tm.logger.Info("Database connection test successful")
	return nil
}
