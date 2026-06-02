package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/schemavalidator"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	filteruc "github.com/sergeyslonimsky/elara/internal/usecase/filter"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	profileuc "github.com/sergeyslonimsky/elara/internal/usecase/profile"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
	transferuc "github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	useruc "github.com/sergeyslonimsky/elara/internal/usecase/user"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

// Services bundles every application service. One field per domain — handlers
// take whichever ones they need. Auth is nil when UI auth is disabled.
type Services struct {
	Config    *configuc.Service
	Namespace *nsuc.Service
	Schema    *schemauc.Service
	Clients   *clientsuc.Service
	Dashboard *dashboarduc.Service
	Transfer  *transferuc.Service
	Webhook   *webhookuc.Service
	Profile   *profileuc.Service
	User      *useruc.Service
	Group     *groupuc.Service
	Token     *tokenuc.Service
	Auth      *authuc.Service
	Filter    *filteruc.Service

	// Authz is the shared per-RPC authorization gate used by v2 handlers.
	// It is exposed on Services so V2Handlers wiring can pass it into each
	// handler that needs Require(...).
	Authz *authz.Authz

	// AdminBootstrap is exposed so cmd/service/main.go can run the idempotent
	// superadmin seed before the HTTP/gRPC listeners come up. Lives on Services
	// rather than Manager because it depends on the same repos as the usecases.
	AdminBootstrap *auth.AdminBootstrap
}

// NewServices constructs every domain service. Pure wiring: no DB writes,
// no policy seeding — those live in Bootstrap.
func NewServices(
	ctx context.Context,
	a *Adapters,
	cfg config.Config,
	enforcer *casbin.Enforcer,
	sessionSvc *sessions.Service,
) (*Services, error) {
	schemaValidator := schemavalidator.New(a.SchemaRepo)

	pdp := authz.NewPDP(enforcer, authz.WithSkipPermissions(cfg.DangerouslySkipPermissions))
	pap := authz.NewPAP(enforcer, a.StorageManager)
	scope := authz.NewScope(pdp, pap, a.AuthGroups)
	authzSvc := authz.NewAuthz(pdp)
	userSvc := auth.NewUserService(a.AuthUsers)
	adminBootstrap := auth.NewAdminBootstrap(a.StorageManager, userSvc, a.AuthGroups, a.AuthPolicy)

	services := &Services{
		Config: configuc.New(
			a.StorageManager,
			pdp,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		Namespace: nsuc.New(a.StorageManager, pdp, a.NamespaceRepo, a.Watch),
		Schema:    schemauc.New(pdp, a.SchemaRepo, a.NamespaceRepo),
		Clients:   clientsuc.New(pdp, a.ClientRegistry, a.ClientHistory),
		Dashboard: dashboarduc.New(
			pdp,
			a.NamespaceRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ClientRegistry,
		),
		Transfer: transferuc.New(pdp, a.ConfigRepo, a.NamespaceRepo),
		Webhook:  webhookuc.New(pdp, a.WebhookRepo, a.WebhookDispatcher),
		Profile:  profileuc.New(a.StorageManager, pdp, a.AuthUsers, a.AuthUsers, sessionSvc),
		User: useruc.New(
			a.StorageManager,
			a.AuthUsers,
			userSvc,
			a.AuthGroups,
			sessionSvc,
			pdp,
			pap,
			scope,
		),
		Group:  groupuc.New(a.StorageManager, a.AuthGroups, pdp, pap, scope),
		Token:  tokenuc.New(pdp, a.AuthTokens),
		Filter: filteruc.New(pdp, a.NamespaceRepo, a.AuthGroups, a.AuthUsers),

		Authz:          authzSvc,
		AdminBootstrap: adminBootstrap,
	}

	if err := configureAuthService(ctx, services, a, cfg, userSvc, adminBootstrap, sessionSvc); err != nil {
		return nil, err
	}

	return services, nil
}

// configureAuthService wires the auth usecase (and its OIDC provider, when
// configured) onto services when UI auth is enabled. Split out of NewServices
// to keep the constructor focused on plain dependency wiring.
func configureAuthService(
	ctx context.Context,
	services *Services,
	a *Adapters,
	cfg config.Config,
	userSvc *auth.UserService,
	adminBootstrap *auth.AdminBootstrap,
	sessionSvc *sessions.Service,
) error {
	if !cfg.UI.Auth.Enabled || cfg.UI.Auth.Type == domain.AuthTypeNone {
		return nil
	}

	var oidcProvider *auth.OIDCProvider

	if cfg.UI.Auth.Type == domain.AuthTypeOIDC {
		provider, err := auth.NewOIDCProvider(ctx, auth.OIDCConfig{
			IssuerURL:    cfg.UI.Auth.OIDC.IssuerURL,
			ClientID:     cfg.UI.Auth.OIDC.ClientID,
			ClientSecret: cfg.UI.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.UI.Auth.OIDC.RedirectURL,
			Scopes:       cfg.UI.Auth.OIDC.Scopes,
		})
		if err != nil {
			return fmt.Errorf("create oidc provider: %w", err)
		}

		oidcProvider = provider
	}

	services.Auth = authuc.New(
		a.StorageManager,
		oidcProvider,
		userSvc,
		adminBootstrap,
		sessionSvc,
		cfg.UI.Auth.OIDC.AdminEmail,
	)

	return nil
}
