package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Service orchestrates group lifecycle and Casbin role/membership rules.
//
// All mutating methods route through PAP.Write so the bbolt write of the
// Group entity and the Casbin g/p-rule updates commit (or roll back)
// atomically.
//
// store is a concrete *bbolt.GroupRepo because the per-tx view
// (GroupRepo.WithTx) returns a concrete type. Tests use the real
// bbolt + Casbin integration helper instead of mocks (see service_test.go).
type Service struct {
	store *bbolt.GroupRepo
	pdp   *authz.PDP
	pap   *authz.PAP
	scope *authz.Scope
}

func New(
	store *bbolt.GroupRepo,
	pdp *authz.PDP,
	pap *authz.PAP,
	scope *authz.Scope,
) *Service {
	return &Service{
		store: store,
		pdp:   pdp,
		pap:   pap,
		scope: scope,
	}
}

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)

const defaultListLimit = 20

// loadMutableGroup fetches a group within the given tx and rejects system
// groups via EnsureMutable. Shared by Update*, Delete, and Get-then-mutate
// flows. The three Update* methods perform their own version check on the
// relevant counter (metadata / members / permissions); this helper is
// version-agnostic.
func (s *Service) loadMutableGroup(
	ctx context.Context,
	tx storage.Tx,
	id string,
) (*domain.Group, error) {
	existing, err := s.store.WithTx(tx).Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err := existing.EnsureMutable(); err != nil {
		return nil, fmt.Errorf("ensure mutable: %w", err)
	}

	return existing, nil
}

// filterVisibleMembers returns the subset of `members` the actor can see
// per the derived User:Read rule. Delegated to authz.Scope so the same
// rule applies in every call site (group.Get, group.UpdateMembers,
// group.Create, user.Get/List).
func (s *Service) filterVisibleMembers(
	ctx context.Context,
	actor domain.AuthInfo,
	members []string,
) []string {
	return s.scope.FilterVisibleUsers(ctx, actor.Email, members)
}
