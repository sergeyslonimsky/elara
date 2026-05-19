package service

import (
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

// UseCases groups every application-layer service that orchestrates business
// logic. Each usecase owns its minimal dependency interface; the struct here
// is just a registry to make wiring auditable from one place.
//
// A piece of code belongs in UseCases when it:
//   - Combines multiple repositories or repositories + infrastructure (e.g.,
//     write to bbolt + emit watch event + audit log).
//   - Enforces business rules: authorization (via Services.Enforcer),
//     validation (via Services.SchemaValidator), cross-entity invariants.
//   - Has a domain-shaped public API used by Handlers (RPC layer).
//
// What does NOT belong here:
//   - Proto/HTTP/gRPC translation → Handlers.
//   - Direct bbolt key encoding → Repositories.
//   - Background loops → Workers.
//
// UseCases consume from Repositories (persistence), Components (in-memory
// primitives: watch publisher, monitors), Services (stateless helpers), and
// Workers (e.g. dispatcher state queries). They are themselves consumed only
// by Handlers.
type UseCases struct {
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
	Auth      *authuc.Service // nil when UI auth disabled
}

// NewUseCases wires every domain usecase. authEnabled gates the Auth usecase
// (its OIDC dependency may be nil when auth is off — building it anyway would
// hide a future NPE behind the router skipping the mount).
func NewUseCases(
	repos *Repositories,
	components *Components,
	srv *Services,
	workers *Workers,
	authEnabled bool,
	adminEmail string,
) *UseCases {
	uc := &UseCases{
		Config: configuc.New(
			srv.Enforcer,
			repos.Config,
			components.Watch,
			repos.Namespace,
			srv.SchemaValidator,
		),
		Namespace: nsuc.New(srv.Authz, srv.PDP, repos.Namespace, components.Watch),
		Schema:    schemauc.New(srv.Enforcer, repos.Schema, repos.Namespace),
		Clients:   clientsuc.New(srv.Enforcer, components.ClientRegistry, components.ClientHistory),
		Dashboard: dashboarduc.New(
			srv.Enforcer,
			repos.Namespace,
			repos.Config,
			repos.Config,
			components.ClientRegistry,
		),
		Transfer: transferuc.New(srv.Enforcer, repos.Config, repos.Namespace),
		Webhook:  webhookuc.New(srv.Enforcer, repos.Webhook, workers.WebhookDispatch),
		Profile:  profileuc.New(srv.Enforcer, repos.Namespace, repos.AuthUsers, repos.AuthUsers, srv.Session),
		User:     useruc.New(srv.Enforcer, repos.AuthUsers, repos.AuthUsers, srv.TxM),
		Group:    groupuc.New(srv.Enforcer, repos.AuthGroups, srv.TxM, srv.PDP),
		Policy:   policyuc.New(srv.Enforcer, repos.AuthGroups, srv.TxM),
		Token:    tokenuc.New(srv.Enforcer, repos.AuthTokens),
	}

	if authEnabled {
		uc.Auth = authuc.New(srv.OIDC, repos.AuthUsers, srv.Session, srv.AdminBootstrap, adminEmail)
	}

	return uc
}
