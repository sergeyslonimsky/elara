package service

import (
	"net/http"
	"slices"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	coreapp "github.com/sergeyslonimsky/core/app"
	coregrpc "github.com/sergeyslonimsky/core/grpc"
	corehttp "github.com/sergeyslonimsky/core/http2"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	etcdinterceptor "github.com/sergeyslonimsky/elara/internal/handler/etcdv3/interceptor"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1/authv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1/webhookv1connect"
	grpctransport "github.com/sergeyslonimsky/elara/internal/transport/grpc"
)

// httpServer is the minimal subset of *corehttp.Server we need to mount
// ConnectRPC handlers and the UI.
type httpServer interface {
	Mount(pattern string, handler http.Handler)
}

// Router owns every transport-level concern: connect interceptors, full
// HTTP/gRPC server option composition (recovery, health, otel, metrics, stats
// handler, token interceptor), and the actual mounting of HTTP and etcd
// routes. Self-contained — holds the Handlers it'll mount.
type Router struct {
	handlers          *Handlers
	telemetry         *PrometheusMetrics // nil when metrics disabled
	tracingEnabled    bool
	authEnabled       bool
	clientAuthEnabled bool

	publicOpts        connect.Option
	privateOpts       connect.Option
	etcdRouterOptions []coregrpc.Option
}

func NewRouter(
	srv *Services,
	components *Components,
	infra *Infrastructure,
	repos *Repositories,
	handlers *Handlers,
	cfg config.Config,
) *Router {
	authCfg := cfg.UI.Auth
	clientAuthCfg := cfg.Client.Auth
	sharedInterceptors := []connect.Interceptor{
		interceptor.NewRecoveryInterceptor(),
		interceptor.NewLoggingInterceptor(),
		validate.NewInterceptor(),
	}

	publicInterceptors := slices.Clone(sharedInterceptors)
	privateInterceptors := slices.Clone(sharedInterceptors)

	if authCfg.Enabled && srv.Session != nil {
		privateInterceptors = append(privateInterceptors, interceptor.NewAuthInterceptor(srv.Session))
	} else {
		privateInterceptors = append(privateInterceptors, &interceptor.PassthroughInterceptor{})
	}

	privateInterceptors = append(privateInterceptors, interceptor.NewAuthzInterceptor(srv.Authz))

	privateInterceptors = append(privateInterceptors, interceptor.NewRBACInterceptor(
		srv.Enforcer,
		interceptor.DefaultRBACPolicies(),
		interceptor.DefaultRBACAuthOnly(),
	))

	etcdOpts := []coregrpc.Option{
		coregrpc.WithStatsHandler(grpctransport.NewStatsHandler(components.ClientRegistry)),
	}

	if clientAuthCfg.Enabled {
		tokenInterceptor := etcdinterceptor.NewTokenInterceptor(repos.AuthTokens)
		etcdOpts = append(etcdOpts,
			coregrpc.WithUnaryInterceptor(tokenInterceptor.Unary()),
			coregrpc.WithStreamInterceptor(tokenInterceptor.Stream()),
		)
	}

	return &Router{
		handlers:          handlers,
		telemetry:         infra.Telemetry,
		tracingEnabled:    cfg.Tracing.Enabled,
		authEnabled:       authCfg.Enabled,
		clientAuthEnabled: clientAuthCfg.Enabled,
		publicOpts:        connect.WithInterceptors(publicInterceptors...),
		privateOpts:       connect.WithInterceptors(privateInterceptors...),
		etcdRouterOptions: etcdOpts,
	}
}

func (r *Router) HTTPServerOptions(a *coreapp.App) []corehttp.Option {
	opts := []corehttp.Option{
		corehttp.WithRecovery(),
		corehttp.WithHealthcheckFrom(a),
	}

	if r.tracingEnabled {
		opts = append(opts, corehttp.WithOtel())
	}

	if r.telemetry != nil {
		opts = append(opts, corehttp.WithMetricsHandler(r.telemetry.Handler()))
	}

	return opts
}

func (r *Router) EtcdServerOptions(a *coreapp.App) []coregrpc.Option {
	opts := []coregrpc.Option{
		coregrpc.WithRecovery(),
		coregrpc.WithHealthService(a),
	}

	if r.tracingEnabled {
		opts = append(opts, coregrpc.WithOtel())
	}

	opts = append(opts, r.etcdRouterOptions...)

	return opts
}

func (r *Router) MountHTTP(server httpServer) {
	h := r.handlers

	// Always-on private services.
	server.Mount(configv1connect.NewConfigServiceHandler(h.Config, r.privateOpts))
	server.Mount(namespacev1connect.NewNamespaceServiceHandler(h.Namespace, r.privateOpts))
	server.Mount(clientsv1connect.NewClientsServiceHandler(h.Clients, r.privateOpts))
	server.Mount(dashboardv1connect.NewDashboardServiceHandler(h.Dashboard, r.privateOpts))
	server.Mount(transferv1connect.NewTransferServiceHandler(h.Transfer, r.privateOpts))
	server.Mount(configv1connect.NewSchemaServiceHandler(h.Schema, r.privateOpts))
	server.Mount(webhookv1connect.NewWebhookServiceHandler(h.Webhook, r.privateOpts))

	// Auth + user management — public/private split lives here.
	if r.authEnabled {
		server.Mount(authv1connect.NewAuthServiceHandler(h.Auth, r.publicOpts))
		server.Mount(authv1connect.NewUserServiceHandler(h.Users, r.privateOpts))
		server.Mount(authv1connect.NewGroupServiceHandler(h.Groups, r.privateOpts))
		server.Mount(authv1connect.NewAccessServiceHandler(h.Access, r.privateOpts))
	}

	if r.clientAuthEnabled {
		server.Mount(authv1connect.NewTokenServiceHandler(h.Tokens, r.privateOpts))
	}

	// UI static fallback. Mounted last so ConnectRPC routes win on overlap.
	if h.UI != nil {
		server.Mount("/", h.UI)
	}
}

func (r *Router) MountEtcd(server *coregrpc.Server) {
	h := r.handlers

	server.Mount(func(gs *grpc.Server) {
		etcdserverpb.RegisterKVServer(gs, h.KV)
		etcdserverpb.RegisterWatchServer(gs, h.Watch)
		etcdserverpb.RegisterMaintenanceServer(gs, h.Maintenance)
		etcdserverpb.RegisterClusterServer(gs, h.Cluster)
	})
}
