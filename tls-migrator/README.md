# TLS Migrator

A Go library that wraps the SignOz schema migrator with enhanced TLS support and additional functionality for use in other applications.

## Overview

The `tlsmigrator` package provides a convenient wrapper around the SignOz schema migration functionality, specifically designed for ClickHouse databases with TLS/mTLS configurations. It offers a clean API for running database migrations programmatically from within Go applications.

## Features

- **TLS/mTLS Support**: Built-in support for TLS configurations with client certificates
- **Flexible Migration Control**: Run all migrations, specific up migrations, or down migrations
- **Connection Testing**: Test database connectivity before running migrations
- **Configuration Validation**: Validate TLS certificate files before use
- **Structured Logging**: Uses zap logger for structured logging output
- **Error Handling**: Comprehensive error handling with detailed error messages

## Installation

Since this is part of the SignOz OpenTelemetry Collector repository, you can import it directly:

```go
import "github.com/SigNoz/signoz-otel-collector/tls-migrator"
```

## Usage

### Basic Usage

```go
package main

import (
    "log"
    tlsmigrator "github.com/SigNoz/signoz-otel-collector/tls-migrator"
)

func main() {
    // Create a new TLS migrator instance
    migrator := tlsmigrator.NewTLSMigrator()
    
    // Run all up migrations with default certificate names
    err := migrator.RunSyncMigrateWithDefaults(
        "clickhouse://username:password@hostname:9000/database",
        "cluster",
        "/path/to/certificates",
    )
    if err != nil {
        log.Fatalf("Migration failed: %v", err)
    }
    
    log.Println("Migration completed successfully!")
}
```

### Advanced Usage with Custom Configuration

```go
package main

import (
    "log"
    tlsmigrator "github.com/SigNoz/signoz-otel-collector/tls-migrator"
)

func main() {
    migrator := tlsmigrator.NewTLSMigrator()
    
    // Custom migration arguments
    args := &tlsmigrator.MigrateArgs{
        DSN:                "clickhouse://username:password@hostname:9000/database",
        ClusterName:        "my-cluster",
        ReplicationEnabled: true,
        Development:        false,
        UpVersions:         []uint64{1, 2, 3}, // Run specific migrations
        DownVersions:       []uint64{},
        CertDir:            "/path/to/certificates",
        CertName:           "client.crt",
        KeyName:            "client.key",
        CAName:             "ca.crt",
    }
    
    // Validate TLS configuration first
    if err := migrator.ValidateTLSConfig(args); err != nil {
        log.Fatalf("TLS validation failed: %v", err)
    }
    
    // Test connection before migration
    if err := migrator.TestConnection(args); err != nil {
        log.Fatalf("Connection test failed: %v", err)
    }
    
    // Run the migration
    if err := migrator.RunSyncMigrate(args); err != nil {
        log.Fatalf("Migration failed: %v", err)
    }
    
    log.Println("Migration completed successfully!")
}
```

### Running Specific Migrations

```go
// Run specific up migrations
versions := []uint64{1, 2, 3}
err := migrator.RunUpMigrations(
    "clickhouse://username:password@hostname:9000/database",
    "cluster",
    "/path/to/certificates",
    versions,
)

// Run specific down migrations
downVersions := []uint64{3, 2, 1}
err = migrator.RunDownMigrations(
    "clickhouse://username:password@hostname:9000/database",
    "cluster", 
    "/path/to/certificates",
    downVersions,
)
```

## API Reference

### Types

#### `MigrateArgs`

```go
type MigrateArgs struct {
    DSN                string    // ClickHouse connection string
    ClusterName        string    // Cluster name for migrations
    ReplicationEnabled bool      // Enable replication
    Development        bool      // Development mode
    UpVersions         []uint64  // Specific up migrations to run (empty = all)
    DownVersions       []uint64  // Specific down migrations to run
    CertDir            string    // Directory containing TLS certificates
    CertName           string    // Certificate file name
    KeyName            string    // Private key file name
    CAName             string    // CA certificate file name
}
```

#### `TLSMigrator`

The main migrator instance that provides all migration functionality.

### Methods

#### `NewTLSMigrator() *TLSMigrator`

Creates a new TLS migrator instance with a configured logger.

#### `RunSyncMigrate(args *MigrateArgs) error`

Runs the migration with the provided arguments. This is the core migration method.

#### `RunSyncMigrateWithDefaults(dsn, clusterName, certDir string) error`

Runs all up migrations with default certificate names:
- Certificate: `fullchain.crt`
- Private Key: `private_migration.key`
- CA Certificate: `partialchain.crt`

#### `RunUpMigrations(dsn, clusterName, certDir string, versions []uint64) error`

Runs specific up migrations with default certificate names.

#### `RunDownMigrations(dsn, clusterName, certDir string, versions []uint64) error`

Runs specific down migrations with default certificate names.

#### `ValidateTLSConfig(args *MigrateArgs) error`

Validates that TLS certificate files exist, are readable, and can be loaded properly.

#### `TestConnection(args *MigrateArgs) error`

Tests the database connection without running any migrations.

## Certificate Files

The library expects three certificate files in the specified directory:

1. **Client Certificate** (`fullchain.crt` by default): The client certificate for mTLS
2. **Private Key** (`private_migration.key` by default): The private key for the client certificate
3. **CA Certificate** (`partialchain.crt` by default): The Certificate Authority certificate

## Error Handling

All methods return detailed errors that can be used for debugging:

```go
if err := migrator.RunSyncMigrateWithDefaults(dsn, cluster, certDir); err != nil {
    // Handle specific error types
    switch {
    case strings.Contains(err.Error(), "certificate"):
        log.Printf("Certificate error: %v", err)
    case strings.Contains(err.Error(), "connection"):
        log.Printf("Connection error: %v", err)
    default:
        log.Printf("Migration error: %v", err)
    }
}
```

## Logging

The library uses structured logging via zap. Logs include:

- TLS configuration loading steps
- Database connection status
- Migration progress
- Error details

## Dependencies

This library depends on:
- `github.com/ClickHouse/clickhouse-go/v2` - ClickHouse Go driver
- `go.uber.org/zap` - Structured logging
- SignOz schema migrator internal packages

## License

This library is part of the SignOz OpenTelemetry Collector and follows the same license terms.
