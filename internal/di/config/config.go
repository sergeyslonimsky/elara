package config

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/sergeyslonimsky/core/di"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	defaultHTTPPort  = "8080"
	defaultGRPCPort  = "2379"
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

	// Demo enables sample-data seeding and the first-run welcome modal. Off by
	// default; the `elara:demo` image sets DEMO_MODE=true.
	Demo DemoConfig

	DangerouslySkipPermissions bool
}

// ErrDangerousSkipPermissionsWithRealAuth is returned when
// DangerouslySkipPermissions is combined with a real auth surface
// (ui.auth.enabled and/or client.auth.enabled). That combination makes the
// deployment look secured — a login screen, a Tokens UI that appears to
// enforce scopes — while every permission check is actually bypassed
// underneath (internal/di/service/services.go's PDP.WithSkipPermissions,
// internal/di/service/handler.go's web-auth-interceptor skip, and
// cmd/service/main.go's etcd token-interceptor skip). "Dangerously" in the
// name is the intended failure mode for a fully-open dev instance —
// combined with real auth config it stops being a deliberate choice and
// becomes a trap. Leave both auth surfaces disabled instead if an open
// instance is actually wanted.
var ErrDangerousSkipPermissionsWithRealAuth = errors.New(
	"dangerously.skip.permissions=true cannot be combined with ui.auth.enabled=true or " +
		"client.auth.enabled=true — the deployment would look secured while enforcing nothing; " +
		"disable those instead if an open instance is intended",
)

// DemoConfig controls demo mode. When Enabled, the service seeds sample
// namespaces/configs/schemas on startup and injects simulated etcd clients
// into the connected-clients monitor, and the UI shows a welcome modal.
type DemoConfig struct {
	Enabled bool
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

	ui, err := newUIConfig(cfg)
	if err != nil {
		return Config{}, err
	}

	client := newClientConfig(cfg)

	c := Config{
		UI:     ui,
		Client: client,

		DataPath:       di.GetOrDefault(cfg, "config.data.path", defaultDataPath()),
		ServiceName:    di.GetOrDefault(cfg, "service.name", defaultServiceName),
		ServiceVersion: di.Get[string](cfg, "service.version"),
		Metrics: MetricsConfig{
			// Reads metrics.enabled / METRICS_ENABLED. Default: false.
			Enabled: di.Get[bool](cfg, "metrics.enabled"),
		},
		Tracing: TracingConfig{
			// Reads tracing.enabled / TRACING_ENABLED. Default: false.
			Enabled: di.Get[bool](cfg, "tracing.enabled"),
			// Reads tracing.otlp.endpoint / TRACING_OTLP_ENDPOINT.
			// Required when Tracing.Enabled is true; validated at setup.
			OTLPEndpoint: di.Get[string](cfg, "tracing.otlp.endpoint"),
		},
		Log: LogConfig{
			Level:    di.GetOrDefault(cfg, "log.level", defaultLogLevel),
			Format:   di.GetOrDefault(cfg, "log.format", defaultLogFormat),
			NoSource: di.Get[bool](cfg, "log.noSource"),
		},
		Demo: DemoConfig{
			// Reads demo.mode / DEMO_MODE. Default: false.
			Enabled: di.Get[bool](cfg, "demo.mode"),
		},
		DangerouslySkipPermissions: di.Get[bool](cfg, "dangerously.skip.permissions"),
	}

	if err := c.validateSkipPermissions(); err != nil {
		return Config{}, err
	}

	return c, nil
}

// defaultDataPath is the bbolt data directory used when config.data.path /
// CONFIG_DATA_PATH is not set. Inside the container image CONFIG_DATA_PATH
// is always set explicitly (to /var/lib/elara), so this only matters for a
// bare binary run from a shell: prefer ~/.elara/data there, falling back to
// the previous relative "./data" when no home directory is available.
func defaultDataPath() string {
	home := ElaraHomeDir()
	if home == "" {
		return "./data"
	}

	return filepath.Join(home, "data")
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

// ShouldSkipPermissionsForUI reports whether the web/ConnectRPC surface
// (PDP enforcement and the session AuthInterceptor) should bypass permission
// checks: passthrough mode (Type == AuthTypeNone) or the explicit escape
// hatch. Type == AuthTypeNone alone already covers !UI.Auth.Enabled, since
// resolveAuthType always resolves to AuthTypeNone when auth is disabled —
// using Enabled here instead of Type was a real bug: ui.auth.enabled=true +
// ui.auth.type=none ("passthrough", Validate()'s documented legitimate
// case) left PDP enforcing against the AuthInterceptor's synthetic
// uuid.Nil bypass principal, which has no Casbin policy — every request
// was denied.
//
// The web-auth-interceptor skip and the PDP skip MUST stay in lockstep (see
// interceptor.AuthInterceptor's doc comment) — both call this one function
// instead of duplicating the condition.
func (c Config) ShouldSkipPermissionsForUI() bool {
	return c.UI.Auth.Type == domain.AuthTypeNone || c.DangerouslySkipPermissions
}

// validateSkipPermissions rejects the DangerouslySkipPermissions +
// real-auth-surface combination described on ErrDangerousSkipPermissionsWithRealAuth.
func (c Config) validateSkipPermissions() error {
	if !c.DangerouslySkipPermissions {
		return nil
	}

	if c.UI.Auth.Enabled || c.Client.Auth.Enabled {
		return ErrDangerousSkipPermissionsWithRealAuth
	}

	return nil
}
