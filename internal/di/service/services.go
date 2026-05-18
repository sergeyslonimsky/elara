package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/schemavalidator"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	policyuc "github.com/sergeyslonimsky/elara/internal/usecase/policy"
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
	Policy    *policyuc.Service
	Token     *tokenuc.Service
	Auth      *authuc.Service

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
	sessionManager *auth.SessionManager,
) (*Services, error) {
	schemaValidator := schemavalidator.New(a.SchemaRepo)

	txm := bbolt.NewTxManager(a.Store.DB())
	pdp := authz.NewPDP(enforcer)
	authzSvc := authz.NewAuthz(pdp)
	adminBootstrap := auth.NewAdminBootstrap(txm, a.AuthUsers, a.AuthGroups, a.AuthPolicy)

	services := &Services{
		Config: configuc.New(
			enforcer,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		Namespace: nsuc.New(authzSvc, pdp, a.NamespaceRepo, a.Watch),
		Schema:    schemauc.New(enforcer, a.SchemaRepo, a.NamespaceRepo),
		Clients:   clientsuc.New(enforcer, a.ClientRegistry, a.ClientHistory),
		Dashboard: dashboarduc.New(
			enforcer,
			a.NamespaceRepo,
			a.ConfigRepo,
			a.ConfigRepo,
			a.ClientRegistry,
		),
		Transfer: transferuc.New(enforcer, a.ConfigRepo, a.NamespaceRepo),
		Webhook:  webhookuc.New(enforcer, a.WebhookRepo, a.WebhookDispatcher),
		Profile:  profileuc.New(enforcer, a.NamespaceRepo, a.AuthUsers, a.AuthUsers, sessionManager),
		User:     useruc.New(enforcer, a.AuthUsers, a.AuthUsers, txm),
		Group:    groupuc.New(enforcer, a.AuthGroups, txm, pdp),
		Policy:   policyuc.New(enforcer, a.AuthGroups, txm),
		Token:    tokenuc.New(enforcer, a.AuthTokens),

		AdminBootstrap: adminBootstrap,
	}

	if cfg.UI.Auth.Enabled {
		var oidcProvider *auth.OIDCProvider

		if cfg.UI.Auth.Type == config.AuthTypeOIDC {
			provider, err := auth.NewOIDCProvider(ctx, auth.OIDCConfig{
				IssuerURL:    cfg.UI.Auth.OIDC.IssuerURL,
				ClientID:     cfg.UI.Auth.OIDC.ClientID,
				ClientSecret: cfg.UI.Auth.OIDC.ClientSecret,
				RedirectURL:  cfg.UI.Auth.OIDC.RedirectURL,
				Scopes:       cfg.UI.Auth.OIDC.Scopes,
			})
			if err != nil {
				return nil, fmt.Errorf("create oidc provider: %w", err)
			}

			oidcProvider = provider
		}

		if cfg.UI.Auth.Type != config.AuthTypeNone {
			services.Auth = authuc.New(
				oidcProvider,
				a.AuthUsers,
				sessionManager,
				adminBootstrap,
				cfg.UI.Auth.AdminEmail,
			)
		}
	}

	return services, nil
}
