package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
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

	AuthLogin          *authuc.LoginUseCase
	AuthCallback       *authuc.CallbackUseCase
	AuthMe             *authuc.MeUseCase
	AuthBasicLogin     *authuc.BasicLoginUseCase
	AuthChangePassword *authuc.ChangePasswordUseCase
	AuthResetPassword  *authuc.ResetPasswordUseCase
	AuthCreateUser     *authuc.CreateUserUseCase
	AuthDeleteUser     *authuc.DeleteUserUseCase

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
	sessionManager := auth.NewSessionManager(cfg.UI.Auth.Session.Secret, cfg.UI.Auth.Session.TTL)

	enforcer, err := casbin.NewEnforcer(a.AuthPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	uc := newCoreUseCases(a, enforcer)

	// MeUseCase is always available — it uses the enforcer to resolve per-user permissions.
	uc.AuthMe = authuc.NewMeUseCase(enforcer, a.NamespaceRepo)

	if !cfg.UI.Auth.Enabled {
		if err := enforcer.SeedPassthroughAdmin(); err != nil {
			return nil, nil, fmt.Errorf("seed passthrough admin: %w", err)
		}
	}

	if cfg.UI.Auth.Enabled {
		if err := wireUIAuth(ctx, uc, a, cfg, sessionManager, enforcer); err != nil {
			return nil, nil, err
		}
	}

	if cfg.UI.Auth.Enabled || cfg.Client.Auth.Enabled {
		wireTokenUseCases(uc, a, enforcer)
	}

	return uc, sessionManager, nil
}

func wireUIAuth(
	ctx context.Context,
	uc *UseCases,
	a *Adapters,
	cfg config.Config,
	sessionManager *auth.SessionManager,
	enforcer *casbin.Enforcer,
) error {
	if err := wireUIAuthUseCases(ctx, uc, a, cfg, sessionManager, enforcer); err != nil {
		return err
	}

	if cfg.UI.Auth.Type == config.AuthTypeBasicAuth {
		if err := bootstrapBasicAuthAdmin(ctx, a, cfg); err != nil {
			return err
		}
	}

	if email := cfg.UI.Auth.AdminEmail; email != "" {
		if err := enforcer.AddRoleForUser(email, auth.RoleAdmin, auth.ObjectAll); err != nil {
			return fmt.Errorf("bootstrap admin role %q: %w", email, err)
		}
		if err := enforcer.AddPolicy(email, auth.ObjectAll, auth.ObjectAll, auth.ActionAll); err != nil {
			return fmt.Errorf("bootstrap admin policy %q: %w", email, err)
		}
	}

	return nil
}

func bootstrapBasicAuthAdmin(
	ctx context.Context,
	a *Adapters,
	cfg config.Config,
) error {
	email := cfg.UI.Auth.AdminEmail
	if email == "" {
		return nil
	}

	_, err := a.AuthUsers.Get(ctx, email)
	if err == nil {
		return nil // Already exists
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("check admin user existence: %w", err)
	}

	// Create initial admin user
	user := &domain.User{
		Email:    email,
		Name:     "Administrator",
		Provider: domain.ProviderBasicAuth,
	}

	if err := a.AuthUsers.Upsert(ctx, user); err != nil {
		return fmt.Errorf("bootstrap admin upsert: %w", err)
	}

	hash, err := auth.HashPassword(cfg.UI.Auth.BasicAuth.AdminInitialPassword)
	if err != nil {
		return fmt.Errorf("bootstrap admin hash: %w", err)
	}

	if err := a.AuthUsers.SetPassword(ctx, email, hash, true); err != nil {
		return fmt.Errorf("bootstrap admin set password: %w", err)
	}

	return nil
}

func newCoreUseCases(a *Adapters, enforcer *casbin.Enforcer) *UseCases {
	schemaValidator := schemauc.NewValidateContentUseCase(a.SchemaRepo)

	return &UseCases{
		CreateConfig: configuc.NewCreateUseCase(
			enforcer,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			a.NamespaceRepo,
			schemaValidator,
		),
		GetConfig: configuc.NewGetUseCase(enforcer, a.ConfigRepo),
		UpdateConfig: configuc.NewUpdateUseCase(
			enforcer,
			a.ConfigRepo,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		DeleteConfig:  configuc.NewDeleteUseCase(enforcer, a.ConfigRepo, a.Watch),
		ListConfigs:   configuc.NewListUseCase(enforcer, a.ConfigRepo),
		ConfigHistory: configuc.NewHistoryUseCase(enforcer, a.ConfigRepo),
		SearchConfigs: configuc.NewSearchUseCase(enforcer, a.ConfigRepo),
		CopyConfig: configuc.NewCopyUseCase(
			enforcer,
			a.ConfigRepo,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			a.NamespaceRepo,
		),
		ValidateConfig: configuc.NewValidateUseCase(enforcer, schemaValidator),
		WatchConfigs:   configuc.NewWatchUseCase(enforcer, a.Watch),
		ConfigDiff:     configuc.NewDiffUseCase(enforcer, a.ConfigRepo),
		LockConfig:     configuc.NewLockUseCase(enforcer, a.ConfigRepo, a.Watch),
		UnlockConfig:   configuc.NewUnlockUseCase(enforcer, a.ConfigRepo, a.Watch),

		CreateNamespace: nsuc.NewCreateUseCase(enforcer, a.NamespaceRepo, a.NamespaceRepo),
		GetNamespace:    nsuc.NewGetUseCase(enforcer, a.NamespaceRepo, a.NamespaceRepo),
		UpdateNamespace: nsuc.NewUpdateUseCase(enforcer, a.NamespaceRepo, a.NamespaceRepo, a.NamespaceRepo),
		ListNamespaces:  nsuc.NewListUseCase(enforcer, a.NamespaceRepo, a.NamespaceRepo),
		DeleteNamespace: nsuc.NewDeleteUseCase(enforcer, a.NamespaceRepo, a.NamespaceRepo),
		LockNamespace:   nsuc.NewLockUseCase(enforcer, a.NamespaceRepo, a.Watch),
		UnlockNamespace: nsuc.NewUnlockUseCase(enforcer, a.NamespaceRepo, a.Watch),

		ExportNamespace: transferuc.NewExportNamespaceUseCase(enforcer, a.ConfigRepo, a.NamespaceRepo),
		ExportAll:       transferuc.NewExportAllUseCase(enforcer, a.ConfigRepo, a.NamespaceRepo),
		ImportNamespace: transferuc.NewImportNamespaceUseCase(
			enforcer,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.NamespaceRepo,
			a.NamespaceRepo,
		),

		AttachSchema:       schemauc.NewAttachUseCase(enforcer, a.SchemaRepo, a.NamespaceRepo),
		DetachSchema:       schemauc.NewDetachUseCase(enforcer, a.SchemaRepo, a.NamespaceRepo),
		GetSchema:          schemauc.NewGetUseCase(enforcer, a.SchemaRepo),
		GetEffectiveSchema: schemauc.NewGetEffectiveUseCase(enforcer, a.SchemaRepo),
		ListSchemas:        schemauc.NewListUseCase(enforcer, a.SchemaRepo),

		Clients: clientsuc.NewUseCase(enforcer, a.ClientRegistry, a.ClientHistory),
		Dashboard: dashboarduc.NewUseCase(
			enforcer,
			a.NamespaceRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ClientRegistry,
		),

		CreateWebhook:  webhookuc.NewCreateUseCase(enforcer, a.WebhookRepo),
		GetWebhook:     webhookuc.NewGetUseCase(enforcer, a.WebhookRepo),
		UpdateWebhook:  webhookuc.NewUpdateUseCase(enforcer, a.WebhookRepo),
		DeleteWebhook:  webhookuc.NewDeleteUseCase(enforcer, a.WebhookRepo, a.WebhookRepo, a.WebhookDispatcher),
		ListWebhooks:   webhookuc.NewListUseCase(enforcer, a.WebhookRepo),
		WebhookHistory: webhookuc.NewHistoryUseCase(enforcer, a.WebhookDispatcher, a.WebhookRepo),
	}
}

func wireUIAuthUseCases(
	ctx context.Context,
	uc *UseCases,
	a *Adapters,
	cfg config.Config,
	sessionManager *auth.SessionManager,
	enforcer *casbin.Enforcer,
) error {
	switch cfg.UI.Auth.Type {
	case config.AuthTypeOIDC:
		oidcProvider, err := auth.NewOIDCProvider(ctx, auth.OIDCConfig{
			IssuerURL:    cfg.UI.Auth.OIDC.IssuerURL,
			ClientID:     cfg.UI.Auth.OIDC.ClientID,
			ClientSecret: cfg.UI.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.UI.Auth.OIDC.RedirectURL,
			Scopes:       cfg.UI.Auth.OIDC.Scopes,
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
			cfg.UI.Auth.AdminEmail,
		)
	case config.AuthTypeBasicAuth:
		uc.AuthBasicLogin = authuc.NewBasicLoginUseCase(a.AuthUsers, sessionManager, enforcer, cfg.UI.Auth.AdminEmail)
		uc.AuthChangePassword = authuc.NewChangePasswordUseCase(a.AuthUsers, a.AuthUsers, sessionManager)
		uc.AuthResetPassword = authuc.NewResetPasswordUseCase(enforcer, a.AuthUsers)
		uc.AuthDeleteUser = authuc.NewDeleteUserUseCase(enforcer, a.AuthUsers)
	case config.AuthTypeNone:
		// No specific use cases to wire for AuthTypeNone
	}

	if cfg.UI.Auth.Type != config.AuthTypeNone {
		uc.AuthCreateUser = authuc.NewCreateUserUseCase(enforcer, a.AuthUsers)
	}

	uc.AuthListUsers = authuc.NewListUsersUseCase(enforcer, a.AuthUsers)
	uc.AuthGetUser = authuc.NewGetUserUseCase(enforcer, a.AuthUsers)

	uc.AuthCreateGroup = authuc.NewCreateGroupUseCase(enforcer, a.AuthGroups)
	uc.AuthGetGroup = authuc.NewGetGroupUseCase(enforcer, a.AuthGroups)
	uc.AuthUpdateGroup = authuc.NewUpdateGroupUseCase(enforcer, enforcer, a.AuthGroups)
	uc.AuthDeleteGroup = authuc.NewDeleteGroupUseCase(enforcer, enforcer, a.AuthGroups)
	uc.AuthListGroups = authuc.NewListGroupsUseCase(enforcer, a.AuthGroups)
	uc.AuthAddMember = authuc.NewAddMemberUseCase(enforcer, enforcer, a.AuthGroups)
	uc.AuthRemoveMember = authuc.NewRemoveMemberUseCase(enforcer, enforcer, a.AuthGroups)

	uc.AuthAssignRole = authuc.NewAssignRoleUseCase(enforcer, a.AuthGroups)
	uc.AuthRevokeRole = authuc.NewRevokeRoleUseCase(enforcer, a.AuthGroups)
	uc.AuthListPolicies = authuc.NewListPoliciesUseCase(enforcer)

	return nil
}

func wireTokenUseCases(uc *UseCases, a *Adapters, enforcer *casbin.Enforcer) {
	uc.AuthCreateToken = authuc.NewCreateTokenUseCase(enforcer, a.AuthTokens)
	uc.AuthListTokens = authuc.NewListTokensUseCase(enforcer, a.AuthTokens)
	uc.AuthGetToken = authuc.NewGetTokenUseCase(enforcer, a.AuthTokens)
	uc.AuthRevokeToken = authuc.NewRevokeTokenUseCase(enforcer, a.AuthTokens, a.AuthTokens)
}
