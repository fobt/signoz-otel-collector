module github.com/fobt/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator

go 1.23.0

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.36.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/ClickHouse/ch-go v0.66.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/paulmach/orb v0.11.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/ClickHouse/ch-go v0.66.0 => github.com/SigNoz/ch-go v0.66.0-dd-sketch
	github.com/ClickHouse/clickhouse-go/v2 v2.36.0 => github.com/SigNoz/clickhouse-go/v2 v2.36.0-dd-sketch
	github.com/segmentio/ksuid => github.com/signoz/ksuid v1.0.4
	github.com/vjeantet/grok => github.com/signoz/grok v1.0.3

	// using 0.23.0 as there is an issue with 0.24.0 stats that results in
	// an error
	// panic: interface conversion: interface {} is nil, not func(*tag.Map, []stats.Measurement, map[string]interface {})

	go.opencensus.io => go.opencensus.io v0.23.0
)

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4433
exclude github.com/StackExchange/wmi v1.2.0
