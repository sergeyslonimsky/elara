package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/core/di"
)

const (
	defaultHTTPPort  = "8080"
	defaultGRPCPort  = "2379"
	defaultDataPath  = "./data"
	defaultLogLevel  = "info"
	defaultLogFormat = "json"

	defaultClientHistoryMaxRecords = 1000
	defaultClientHistoryMaxAge     = 30 * 24 * time.Hour
	defaultClientRecentEventsCap   = 100

	// defaultFrontendWriteTimeout governs how long a single response body can
	// take to write. We host server-streaming RPCs (WatchClients, WatchClient)
	// on the frontend port, so this must be much larger than any normal
	// request — otherwise streams get cut every N seconds. 24h means streams
	// effectively live until the client closes them.
	defaultFrontendWriteTimeout = 24 * time.Hour

	// defaultServiceName is embedded in Prometheus/OTLP resource labels
	// when operators don't override it.
	defaultServiceName = "elara"

	defaultSessionTTL = 24 * time.Hour
)

type Config struct {
	UI     UI
	Client Client

	// Service identity — propagated to OTel / Prometheus resource labels.
	ServiceName    string
	ServiceVersion string

	DataPath string

	// Observability is opt-in. Default for both Metrics and Tracing is
	// OFF so operators deploying elara into a cluster without Prometheus
	// Operator / Tempo / Jaeger can boot it without extra config.
	Metrics MetricsConfig
	Tracing TracingConfig
	Log     LogConfig
}

// LogConfig controls structured-log verbosity, output format, and source location.
type LogConfig struct {
	Level    string // "debug" | "info" | "warn" | "error"
	Format   string // "json" | "text"
	NoSource bool
}

// MetricsConfig controls the Prometheus /metrics pull endpoint. When
// Enabled, the HTTP server serves Prometheus-format metrics at /metrics
// and Prometheus Operator can scrape it via a ServiceMonitor.
type MetricsConfig struct {
	Enabled bool
}

// TracingConfig controls OTLP trace push. When Enabled, elara creates
// spans for HTTP requests and gRPC RPCs and pushes them to OTLPEndpoint
// (typically an OTel collector, Tempo, or Jaeger OTLP gateway).
type TracingConfig struct {
	Enabled      bool
	OTLPEndpoint string
}

func NewConfig(ctx context.Context) (Config, error) {
	cfg, err := di.NewConfig(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("init di config: %w", err)
	}

	return Config{
		UI:     newUIConfig(cfg),
		Client: newClientConfig(cfg),

		DataPath:       cfg.GetStringOrDefault("config.data.path", defaultDataPath),
		ServiceName:    cfg.GetStringOrDefault("service.name", defaultServiceName),
		ServiceVersion: cfg.GetString("service.version"),
		Metrics: MetricsConfig{
			// Reads metrics.enabled / METRICS_ENABLED. Default: false.
			Enabled: cfg.GetBool("metrics.enabled"),
		},
		Tracing: TracingConfig{
			// Reads tracing.enabled / TRACING_ENABLED. Default: false.
			Enabled: cfg.GetBool("tracing.enabled"),
			// Reads tracing.otlp.endpoint / TRACING_OTLP_ENDPOINT.
			// Required when Tracing.Enabled is true; validated at setup.
			OTLPEndpoint: cfg.GetString("tracing.otlp.endpoint"),
		},
		Log: LogConfig{
			Level:    cfg.GetStringOrDefault("log.level", defaultLogLevel),
			Format:   cfg.GetStringOrDefault("log.format", defaultLogFormat),
			NoSource: cfg.GetBool("log.noSource"),
		},
	}, nil
}

func intOrDefault(v, d int) int {
	if v <= 0 {
		return d
	}

	return v
}

func durOrDefault(v, d time.Duration) time.Duration {
	if v <= 0 {
		return d
	}

	return v
}

func stringsOrDefault(v, d []string) []string {
	if len(v) == 0 {
		return d
	}

	return v
}
