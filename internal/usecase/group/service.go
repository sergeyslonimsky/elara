package group

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// GroupReader is the narrow read-only port the usecase needs over the group
// repo.
type GroupReader interface {
	Get(ctx context.Context, name string) (*domain.Group, error)
	Create(ctx context.Context, group *domain.Group) error
	Update(ctx context.Context, group *domain.Group) error
	Delete(ctx context.Context, name string) error
	List(
		ctx context.Context,
		filter domain.GroupFilter,
		params domain.GroupListParams,
	) ([]*domain.Group, int, error)
}

// Service orchestrates group lifecycle and Casbin role/membership rules.
type Service struct {
	txm   storage.Manager
	store GroupReader
	pdp   *authz.PDP
	pap   *authz.PAP
	scope *authz.Scope
}

func New(
	txm storage.Manager,
	store GroupReader,
	pdp *authz.PDP,
	pap *authz.PAP,
	scope *authz.Scope,
) *Service {
	return &Service{
		txm:   txm,
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

// loadMutableGroup fetches a group within the given context (potentially carrying
// a transaction) and rejects system groups via EnsureMutable.
// Shared by Update*, Delete, and Get-then-mutate flows.
func (s *Service) loadMutableGroup(
	ctx context.Context,
	name string,
) (*domain.Group, error) {
	existing, err := s.store.Get(ctx, name)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, fmt.Errorf(errGetGroup, domain.NewNotFoundError("group", name))
		}

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
	return s.scope.FilterVisibleUsers(ctx, actor.UserID, members)
}
