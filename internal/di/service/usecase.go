package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
	transferuc "github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
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
	LockConfig     *configuc.LockUseCase
	UnlockConfig   *configuc.UnlockUseCase

	CreateNamespace *nsuc.CreateUseCase
	GetNamespace    *nsuc.GetUseCase
	UpdateNamespace *nsuc.UpdateUseCase
	ListNamespaces  *nsuc.ListUseCase
	DeleteNamespace *nsuc.DeleteUseCase
	LockNamespace   *nsuc.LockUseCase
	UnlockNamespace *nsuc.UnlockUseCase

	AttachSchema       *schemauc.AttachUseCase
	DetachSchema       *schemauc.DetachUseCase
	GetSchema          *schemauc.GetUseCase
	GetEffectiveSchema *schemauc.GetEffectiveUseCase
	ListSchemas        *schemauc.ListUseCase

	Clients   *clientsuc.UseCase
	Dashboard *dashboarduc.UseCase

	ExportNamespace *transferuc.ExportNamespaceUseCase
	ExportAll       *transferuc.ExportAllUseCase
	ImportNamespace *transferuc.ImportNamespaceUseCase

	CreateWebhook  *webhookuc.CreateUseCase
	GetWebhook     *webhookuc.GetUseCase
	UpdateWebhook  *webhookuc.UpdateUseCase
	DeleteWebhook  *webhookuc.DeleteUseCase
	ListWebhooks   *webhookuc.ListUseCase
	WebhookHistory *webhookuc.HistoryUseCase

	AuthLogin    *authuc.LoginUseCase
	AuthCallback *authuc.CallbackUseCase
	AuthMe       *authuc.MeUseCase

	AuthListUsers *authuc.ListUsersUseCase
	AuthGetUser   *authuc.GetUserUseCase

	AuthCreateGroup  *authuc.CreateGroupUseCase
	AuthGetGroup     *authuc.GetGroupUseCase
	AuthUpdateGroup  *authuc.UpdateGroupUseCase
	AuthDeleteGroup  *authuc.DeleteGroupUseCase
	AuthListGroups   *authuc.ListGroupsUseCase
	AuthAddMember    *authuc.AddMemberUseCase
	AuthRemoveMember *authuc.RemoveMemberUseCase

	AuthAssignRole   *authuc.AssignRoleUseCase
	AuthRevokeRole   *authuc.RevokeRoleUseCase
	AuthListPolicies *authuc.ListPoliciesUseCase

	AuthCreateToken *authuc.CreateTokenUseCase
	AuthListTokens  *authuc.ListTokensUseCase
	AuthGetToken    *authuc.GetTokenUseCase
	AuthRevokeToken *authuc.RevokeTokenUseCase
}

// NewUseCases creates all application use cases and returns the session manager separately
// so the handler layer can wire it without mixing infrastructure into UseCases.
func NewUseCases(ctx context.Context, a *Adapters, cfg config.Config) (*UseCases, *auth.SessionManager, error) {
	uc := newCoreUseCases(a)

	sessionManager := auth.NewSessionManager(cfg.Auth.Session.Secret, cfg.Auth.Session.TTL)

	if !cfg.Auth.Enabled {
		return uc, sessionManager, nil
	}

	if err := wireAuthUseCases(ctx, uc, a, cfg, sessionManager); err != nil {
		return nil, nil, err
	}

	return uc, sessionManager, nil
}

func newCoreUseCases(a *Adapters) *UseCases {
	schemaValidator := schemauc.NewValidateContentUseCase(a.SchemaRepo)

	return &UseCases{
		CreateConfig: configuc.NewCreateUseCase(
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			a.NamespaceRepo,
			schemaValidator,
		),
		GetConfig: configuc.NewGetUseCase(a.ConfigRepo),
		UpdateConfig: configuc.NewUpdateUseCase(
			a.ConfigRepo,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		DeleteConfig:   configuc.NewDeleteUseCase(a.ConfigRepo, a.Watch),
		ListConfigs:    configuc.NewListUseCase(a.ConfigRepo),
		ConfigHistory:  configuc.NewHistoryUseCase(a.ConfigRepo),
		SearchConfigs:  configuc.NewSearchUseCase(a.ConfigRepo),
		CopyConfig:     configuc.NewCopyUseCase(a.ConfigRepo, a.ConfigRepo, a.Watch, a.NamespaceRepo, a.NamespaceRepo),
		ValidateConfig: configuc.NewValidateUseCase(schemaValidator),
		WatchConfigs:   configuc.NewWatchUseCase(a.Watch),
		ConfigDiff:     configuc.NewDiffUseCase(a.ConfigRepo),
		LockConfig:     configuc.NewLockUseCase(a.ConfigRepo, a.Watch),
		UnlockConfig:   configuc.NewUnlockUseCase(a.ConfigRepo, a.Watch),

		CreateNamespace: nsuc.NewCreateUseCase(a.NamespaceRepo, a.NamespaceRepo),
		GetNamespace:    nsuc.NewGetUseCase(a.NamespaceRepo, a.NamespaceRepo),
		UpdateNamespace: nsuc.NewUpdateUseCase(a.NamespaceRepo, a.NamespaceRepo, a.NamespaceRepo),
		ListNamespaces:  nsuc.NewListUseCase(a.NamespaceRepo, a.NamespaceRepo),
		DeleteNamespace: nsuc.NewDeleteUseCase(a.NamespaceRepo, a.NamespaceRepo),
		LockNamespace:   nsuc.NewLockUseCase(a.NamespaceRepo, a.Watch),
		UnlockNamespace: nsuc.NewUnlockUseCase(a.NamespaceRepo, a.Watch),

		ExportNamespace: transferuc.NewExportNamespaceUseCase(a.ConfigRepo, a.NamespaceRepo),
		ExportAll:       transferuc.NewExportAllUseCase(a.ConfigRepo, a.NamespaceRepo),
		ImportNamespace: transferuc.NewImportNamespaceUseCase(
			a.ConfigRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.NamespaceRepo,
			a.NamespaceRepo,
		),

		AttachSchema:       schemauc.NewAttachUseCase(a.SchemaRepo, a.NamespaceRepo),
		DetachSchema:       schemauc.NewDetachUseCase(a.SchemaRepo, a.NamespaceRepo),
		GetSchema:          schemauc.NewGetUseCase(a.SchemaRepo),
		GetEffectiveSchema: schemauc.NewGetEffectiveUseCase(a.SchemaRepo),
		ListSchemas:        schemauc.NewListUseCase(a.SchemaRepo),

		Clients: clientsuc.NewUseCase(a.ClientRegistry, a.ClientHistory),
		Dashboard: dashboarduc.NewUseCase(
			a.NamespaceRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ClientRegistry,
		),

		CreateWebhook:  webhookuc.NewCreateUseCase(a.WebhookRepo),
		GetWebhook:     webhookuc.NewGetUseCase(a.WebhookRepo),
		UpdateWebhook:  webhookuc.NewUpdateUseCase(a.WebhookRepo),
		DeleteWebhook:  webhookuc.NewDeleteUseCase(a.WebhookRepo, a.WebhookDispatcher),
		ListWebhooks:   webhookuc.NewListUseCase(a.WebhookRepo),
		WebhookHistory: webhookuc.NewHistoryUseCase(a.WebhookDispatcher),
	}
}

func wireAuthUseCases(
	ctx context.Context,
	uc *UseCases,
	a *Adapters,
	cfg config.Config,
	sessionManager *auth.SessionManager,
) error {
	enforcer, err := casbin.NewEnforcer(ctx, a.AuthPolicy)
	if err != nil {
		return fmt.Errorf("create casbin enforcer: %w", err)
	}

	oidcProvider, err := auth.NewOIDCProvider(ctx, auth.OIDCConfig{
		IssuerURL:    cfg.Auth.OIDC.IssuerURL,
		ClientID:     cfg.Auth.OIDC.ClientID,
		ClientSecret: cfg.Auth.OIDC.ClientSecret,
		RedirectURL:  cfg.Auth.OIDC.RedirectURL,
		Scopes:       cfg.Auth.OIDC.Scopes,
	})
	if err != nil {
		return fmt.Errorf("create oidc provider: %w", err)
	}

	uc.AuthLogin = authuc.NewLoginUseCase(oidcProvider)
	uc.AuthCallback = authuc.NewCallbackUseCase(
		oidcProvider,
		a.AuthUsers,
		sessionManager,
		enforcer,
		a.AuthPolicy,
		cfg.Auth.AdminEmails,
	)
	uc.AuthMe = authuc.NewMeUseCase(enforcer)

	uc.AuthListUsers = authuc.NewListUsersUseCase(a.AuthUsers)
	uc.AuthGetUser = authuc.NewGetUserUseCase(a.AuthUsers)

	uc.AuthCreateGroup = authuc.NewCreateGroupUseCase(a.AuthGroups)
	uc.AuthGetGroup = authuc.NewGetGroupUseCase(a.AuthGroups)
	uc.AuthUpdateGroup = authuc.NewUpdateGroupUseCase(a.AuthGroups)
	uc.AuthDeleteGroup = authuc.NewDeleteGroupUseCase(a.AuthGroups)
	uc.AuthListGroups = authuc.NewListGroupsUseCase(a.AuthGroups)
	uc.AuthAddMember = authuc.NewAddMemberUseCase(a.AuthGroups)
	uc.AuthRemoveMember = authuc.NewRemoveMemberUseCase(a.AuthGroups)

	uc.AuthAssignRole = authuc.NewAssignRoleUseCase(enforcer, a.AuthPolicy)
	uc.AuthRevokeRole = authuc.NewRevokeRoleUseCase(enforcer, a.AuthPolicy)
	uc.AuthListPolicies = authuc.NewListPoliciesUseCase(enforcer)

	uc.AuthCreateToken = authuc.NewCreateTokenUseCase(a.AuthTokens)
	uc.AuthListTokens = authuc.NewListTokensUseCase(a.AuthTokens)
	uc.AuthGetToken = authuc.NewGetTokenUseCase(a.AuthTokens)
	uc.AuthRevokeToken = authuc.NewRevokeTokenUseCase(a.AuthTokens)

	return nil
}
