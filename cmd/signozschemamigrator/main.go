package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	schema_migrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

func main() {
	cmd := &cobra.Command{
		Use:   "signoz-schema-migrator",
		Short: "Signoz Schema Migrator",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.New()

			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.AutomaticEnv()

			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				configName := f.Name
				if !f.Changed && v.IsSet(configName) {
					val := v.Get(configName)
					err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
					if err != nil {
						panic(err)
					}
				}
			})
		},
	}

	var dsn string
	var replicationEnabled bool
	var clusterName string
	var development bool

	cmd.PersistentFlags().StringVar(&dsn, "dsn", "", "Clickhouse DSN")
	cmd.PersistentFlags().BoolVar(&replicationEnabled, "replication", false, "Enable replication")
	cmd.PersistentFlags().StringVar(&clusterName, "cluster-name", "cluster", "Cluster name to use while running migrations")
	cmd.PersistentFlags().BoolVar(&development, "dev", false, "Development mode")

	registerSyncMigrate(cmd)
	registerAsyncMigrate(cmd)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func createTLSConfig(args *runSyncMigrateArgs) (*tls.Config, error) {
	// custom tls config for full mtls enabled clickhouse

	// dir := "/home/ubuntu/clickhouse/volume/internal"
	dir := args.certDir
	certName := args.certName
	keyName := args.keyName
	caName := args.caName
	certFile := fmt.Sprintf("%s/%s", dir, certName)
	privateKeyFile := fmt.Sprintf("%s/%s", dir, keyName)
	caFile := fmt.Sprintf("%s/%s", dir, caName)

	log.Printf("regSyncMig> Loading cert/key... Cert=%s Key=%s", certFile, privateKeyFile)
	cert, err := tls.LoadX509KeyPair(certFile, privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client key pair: %w", err)
	}

	log.Printf("regSyncMig> Loading CA cert... Ca=%s", caFile)
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca certificate: %w", err)
	}

	log.Printf("regSyncMig> Creating cert pool...")
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	log.Printf("regSyncMig> Making TLS config...")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	log.Printf("regSyncMig> Done. TlsConfig=%+v", tlsConfig)
	return tlsConfig, nil
}

type runSyncMigrateArgs struct {
	dsn                string
	clusterName        string
	replicationEnabled bool
	development        bool
	upVersions         []uint64
	downVersions       []uint64
	certDir            string
	certName           string
	keyName            string
	caName             string
}

func RunSyncMigrate(args *runSyncMigrateArgs) error {
	logger := getLogger()

	logger.Info("Running migrations in sync mode", zap.String("dsn", args.dsn), zap.Bool("replication", args.replicationEnabled), zap.String("cluster-name", args.clusterName))

	if len(args.upVersions) != 0 && len(args.downVersions) != 0 {
		return fmt.Errorf("cannot provide both up and down migrations")
	}

	opts, err := clickhouse.ParseDSN(args.dsn)
	if err != nil {
		return fmt.Errorf("failed to parse dsn: %w", err)
	}
	logger.Info("Parsed DSN", zap.Any("opts", opts))

	tlsConfig, err := createTLSConfig(args)
	if err != nil {
		return fmt.Errorf("failed to get tls config: %w", err)
	}

	opts.TLS = tlsConfig
	// end of custom tls config for full mtls enabled clickhouse

	log.Printf("regSyncMig> Opening connection...")

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	logger.Info("Opened connection")

	manager, err := schema_migrator.NewMigrationManager(
		schema_migrator.WithClusterName(args.clusterName),
		schema_migrator.WithReplicationEnabled(args.replicationEnabled),
		schema_migrator.WithConn(conn),
		schema_migrator.WithConnOptions(*opts),
		schema_migrator.WithLogger(logger),
		schema_migrator.WithDevelopment(args.development),
	)
	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}
	err = manager.Bootstrap()
	if err != nil {
		return fmt.Errorf("failed to bootstrap migrations: %w", err)
	}
	logger.Info("Bootstrapped migrations")

	err = manager.RunSquashedMigrations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to run squashed migrations: %w", err)
	}
	logger.Info("Ran squashed migrations")

	if len(args.downVersions) != 0 {
		logger.Info("Migrating down")
		return manager.MigrateDownSync(context.Background(), args.downVersions)
	}
	logger.Info("Migrating up")
	return manager.MigrateUpSync(context.Background(), args.upVersions)
}

func registerSyncMigrate(cmd *cobra.Command) {

	var upVersions string
	var downVersions string

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Run migrations in sync mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()
			development := strings.ToLower(cmd.Flags().Lookup("dev").Value.String()) == "true"
			upVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("up").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				upVersions = append(upVersions, v)
			}

			downVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("down").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				downVersions = append(downVersions, v)
			}

			// certDir := "/home/ubuntu/clickhouse/volume/internal"
			// certName := "fullchain.crt"
			// keyName := "private_migration.key"
			// caName := "partialchain.crt"
			certDir := cmd.Flags().Lookup("cert-dir").Value.String()
			certName := cmd.Flags().Lookup("cert-name").Value.String()
			keyName := cmd.Flags().Lookup("key-name").Value.String()
			caName := cmd.Flags().Lookup("ca-name").Value.String()

			return RunSyncMigrate(&runSyncMigrateArgs{
				dsn:                dsn,
				clusterName:        clusterName,
				replicationEnabled: replicationEnabled,
				development:        development,
				upVersions:         upVersions,
				downVersions:       downVersions,
				certDir:            certDir,
				certName:           certName,
				keyName:            keyName,
				caName:             caName,
			})
		},
	}

	syncCmd.Flags().StringVar(&upVersions, "up", "", "Up migrations to run, comma separated. Leave empty to run all up migrations")
	syncCmd.Flags().StringVar(&downVersions, "down", "", "Down migrations to run, comma separated. Must provide down migrations explicitly to run")

	cmd.AddCommand(syncCmd)
}

func registerAsyncMigrate(cmd *cobra.Command) {

	var upVersions string
	var downVersions string

	asyncCmd := &cobra.Command{
		Use:   "async",
		Short: "Run migrations in async mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger()

			dsn := cmd.Flags().Lookup("dsn").Value.String()
			replicationEnabled := strings.ToLower(cmd.Flags().Lookup("replication").Value.String()) == "true"
			clusterName := cmd.Flags().Lookup("cluster-name").Value.String()
			development := strings.ToLower(cmd.Flags().Lookup("dev").Value.String()) == "true"

			logger.Info("Running migrations in async mode", zap.String("dsn", dsn), zap.Bool("replication", replicationEnabled), zap.String("cluster-name", clusterName))

			upVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("up").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				upVersions = append(upVersions, v)
			}
			logger.Info("Up migrations", zap.Any("versions", upVersions))

			downVersions := []uint64{}
			for _, version := range strings.Split(cmd.Flags().Lookup("down").Value.String(), ",") {
				if version == "" {
					continue
				}
				v, err := strconv.ParseUint(version, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse version: %w", err)
				}
				downVersions = append(downVersions, v)
			}
			logger.Info("Down migrations", zap.Any("versions", downVersions))

			if len(upVersions) != 0 && len(downVersions) != 0 {
				return fmt.Errorf("cannot provide both up and down migrations")
			}

			opts, err := clickhouse.ParseDSN(dsn)
			if err != nil {
				return fmt.Errorf("failed to parse dsn: %w", err)
			}
			logger.Info("Parsed DSN", zap.Any("opts", opts))

			conn, err := clickhouse.Open(opts)
			if err != nil {
				return fmt.Errorf("failed to open connection: %w", err)
			}
			logger.Info("Opened connection")

			manager, err := schema_migrator.NewMigrationManager(
				schema_migrator.WithClusterName(clusterName),
				schema_migrator.WithReplicationEnabled(replicationEnabled),
				schema_migrator.WithConn(conn),
				schema_migrator.WithConnOptions(*opts),
				schema_migrator.WithLogger(logger),
				schema_migrator.WithDevelopment(development),
			)
			if err != nil {
				return fmt.Errorf("failed to create migration manager: %w", err)
			}

			if len(downVersions) != 0 {
				logger.Info("Migrating down")
				return manager.MigrateDownAsync(context.Background(), downVersions)
			}
			logger.Info("Migrating up")
			return manager.MigrateUpAsync(context.Background(), upVersions)
		},
	}

	asyncCmd.Flags().StringVar(&upVersions, "up", "", "Up migrations to run, comma separated. Leave empty to run all up migrations")
	asyncCmd.Flags().StringVar(&downVersions, "down", "", "Down migrations to run, comma separated. Must provide down migrations explicitly to run")

	cmd.AddCommand(asyncCmd)
}
