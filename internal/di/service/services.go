package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
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

	services := &Services{
		Config: configuc.New(
			enforcer,
			a.ConfigRepo,
			a.Watch,
			a.NamespaceRepo,
			schemaValidator,
		),
		Namespace: nsuc.New(enforcer, a.NamespaceRepo, a.Watch),
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
		User:     useruc.New(enforcer, a.AuthUsers),
		Group:    groupuc.New(enforcer, enforcer, a.AuthGroups),
		Policy:   policyuc.New(enforcer, a.AuthGroups),
		Token:    tokenuc.New(enforcer, a.AuthTokens),
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
			services.Auth = authuc.New(oidcProvider, a.AuthUsers, sessionManager, enforcer, cfg.UI.Auth.AdminEmail)
		}
	}

	return services, nil
}
