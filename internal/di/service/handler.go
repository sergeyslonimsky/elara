package service

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	accesshandler "github.com/sergeyslonimsky/elara/internal/handler/v2/access"
	authhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/auth"
	clientshandler "github.com/sergeyslonimsky/elara/internal/handler/v2/clients"
	confighandler "github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	dashboardhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/dashboard"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	namespacehandler "github.com/sergeyslonimsky/elara/internal/handler/v2/namespace"
	profilehandler "github.com/sergeyslonimsky/elara/internal/handler/v2/profile"
	tokenhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/token"
	transferhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/transfer"
	userhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/user"
	webhookhandler "github.com/sergeyslonimsky/elara/internal/handler/v2/webhook"
	accessv1connect "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1/accessv1connect"
	authv1connect "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1/authv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	profilev1connect "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1/profilev1connect"
	tokenv1connect "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1/tokenv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	userv1connect "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1/userv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1/webhookv1connect"
)

type V2Handlers struct {
	Config    *confighandler.Handler
	Schema    *confighandler.SchemaHandler
	Namespace *namespacehandler.Handler
	Clients   *clientshandler.Handler
	Dashboard *dashboardhandler.Handler
	Transfer  *transferhandler.Handler
	Webhook   *webhookhandler.Handler
	Auth      *authhandler.Handler
	Profile   *profilehandler.Handler
	Users     *userhandler.Handler
	Groups    *accesshandler.GroupHandler
	Access    *accesshandler.AccessHandler
	Tokens    *tokenhandler.Handler
}

func NewV2Handlers(uc *UseCases, cfg config.Config) *V2Handlers {
	handlers := &V2Handlers{}

	initCoreHandlers(handlers, uc)
	initAuthHandlers(handlers, uc, cfg)
	initIAMHandlers(handlers, uc, cfg)

	return handlers
}

func initCoreHandlers(handlers *V2Handlers, uc *UseCases) {
	handlers.Config = confighandler.New(uc.Config)
	handlers.Schema = confighandler.NewSchemaHandler(
		uc.AttachSchema,
		uc.DetachSchema,
		uc.GetSchema,
		uc.GetEffectiveSchema,
		uc.ListSchemas,
	)
	handlers.Namespace = namespacehandler.New(
		uc.CreateNamespace,
		uc.GetNamespace,
		uc.UpdateNamespace,
		uc.ListNamespaces,
		uc.DeleteNamespace,
		uc.LockNamespace,
		uc.UnlockNamespace,
	)
	handlers.Clients = clientshandler.New(uc.Clients)
	handlers.Dashboard = dashboardhandler.New(uc.Dashboard)
	handlers.Transfer = transferhandler.New(uc.ExportNamespace, uc.ExportAll, uc.ImportNamespace)
	handlers.Webhook = webhookhandler.New(
		uc.CreateWebhook,
		uc.GetWebhook,
		uc.UpdateWebhook,
		uc.DeleteWebhook,
		uc.ListWebhooks,
		uc.WebhookHistory,
	)
}

func initAuthHandlers(handlers *V2Handlers, uc *UseCases, cfg config.Config) {
	handlers.Auth = authhandler.New(
		uc.AuthLogin,
		uc.AuthCallback,
		uc.AuthBasicLogin,
		cfg.UI.Auth.Type,
		cfg.UI.Auth.Session.SecureCookie,
	)
	handlers.Profile = profilehandler.New(
		uc.AuthMe,
		uc.AuthChangePassword,
		cfg.UI.Auth.Type,
		cfg.UI.Auth.Session.SecureCookie,
	)

	if cfg.Client.Auth.Enabled {
		handlers.Tokens = tokenhandler.New(
			uc.AuthCreateToken,
			uc.AuthListTokens,
			uc.AuthGetToken,
			uc.AuthRevokeToken,
		)
	}
}

func initIAMHandlers(handlers *V2Handlers, uc *UseCases, cfg config.Config) {
	if !cfg.UI.Auth.Enabled {
		return
	}

	handlers.Users = userhandler.New(
		uc.AuthListUsers,
		uc.AuthGetUser,
		uc.AuthCreateUser,
		uc.AuthResetPassword,
		uc.AuthDeleteUser,
		cfg.UI.Auth.Type,
	)
	handlers.Groups = accesshandler.NewGroupHandler(
		uc.AuthCreateGroup,
		uc.AuthGetGroup,
		uc.AuthUpdateGroup,
		uc.AuthDeleteGroup,
		uc.AuthListGroups,
		uc.AuthAddMember,
		uc.AuthRemoveMember,
	)
	handlers.Access = accesshandler.NewAccessHandler(uc.AuthAssignRole, uc.AuthRevokeRole, uc.AuthListPolicies)
}

type server interface {
	Mount(pattern string, handler http.Handler)
}

func V2Routes(server server, handlers *V2Handlers, sessionManager *auth.SessionManager, cfg config.Config) {
	sharedInterceptors := []connect.Interceptor{
		interceptor.NewRecoveryInterceptor(),
		interceptor.NewLoggingInterceptor(),
		validate.NewInterceptor(),
	}

	// AuthService is public — no auth interceptor.
	privateInterceptors := append(
		sharedInterceptors[:len(sharedInterceptors):len(sharedInterceptors)],
		func() []connect.Interceptor {
			if cfg.UI.Auth.Enabled && sessionManager != nil {
				return []connect.Interceptor{interceptor.NewAuthInterceptor(sessionManager)}
			}

			return nil
		}()...)

	publicOpts := connect.WithInterceptors(sharedInterceptors...)
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

	if handlers.Users != nil {
		path, handler = userv1connect.NewUserServiceHandler(handlers.Users, privateOpts)
		server.Mount(path, handler)
	}

	if handlers.Groups != nil {
		path, handler = accessv1connect.NewGroupServiceHandler(handlers.Groups, privateOpts)
		server.Mount(path, handler)
	}

	if handlers.Access != nil {
		path, handler = accessv1connect.NewAccessServiceHandler(handlers.Access, privateOpts)
		server.Mount(path, handler)
	}

	if handlers.Tokens != nil {
		path, handler = tokenv1connect.NewTokenServiceHandler(handlers.Tokens, privateOpts)
		server.Mount(path, handler)
	}
}
