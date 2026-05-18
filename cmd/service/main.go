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

	"github.com/sergeyslonimsky/elara/internal/di"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
	etcdinterceptor "github.com/sergeyslonimsky/elara/internal/handler/etcdv3/interceptor"
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

	// Idempotent data-plane seeding (admin user, casbin policies). Runs after
	// the container is wired but before any traffic-serving runners — the
	// container itself stays a pure constructor.
	if err := bootstrap(ctx, svc, cfg); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// Background worker: fan-out webhook delivery. Lives for the lifetime of
	// ctx — app.App's signal handler cancels ctx on shutdown, the dispatcher
	// drains and exits.
	go func() {
		if err := svc.Adapters.WebhookDispatcher.Run(ctx); err != nil {
			// Dispatcher's Run blocks until ctx cancel; non-nil error is
			// logged but doesn't fail the process — Shutdown handles drain.
			_ = err
		}
	}()

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

	frontendServer := corehttp.NewServer(cfg.UI.Server, frontendServerOptions(a, cfg, promMetrics)...)
	service.V2Routes(frontendServer, svc.V2Handlers, svc.SessionManager, cfg)

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
	etcdServer := coregrpc.NewServer(cfg.Client.EtcdServer, etcdServerOptions(a, cfg, svc.Adapters, statsHandler)...)
	service.EtcdRoutes(etcdServer, svc.EtcdHandlers)

	// frontendServer registered LAST → drains FIRST on SIGTERM.
	a.AddRunner(etcdServer, frontendServer)

	if err := a.Run(); err != nil {
		return fmt.Errorf("app run: %w", err)
	}

	return nil
}

// bootstrap performs idempotent superadmin seeding: the system:superadmin
// group, the superadmin user from config, and the (*,*,*) p-rule. Safe to
// call on every startup — each step is idempotent. Skipped when UI auth is
// disabled (passthrough mode handles its own enforcer seeding).
func bootstrap(ctx context.Context, svc *service.Manager, cfg config.Config) error {
	if !cfg.UI.Auth.Enabled {
		return nil
	}

	if svc.Services.AdminBootstrap == nil {
		return nil
	}

	if err := svc.Services.AdminBootstrap.Bootstrap(
		ctx,
		cfg.UI.Auth.SuperAdminUsername,
		cfg.UI.Auth.SuperAdminPassword,
	); err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	}

	return nil
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
	adapters *service.Adapters,
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

	if cfg.Client.Auth.Enabled {
		tokenInterceptor := etcdinterceptor.NewTokenInterceptor(adapters.AuthTokens)
		opts = append(opts,
			coregrpc.WithUnaryInterceptor(tokenInterceptor.Unary()),
			coregrpc.WithStreamInterceptor(tokenInterceptor.Stream()),
		)
	}

	return opts
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
