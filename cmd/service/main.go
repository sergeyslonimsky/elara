package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	coreapp "github.com/sergeyslonimsky/core/app"
	coregrpc "github.com/sergeyslonimsky/core/grpc"
	corehttp "github.com/sergeyslonimsky/core/http2"

	"github.com/sergeyslonimsky/elara/internal/di"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
	"github.com/sergeyslonimsky/elara/internal/domain"
	etcdinterceptor "github.com/sergeyslonimsky/elara/internal/handler/etcdv3/interceptor"
	"github.com/sergeyslonimsky/elara/internal/handler/ui"
	grpctransport "github.com/sergeyslonimsky/elara/internal/transport/grpc"
	"github.com/sergeyslonimsky/elara/internal/usecase/demo"
	"github.com/sergeyslonimsky/elara/web"
)

const shutdownTimeout = 30 * time.Second

// Env vars core/di's config loader checks to decide whether to read a
// config file at all (see vendor/.../core/di/config.go loadFromFile) — it
// only loads a file when one of these is explicitly set, it never searches
// standard locations on its own.
const (
	envConfigFilePaths = "APP_CONFIG_FILE_PATHS"
	envConfigFilePath  = "APP_CONFIG_FILE_PATH"
)

const localConfigFileName = "config.yaml"

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

	useLocalConfigFileIfPresent()

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

	// Reload Casbin's in-memory model from bbolt. Bootstrap writes p- and
	// g-rules directly through PolicyRepo (it must, because the rules are
	// what the enforcer would otherwise load on construction). On a fresh
	// DB those rules land in bbolt but never enter the enforcer cache that
	// was loaded before bootstrap ran — without this reload the superadmin's
	// (*,*,*) wildcard is invisible until the next process restart, which
	// presents to the operator as "logged in as admin but only see Dashboard".
	if err := svc.Enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("reload casbin policy after bootstrap: %w", err)
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
	service.V2Routes(frontendServer, svc.V2Handlers, svc.Sessions, svc.Adapters.AuthUsers, cfg)

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
// group, the (*,*,*) p-rule, and a type-specific admin identity. Safe to call
// on every startup — each step is idempotent.
//
//   - basic-auth: creates a local admin user from cfg.UI.Auth.BasicAuth and
//     adds it to the superadmin group.
//   - oidc: creates the group, policy, and a placeholder system user with
//     Email=cfg.UI.Auth.OIDC.AdminEmail. The first OIDC login matching that
//     email attaches its (provider, sub) identity to the placeholder and is
//     elevated into the group on callback.
//   - none: creates the synthetic passthrough user used by the interceptor.
func bootstrap(ctx context.Context, svc *service.Managers, cfg config.Config) error {
	if svc.Services.AdminBootstrap == nil {
		return nil
	}

	var err error

	switch cfg.UI.Auth.Type {
	case domain.AuthTypeBasicAuth:
		err = svc.Services.AdminBootstrap.BootstrapBasic(
			ctx,
			cfg.UI.Auth.BasicAuth.Username,
			cfg.UI.Auth.BasicAuth.Password,
		)
	case domain.AuthTypeOIDC:
		err = svc.Services.AdminBootstrap.BootstrapOIDC(ctx, cfg.UI.Auth.OIDC.AdminEmail)
	case domain.AuthTypeNone:
		err = svc.Services.AdminBootstrap.BootstrapPassthrough(ctx)
	}

	if err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	}

	return seedDemo(ctx, svc, cfg)
}

// seedDemo populates sample data when DEMO_MODE is set. It is a no-op for a
// normal deployment. Idempotent for persistent data; simulated clients are
// re-injected on every startup because the monitor is in-memory.
func seedDemo(ctx context.Context, svc *service.Managers, cfg config.Config) error {
	if !cfg.Demo.Enabled {
		return nil
	}

	err := demo.Seed(ctx, demo.Deps{
		Namespaces: svc.Services.Namespace,
		Configs:    svc.Services.Config,
		Schemas:    svc.Services.Schema,
		Clients:    svc.Adapters.ClientRegistry,
	})
	if err != nil {
		return fmt.Errorf("seed demo data: %w", err)
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

	if cfg.Client.Auth.Enabled || cfg.DangerouslySkipPermissions {
		tokenInterceptor := etcdinterceptor.NewTokenInterceptor(
			adapters.AuthTokens,
			etcdinterceptor.WithTokenSkipPermissions(cfg.DangerouslySkipPermissions),
		)
		opts = append(opts,
			coregrpc.WithUnaryInterceptor(tokenInterceptor.Unary()),
			coregrpc.WithStreamInterceptor(tokenInterceptor.Stream()),
		)
	}

	return opts
}

// useLocalConfigFileIfPresent lets a bare binary run (go install / a
// downloaded release, as opposed to the container image, which always sets
// CONFIG_DATA_PATH and its own config explicitly) pick up ~/.elara/config.yaml
// automatically, the way e.g. ~/.docker/config.json or ~/.kube/config do for
// other CLI-shaped tools.
//
// core/di's config loader (see vendor/.../core/di/config.go) never searches
// standard locations on its own — it only reads a file when
// APP_CONFIG_FILE_PATH(S) is set. So: if the operator already set one of
// those, or ~/.elara/config.yaml doesn't exist, this is a no-op.
func useLocalConfigFileIfPresent() {
	if os.Getenv(envConfigFilePaths) != "" || os.Getenv(envConfigFilePath) != "" {
		return
	}

	home := config.ElaraHomeDir()
	if home == "" {
		return
	}

	if _, err := os.Stat(filepath.Join(home, localConfigFileName)); err != nil {
		return
	}

	// loadFromFile does SetConfigName("config") + AddConfigPath(<dir>), so
	// the value here is the directory, not the file path.
	_ = os.Setenv(envConfigFilePaths, home)
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
