package group

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Service orchestrates group lifecycle and Casbin role/membership rules.
//
// All mutating methods route through Enforcer.WriteTx so the bbolt write of
// the Group entity and the Casbin g-rules update commit (or roll back)
// atomically — preserving the §4 level-2 invariant from EL-4.
//
// store and enforcer are concrete pointers rather than interfaces because
// the per-tx views (bbolt.GroupRepo.WithTx, casbin.Enforcer.WithTx) return
// concrete types. Tests use a real bbolt + real Enforcer integration helper
// instead of mocks (see service_test.go).
type Service struct {
	enforcer *casbin.Enforcer
	store    *bbolt.GroupRepo
	txm      storage.TxManager
	pdp      *authz.PDP
}

func New(enforcer *casbin.Enforcer, store *bbolt.GroupRepo, txm storage.TxManager, pdp *authz.PDP) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
		txm:      txm,
		pdp:      pdp,
	}
}

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)

func diffPermissions(old, new []domain.Permission) (added, removed []domain.Permission) {
	oldMap := make(map[domain.Permission]struct{})
	for _, p := range old {
		oldMap[p] = struct{}{}
	}

	newMap := make(map[domain.Permission]struct{})
	for _, p := range new {
		newMap[p] = struct{}{}
	}

	for p := range newMap {
		if _, ok := oldMap[p]; !ok {
			added = append(added, p)
		}
	}

	for p := range oldMap {
		if _, ok := newMap[p]; !ok {
			removed = append(removed, p)
		}
	}

	return
}

func diffStrings(old, new []string) (added, removed []string) {
	oldMap := make(map[string]struct{})
	for _, s := range old {
		oldMap[s] = struct{}{}
	}

	newMap := make(map[string]struct{})
	for _, s := range new {
		newMap[s] = struct{}{}
	}

	for s := range newMap {
		if _, ok := oldMap[s]; !ok {
			added = append(added, s)
		}
	}

	for s := range oldMap {
		if _, ok := newMap[s]; !ok {
			removed = append(removed, s)
		}
	}

	return
}

func unionPermissions(a, b []domain.Permission) []domain.Permission {
	m := make(map[domain.Permission]struct{})
	for _, p := range a {
		m[p] = struct{}{}
	}

	for _, p := range b {
		m[p] = struct{}{}
	}

	var res []domain.Permission
	for p := range m {
		res = append(res, p)
	}

	return res
}
