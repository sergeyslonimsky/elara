package service

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1/authv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1/webhookv1connect"
)

type V2Handlers struct {
	Config    *v2.ConfigHandler
	Namespace *v2.NamespaceHandler
	Clients   *v2.ClientsHandler
	Dashboard *v2.DashboardHandler
	Transfer  *v2.TransferHandler
	Schema    *v2.SchemaHandler
	Webhook   *v2.WebhookHandler
	Auth      *v2.AuthHandler
	Users     *v2.UserHandler
	Groups    *v2.GroupHandler
	Access    *v2.AccessHandler
	Tokens    *v2.TokenHandler
}

func NewV2Handlers(uc *UseCases, cfg config.Config, sessionManager *auth.SessionManager) *V2Handlers {
	handlers := &V2Handlers{
		Config: v2.NewConfigHandler(
			uc.CreateConfig,
			uc.GetConfig,
			uc.UpdateConfig,
			uc.DeleteConfig,
			uc.ListConfigs,
			uc.ConfigHistory,
			uc.SearchConfigs,
			uc.CopyConfig,
			uc.ValidateConfig,
			uc.WatchConfigs,
			uc.ConfigDiff,
			uc.LockConfig,
			uc.UnlockConfig,
		),
		Namespace: v2.NewNamespaceHandler(
			uc.CreateNamespace,
			uc.GetNamespace,
			uc.UpdateNamespace,
			uc.ListNamespaces,
			uc.DeleteNamespace,
			uc.LockNamespace,
			uc.UnlockNamespace,
		),
		Clients:   v2.NewClientsHandler(uc.Clients),
		Dashboard: v2.NewDashboardHandler(uc.Dashboard),
		Transfer:  v2.NewTransferHandler(uc.ExportNamespace, uc.ExportAll, uc.ImportNamespace),
		Schema: v2.NewSchemaHandler(
			uc.AttachSchema,
			uc.DetachSchema,
			uc.GetSchema,
			uc.GetEffectiveSchema,
			uc.ListSchemas,
		),
		Webhook: v2.NewWebhookHandler(
			uc.CreateWebhook,
			uc.GetWebhook,
			uc.UpdateWebhook,
			uc.DeleteWebhook,
			uc.ListWebhooks,
			uc.WebhookHistory,
		),
	}

	if cfg.Auth.Enabled {
		handlers.Auth = v2.NewAuthHandler(uc.AuthLogin, uc.AuthCallback, uc.AuthMe)
		handlers.Users = v2.NewUserHandler(uc.AuthListUsers, uc.AuthGetUser)
		handlers.Groups = v2.NewGroupHandler(
			uc.AuthCreateGroup,
			uc.AuthGetGroup,
			uc.AuthUpdateGroup,
			uc.AuthDeleteGroup,
			uc.AuthListGroups,
			uc.AuthAddMember,
			uc.AuthRemoveMember,
		)
		handlers.Access = v2.NewAccessHandler(uc.AuthAssignRole, uc.AuthRevokeRole, uc.AuthListPolicies)
		handlers.Tokens = v2.NewTokenHandler(
			uc.AuthCreateToken,
			uc.AuthListTokens,
			uc.AuthGetToken,
			uc.AuthRevokeToken,
		)
	}

	return handlers
}

type server interface {
	Mount(pattern string, handler http.Handler)
}

func V2Routes(server server, handlers *V2Handlers, sessionManager *auth.SessionManager, cfg config.Config) {
	baseInterceptors := []connect.Interceptor{
		interceptor.NewRecoveryInterceptor(),
		interceptor.NewLoggingInterceptor(),
		validate.NewInterceptor(),
	}

	if cfg.Auth.Enabled && sessionManager != nil {
		publicProcedures := []string{
			"/elara.auth.v1.AuthService/Login",
			"/elara.auth.v1.AuthService/Callback",
			"/elara.auth.v1.AuthService/Logout",
		}
		baseInterceptors = append(baseInterceptors, interceptor.NewAuthInterceptor(sessionManager, publicProcedures))
	}

	opts := connect.WithInterceptors(baseInterceptors...)

	path, handler := configv1connect.NewConfigServiceHandler(handlers.Config, opts)
	server.Mount(path, handler)

	path, handler = namespacev1connect.NewNamespaceServiceHandler(handlers.Namespace, opts)
	server.Mount(path, handler)

	path, handler = clientsv1connect.NewClientsServiceHandler(handlers.Clients, opts)
	server.Mount(path, handler)

	path, handler = dashboardv1connect.NewDashboardServiceHandler(handlers.Dashboard, opts)
	server.Mount(path, handler)

	path, handler = transferv1connect.NewTransferServiceHandler(handlers.Transfer, opts)
	server.Mount(path, handler)

	path, handler = configv1connect.NewSchemaServiceHandler(handlers.Schema, opts)
	server.Mount(path, handler)

	path, handler = webhookv1connect.NewWebhookServiceHandler(handlers.Webhook, opts)
	server.Mount(path, handler)

	if handlers.Auth != nil {
		path, handler = authv1connect.NewAuthServiceHandler(handlers.Auth, opts)
		server.Mount(path, handler)
	}

	if handlers.Users != nil {
		path, handler = authv1connect.NewUserServiceHandler(handlers.Users, opts)
		server.Mount(path, handler)
	}

	if handlers.Groups != nil {
		path, handler = authv1connect.NewGroupServiceHandler(handlers.Groups, opts)
		server.Mount(path, handler)
	}

	if handlers.Access != nil {
		path, handler = authv1connect.NewAccessServiceHandler(handlers.Access, opts)
		server.Mount(path, handler)
	}

	if handlers.Tokens != nil {
		path, handler = authv1connect.NewTokenServiceHandler(handlers.Tokens, opts)
		server.Mount(path, handler)
	}
}
