package service

import (
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

// Repositories bundles every persistence-layer adapter — one field per
// domain entity. Each repo is a thin wrapper around the bbolt store that
// exposes domain-shaped read/write methods (Get, List, Upsert, Delete) and
// hides the bucket/key encoding.
//
// What belongs here:
//   - Adapters that translate between domain models and persisted bytes.
//   - Pure read/write code with no business rules.
//
// What does NOT belong here:
//   - Stateful primitives with lifecycle (pub/sub, monitors) → Infrastructure.
//   - Business logic (validation, authorization, cross-entity orchestration)
//     → UseCases. A repository never invokes another repository, never reads
//     from the enforcer, never publishes watch events. Those are usecase-level
//     concerns.
//   - Caching, rate-limiting, or any cross-cutting concern → UseCase or a
//     dedicated Service.
//
// Repositories depends only on the bbolt store from Infrastructure. It is the
// second layer assembled in Manager wiring, after external connections are
// open but before in-memory infrastructure primitives (Watch, monitors) —
// because monitor.HistoryStore writes through repos.ClientHistory.
type Repositories struct {
	Store *bbolt.Store

	Config        *bbolt.ConfigRepo
	Namespace     *bbolt.NamespaceRepo
	ClientHistory *bbolt.ClientHistoryRepo
	Schema        *bbolt.SchemaRepo
	Webhook       *bbolt.WebhookRepo
	AuthUsers     *bbolt.UserRepo
	AuthGroups    *bbolt.GroupRepo
	AuthTokens    *bbolt.TokenRepo
	AuthPolicy    *bbolt.PolicyRepo
}

// NewRepositories constructs every persistence adapter from a single bbolt
// store. Cheap — each constructor just captures the store pointer.
func NewRepositories(store *bbolt.Store) *Repositories {
	return &Repositories{
		Store:         store,
		Config:        bbolt.NewConfigRepo(store),
		Namespace:     bbolt.NewNamespaceRepo(store),
		ClientHistory: bbolt.NewClientHistoryRepo(store),
		Schema:        bbolt.NewSchemaRepo(store),
		Webhook:       bbolt.NewWebhookRepo(store),
		AuthUsers:     bbolt.NewUserRepo(store),
		AuthGroups:    bbolt.NewGroupRepo(store),
		AuthTokens:    bbolt.NewTokenRepo(store),
		AuthPolicy:    bbolt.NewPolicyRepo(store),
	}
}
