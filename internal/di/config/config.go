package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/core/di"
	coregrpc "github.com/sergeyslonimsky/core/grpc"
	"github.com/sergeyslonimsky/core/http2"
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
	FrontendServer http2.Config
	EtcdServer     coregrpc.Config
	DataPath       string
	Clients        ClientsConfig

	// Service identity — propagated to OTel / Prometheus resource labels.
	ServiceName    string
	ServiceVersion string

	// Observability is opt-in. Default for both Metrics and Tracing is
	// OFF so operators deploying elara into a cluster without Prometheus
	// Operator / Tempo / Jaeger can boot it without extra config.
	Metrics MetricsConfig
	Tracing TracingConfig
	Log     LogConfig
	Auth    AuthConfig
}

// AuthConfig controls authentication and session management.
type AuthConfig struct {
	Enabled     bool
	AdminEmails []string
	OIDC        OIDCConfig
	Session     SessionConfig
}

// OIDCConfig holds OpenID Connect provider settings.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// SessionConfig controls JWT session token signing and lifetime.
type SessionConfig struct {
	Secret string
	TTL    time.Duration
}

// LogConfig controls structured-log verbosity, output format, and source location.
type LogConfig struct {
	Level    string // "debug" | "info" | "warn" | "error"
	Format   string // "json" | "text"
	NoSource bool
}

// ClientsConfig is the in-process config for the connected-clients monitor.
type ClientsConfig struct {
	HistoryMaxRecords    int
	HistoryMaxAge        time.Duration
	RecentEventsCapacity int
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
		FrontendServer: http2.Config{
			Port:        cfg.GetStringOrDefault("http.frontend.port", defaultHTTPPort),
			ReadTimeout: cfg.GetDuration("http.frontend.readTimeout"),
			// Streaming-friendly default — see defaultFrontendWriteTimeout.
			WriteTimeout: durOrDefault(
				cfg.GetDuration("http.frontend.writeTimeout"),
				defaultFrontendWriteTimeout,
			),
		},
		EtcdServer: coregrpc.Config{
			Port: cfg.GetStringOrDefault("grpc.etcd.port", defaultGRPCPort),
		},
		DataPath:       cfg.GetStringOrDefault("config.data.path", defaultDataPath),
		ServiceName:    cfg.GetStringOrDefault("service.name", defaultServiceName),
		ServiceVersion: cfg.GetString("service.version"),
		Clients: ClientsConfig{
			HistoryMaxRecords: intOrDefault(
				cfg.GetInt("clients.history.max_records"),
				defaultClientHistoryMaxRecords,
			),
			HistoryMaxAge: durOrDefault(
				cfg.GetDuration("clients.history.max_age"),
				defaultClientHistoryMaxAge,
			),
			RecentEventsCapacity: intOrDefault(
				cfg.GetInt("clients.recent_events.capacity"),
				defaultClientRecentEventsCap,
			),
		},
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
		Auth: AuthConfig{
			Enabled:     cfg.GetBool("auth.enabled"),
			AdminEmails: cfg.GetStringSlice("auth.adminEmails"),
			OIDC: OIDCConfig{
				IssuerURL:    cfg.GetString("auth.oidc.issuerUrl"),
				ClientID:     cfg.GetString("auth.oidc.clientId"),
				ClientSecret: cfg.GetString("auth.oidc.clientSecret"),
				RedirectURL:  cfg.GetString("auth.oidc.redirectUrl"),
				Scopes: stringsOrDefault(
					cfg.GetStringSlice("auth.oidc.scopes"),
					[]string{"openid", "email", "profile"},
				),
			},
			Session: SessionConfig{
				Secret: cfg.GetString("auth.session.secret"),
				TTL: durOrDefault(
					cfg.GetDuration("auth.session.ttl"),
					defaultSessionTTL,
				),
			},
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
