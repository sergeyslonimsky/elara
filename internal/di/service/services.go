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
func NewServices( //nolint:funlen // wiring-only: one constructor call per usecase
	ctx context.Context,
	a *Adapters,
	cfg config.Config,
	enforcer *casbin.Enforcer,
	sessionManager *auth.SessionManager,
) (*Services, error) {
	schemaValidator := schemavalidator.New(a.SchemaRepo)

	txm := bbolt.NewTxManager(a.Store.DB())
	pdp := authz.NewPDP(enforcer)
	pap := authz.NewPAP(enforcer, txm)
	scope := authz.NewScope(pdp, pap, a.AuthGroups)
	authzSvc := authz.NewAuthz(pdp)
	adminBootstrap := auth.NewAdminBootstrap(txm, a.AuthUsers, a.AuthGroups, a.AuthPolicy)

	services := &Services{
		Config: configuc.New(
			pdp,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		Namespace: nsuc.New(pdp, a.NamespaceRepo, a.Watch),
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
		Profile:  profileuc.New(pdp, a.AuthUsers, a.AuthUsers, sessionManager),
		User: useruc.New(
			useruc.NewBoltUserReader(a.AuthUsers),
			useruc.NewBoltGroupReader(a.AuthGroups),
			pdp,
			pap,
			scope,
		),
		Group:  groupuc.New(a.AuthGroups, pdp, pap, scope),
		Policy: policyuc.New(pap, a.AuthGroups),
		Token:  tokenuc.New(pdp, a.AuthTokens),

		Authz:          authzSvc,
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
				cfg.UI.Auth.OIDC.AdminEmail,
			)
		}
	}

	return services, nil
}
