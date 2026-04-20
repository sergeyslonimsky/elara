package service

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
)

type V2Handlers struct {
	Config    *v2.ConfigHandler
	Namespace *v2.NamespaceHandler
	Clients   *v2.ClientsHandler
	Dashboard *v2.DashboardHandler
}

func NewV2Handlers(uc *UseCases) *V2Handlers {
	return &V2Handlers{
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
		),
		Namespace: v2.NewNamespaceHandler(
			uc.CreateNamespace,
			uc.GetNamespace,
			uc.UpdateNamespace,
			uc.ListNamespaces,
			uc.DeleteNamespace,
		),
		Clients:   v2.NewClientsHandler(uc.Clients),
		Dashboard: v2.NewDashboardHandler(uc.Dashboard),
	}
}

type server interface {
	Mount(pattern string, handler http.Handler)
}

func V2Routes(server server, handlers *V2Handlers) {
	opts := connect.WithInterceptors(
		interceptor.NewRecoveryInterceptor(),
		interceptor.NewLoggingInterceptor(),
		validate.NewInterceptor(),
	)

	path, handler := configv1connect.NewConfigServiceHandler(handlers.Config, opts)
	server.Mount(path, handler)

	path, handler = namespacev1connect.NewNamespaceServiceHandler(handlers.Namespace, opts)
	server.Mount(path, handler)

	path, handler = clientsv1connect.NewClientsServiceHandler(handlers.Clients, opts)
	server.Mount(path, handler)

	path, handler = dashboardv1connect.NewDashboardServiceHandler(handlers.Dashboard, opts)
	server.Mount(path, handler)
}
