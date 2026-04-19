package service

import (
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

type UseCases struct {
	CreateConfig   *configuc.CreateUseCase
	GetConfig      *configuc.GetUseCase
	UpdateConfig   *configuc.UpdateUseCase
	DeleteConfig   *configuc.DeleteUseCase
	ListConfigs    *configuc.ListUseCase
	ConfigHistory  *configuc.HistoryUseCase
	SearchConfigs  *configuc.SearchUseCase
	CopyConfig     *configuc.CopyUseCase
	ValidateConfig *configuc.ValidateUseCase
	WatchConfigs   *configuc.WatchUseCase
	ConfigDiff     *configuc.DiffUseCase

	CreateNamespace *nsuc.CreateUseCase
	GetNamespace    *nsuc.GetUseCase
	UpdateNamespace *nsuc.UpdateUseCase
	ListNamespaces  *nsuc.ListUseCase
	DeleteNamespace *nsuc.DeleteUseCase

	Clients   *clientsuc.UseCase
	Dashboard *dashboarduc.UseCase
}

func NewUseCases(a *Adapters) *UseCases {
	return &UseCases{
		CreateConfig:   configuc.NewCreateUseCase(a.ConfigRepo, a.Watch, a.NamespaceRepo, a.NamespaceRepo),
		GetConfig:      configuc.NewGetUseCase(a.ConfigRepo),
		UpdateConfig:   configuc.NewUpdateUseCase(a.ConfigRepo, a.ConfigRepo, a.Watch, a.NamespaceRepo),
		DeleteConfig:   configuc.NewDeleteUseCase(a.ConfigRepo, a.Watch),
		ListConfigs:    configuc.NewListUseCase(a.ConfigRepo),
		ConfigHistory:  configuc.NewHistoryUseCase(a.ConfigRepo),
		SearchConfigs:  configuc.NewSearchUseCase(a.ConfigRepo),
		CopyConfig:     configuc.NewCopyUseCase(a.ConfigRepo, a.ConfigRepo, a.Watch, a.NamespaceRepo, a.NamespaceRepo),
		ValidateConfig: configuc.NewValidateUseCase(),
		WatchConfigs:   configuc.NewWatchUseCase(a.Watch),
		ConfigDiff:     configuc.NewDiffUseCase(a.ConfigRepo),

		CreateNamespace: nsuc.NewCreateUseCase(a.NamespaceRepo, a.NamespaceRepo),
		GetNamespace:    nsuc.NewGetUseCase(a.NamespaceRepo, a.NamespaceRepo),
		UpdateNamespace: nsuc.NewUpdateUseCase(a.NamespaceRepo, a.NamespaceRepo, a.NamespaceRepo),
		ListNamespaces:  nsuc.NewListUseCase(a.NamespaceRepo, a.NamespaceRepo),
		DeleteNamespace: nsuc.NewDeleteUseCase(a.NamespaceRepo, a.NamespaceRepo),

		Clients: clientsuc.NewUseCase(a.ClientRegistry, a.ClientHistory),
		Dashboard: dashboarduc.NewUseCase(
			a.NamespaceRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ClientRegistry,
		),
	}
}
