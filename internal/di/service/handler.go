package service

import (
	"context"
	"net/http"
	"slices"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/auth"
	capabilitieshandler "github.com/sergeyslonimsky/elara/internal/handler/v2/capabilities"
	clientshandler "github.com/sergeyslonimsky/elara/internal/handler/v2/clients"
	confighandler "github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	dashboardhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/dashboard"
	filterhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/filter"
	grouphandler "github.com/sergeyslonimsky/elara/internal/handler/v2/group"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	namespacehandler "github.com/sergeyslonimsky/elara/internal/handler/v2/namespace"
	profilehandler "github.com/sergeyslonimsky/elara/internal/handler/v2/profile"
	tokenhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/token"
	transferhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/transfer"
	userhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/user"
	webhookhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/webhook"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1/authv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/capabilities/v1/capabilitiesv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/filter/v1/filterv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1/groupv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1/profilev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1/tokenv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1/userv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1/webhookv1connect"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

type V2Handlers struct {
	Config       *confighandler.ConfigHandler
	Schema       *confighandler.SchemaHandler
	Namespace    *namespacehandler.Handler
	Clients      *clientshandler.Handler
	Dashboard    *dashboardhandler.Handler
	Transfer     *transferhandler.Handler
	Webhook      *webhookhandler.Handler
	Filter       *filterhandler.Handler
	Auth         *authhandler.Handler
	Profile      *profilehandler.Handler
	Users        *userhandler.Handler
	Groups       *grouphandler.Handler
	Tokens       *tokenhandler.Handler
	Capabilities capabilitiesv1connect.CapabilitiesServiceHandler
}

func NewV2Handlers(s *Services, _ *sessions.Service, cfg config.Config) *V2Handlers {
	handlers := &V2Handlers{}

	initCoreHandlers(handlers, s)
	initAuthHandlers(handlers, s, cfg)
	initIAMHandlers(handlers, s, cfg)

	return handlers
}

func initCoreHandlers(handlers *V2Handlers, s *Services) {
	handlers.Config = confighandler.NewConfigHandler(s.Authz, s.Config)
	handlers.Schema = confighandler.NewSchemaHandler(s.Authz, s.Schema)
	handlers.Namespace = namespacehandler.New(s.Authz, s.Namespace)
	handlers.Clients = clientshandler.NewHandler(s.Authz, s.Clients)
	handlers.Dashboard = dashboardhandler.New(s.Dashboard)
	handlers.Transfer = transferhandler.New(s.Authz, s.Transfer)
	handlers.Webhook = webhookhandler.New(s.Authz, s.Webhook)
	handlers.Filter = filterhandler.New(s.Filter)
}

func initAuthHandlers(handlers *V2Handlers, s *Services, cfg config.Config) {
	handlers.Auth = authhandler.NewHandler(
		s.Auth,
		cfg.UI.Auth.Type,
		cfg.UI.Auth.Session.SecureCookie,
	)
	handlers.Profile = profilehandler.New(
		s.Profile,
		cfg.UI.Auth.Type,
		cfg.UI.Auth.Session.SecureCookie,
	)

	if cfg.Client.Auth.Enabled {
		handlers.Tokens = tokenhandler.New(s.Authz, s.Token)
	}

	handlers.Capabilities = capabilitieshandler.New(cfg.Client.Auth.Enabled, cfg.UI.Auth.Enabled, cfg.Demo.Enabled)
}

func initIAMHandlers(handlers *V2Handlers, s *Services, cfg config.Config) {
	if !cfg.UI.Auth.Enabled {
		return
	}

	handlers.Users = userhandler.New(s.User, cfg.UI.Auth.Type)
	handlers.Groups = grouphandler.NewHandler(s.Authz, s.Group)
}

type server interface {
	Mount(pattern string, handler http.Handler)
}

// userLookup is the consumer-side interface required by the auth interceptor.
// The concrete *bbolt.UserRepo satisfies it.
type userLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func V2Routes(
	server server,
	handlers *V2Handlers,
	sessionSvc *sessions.Service,
	users userLookup,
	cfg config.Config,
) {
	sharedInterceptors := []connect.Interceptor{
		interceptor.NewRecoveryInterceptor(),
		interceptor.NewLoggingInterceptor(),
		validate.NewInterceptor(),
	}

	publicInterceptors := slices.Clone(sharedInterceptors)
	privateInterceptors := slices.Clone(sharedInterceptors)

	if sessionSvc != nil {
		skip := !cfg.UI.Auth.Enabled || cfg.DangerouslySkipPermissions
		privateInterceptors = append(
			privateInterceptors,
			interceptor.NewAuthInterceptor(
				sessionSvc,
				users,
				interceptor.WithAuthSkipPermissions(skip),
			),
		)
	}

	publicOpts := connect.WithInterceptors(publicInterceptors...)
	privateOpts := connect.WithInterceptors(privateInterceptors...)

	// Public: auth service (login endpoints).
	path, handler := authv1connect.NewAuthServiceHandler(handlers.Auth, publicOpts)
	server.Mount(path, handler)

	// Private: all other services.
	path, handler = configv1connect.NewConfigServiceHandler(handlers.Config, privateOpts)
	server.Mount(path, handler)

	path, handler = namespacev1connect.NewNamespaceServiceHandler(handlers.Namespace, privateOpts)
	server.Mount(path, handler)

	path, handler = clientsv1connect.NewClientsServiceHandler(handlers.Clients, privateOpts)
	server.Mount(path, handler)

	path, handler = dashboardv1connect.NewDashboardServiceHandler(handlers.Dashboard, privateOpts)
	server.Mount(path, handler)

	path, handler = transferv1connect.NewTransferServiceHandler(handlers.Transfer, privateOpts)
	server.Mount(path, handler)

	path, handler = configv1connect.NewSchemaServiceHandler(handlers.Schema, privateOpts)
	server.Mount(path, handler)

	path, handler = webhookv1connect.NewWebhookServiceHandler(handlers.Webhook, privateOpts)
	server.Mount(path, handler)

	path, handler = profilev1connect.NewProfileServiceHandler(handlers.Profile, privateOpts)
	server.Mount(path, handler)

	path, handler = capabilitiesv1connect.NewCapabilitiesServiceHandler(handlers.Capabilities, privateOpts)
	server.Mount(path, handler)

	path, handler = filterv1connect.NewFilterServiceHandler(handlers.Filter, privateOpts)
	server.Mount(path, handler)

	if handlers.Users != nil {
		path, handler = userv1connect.NewUserServiceHandler(handlers.Users, privateOpts)
		server.Mount(path, handler)
	}

	if handlers.Groups != nil {
		path, handler = groupv1connect.NewGroupServiceHandler(handlers.Groups, privateOpts)
		server.Mount(path, handler)
	}

	if handlers.Tokens != nil {
		path, handler = tokenv1connect.NewTokenServiceHandler(handlers.Tokens, privateOpts)
		server.Mount(path, handler)
	}
}
