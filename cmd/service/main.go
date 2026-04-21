package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	coreapp "github.com/sergeyslonimsky/core/app"
	coregrpc "github.com/sergeyslonimsky/core/grpc"
	corehttp "github.com/sergeyslonimsky/core/http2"
	coreotel "github.com/sergeyslonimsky/core/otel"

	"github.com/sergeyslonimsky/elara/internal/di"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
	"github.com/sergeyslonimsky/elara/internal/handler/ui"
	grpctransport "github.com/sergeyslonimsky/elara/internal/transport/grpc"
	"github.com/sergeyslonimsky/elara/web"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("service exited with error", slog.Any("err", err))
		os.Exit(1)
	}
}

func run() error {
	// Signal-aware ctx for the loader: if dynamic-etcd watching is ever
	// enabled, di.NewConfig starts a goroutine that exits on ctx.Done.
	// app.Run manages its own signal handling for runners; the loader ctx
	// is separate so the watcher goroutine doesn't outlive the process.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container, err := di.LoadContainer(ctx)
	if err != nil {
		return fmt.Errorf("load container: %w", err)
	}

	cfg, svc := container.Config, container.Services
	setupLogger(cfg)

	a := coreapp.New(coreapp.WithShutdownTimeout(shutdownTimeout))

	// Observability wiring. Both metrics (Prometheus pull) and tracing
	// (OTLP push) are opt-in — operators deploying elara into a cluster
	// without Prometheus Operator / Tempo can leave them off and the
	// service boots cleanly.
	promMetrics, err := setupMetrics(cfg)
	if err != nil {
		return fmt.Errorf("setup metrics: %w", err)
	}

	otelProvider, err := setupTracing(ctx, cfg)
	if err != nil {
		return fmt.Errorf("setup tracing: %w", err)
	}

	// Registration order is LIFO for shutdown:
	//   otelProvider      ← shuts down LAST (telemetry exporters close last)
	//   promMetrics       ← just before otel (flushes metrics before close)
	//   svc.Adapters      ← domain resources
	//   etcdServer        ← middle
	//   frontendServer    ← shuts down FIRST (stops accepting traffic)
	a.AddResource(otelProvider)

	if promMetrics != nil {
		a.AddResource(promMetrics)
	}

	a.AddResource(svc.Adapters)

	frontendServer := corehttp.NewServer(cfg.FrontendServer, frontendServerOptions(a, cfg, promMetrics)...)
	service.V2Routes(frontendServer, svc.V2Handlers)

	// Mount UI static file handler (serves frontend, fallback to index.html).
	if distFS := web.DistFS(); distFS != nil {
		frontendServer.Mount("/", ui.NewHandler(distFS))
	}

	// etcd-compatible gRPC API. Stats handler bridges connection & per-RPC
	// events into the connected-clients monitor. WithHealthService exposes
	// grpc.health.v1.Health so Envoy / k8s gRPC probes can reach us on the
	// same port as the etcd API — the response is driven by a.Healthcheck,
	// same as the HTTP /readyz above.
	statsHandler := grpctransport.NewStatsHandler(svc.Adapters.ClientRegistry)
	etcdServer := coregrpc.NewServer(cfg.EtcdServer, etcdServerOptions(a, cfg, statsHandler)...)
	service.EtcdRoutes(etcdServer, svc.EtcdHandlers)

	// frontendServer registered LAST → drains FIRST on SIGTERM.
	a.AddRunner(etcdServer, frontendServer)

	if err := a.Run(); err != nil {
		return fmt.Errorf("app run: %w", err)
	}

	return nil
}

func setupLogger(cfg config.Config) {
	level := parseLogLevel(cfg.Log.Level)
	opts := &slog.HandlerOptions{AddSource: !cfg.Log.NoSource, Level: level}

	var handler slog.Handler
	if cfg.Log.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// setupMetrics initialises a Prometheus pull-based MeterProvider and the
// /metrics HTTP handler if cfg.Metrics.Enabled is true. Returns nil (no
// error) when metrics are disabled — callers must check the returned
// value before using it.
func setupMetrics(cfg config.Config) (*service.PrometheusMetrics, error) {
	if !cfg.Metrics.Enabled {
		return nil, nil //nolint:nilnil // "disabled" is a valid non-error outcome
	}

	pm, err := service.NewPrometheusMetrics(cfg.ServiceName, cfg.ServiceVersion)
	if err != nil {
		return nil, fmt.Errorf("init prometheus metrics: %w", err)
	}

	return pm, nil
}

// setupTracing initialises the core/otel tracer with an OTLP HTTP trace
// exporter when cfg.Tracing.Enabled is true. Metrics and logs are left
// off: metrics go via Prometheus pull (see setupMetrics), logs go to
// stdout as JSON and are picked up by the cluster log collector.
//
// When tracing is disabled, returns a noop Provider so the lifecycle
// registration path stays uniform.
func setupTracing(ctx context.Context, cfg config.Config) (*coreotel.Provider, error) {
	otelCfg := coreotel.Config{ //nolint:exhaustruct // we intentionally only enable traces
		Disabled:       !cfg.Tracing.Enabled,
		OTelHost:       cfg.Tracing.OTLPEndpoint,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		EnableTracer:   cfg.Tracing.Enabled,
		EnableMetrics:  false,
		EnableLogger:   false,
	}

	provider, err := coreotel.Setup(ctx, otelCfg)
	if err != nil {
		return nil, fmt.Errorf("otel setup: %w", err)
	}

	return provider, nil
}

// frontendServerOptions builds the http2.Option list, adding otel + metrics
// handlers only when the corresponding feature is enabled.
func frontendServerOptions(
	a *coreapp.App,
	cfg config.Config,
	promMetrics *service.PrometheusMetrics,
) []corehttp.Option {
	opts := []corehttp.Option{
		corehttp.WithRecovery(),
		corehttp.WithHealthcheckFrom(a),
	}

	if cfg.Tracing.Enabled {
		opts = append(opts, corehttp.WithOtel())
	}

	if promMetrics != nil {
		opts = append(opts, corehttp.WithMetricsHandler(promMetrics.Handler()))
	}

	return opts
}

// etcdServerOptions builds the coregrpc.Option list for the etcd API
// server. Stats handler for client monitor, health service, recovery,
// and otel (if enabled) all compose cleanly.
func etcdServerOptions(
	a *coreapp.App,
	cfg config.Config,
	statsHandler *grpctransport.StatsHandler,
) []coregrpc.Option {
	opts := []coregrpc.Option{
		coregrpc.WithRecovery(),
		coregrpc.WithStatsHandler(statsHandler),
		coregrpc.WithHealthService(a),
	}

	if cfg.Tracing.Enabled {
		opts = append(opts, coregrpc.WithOtel())
	}

	return opts
}
