package user

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=user_mock -source=service.go

type (
	// UserReader is the read/write surface the usecase needs from user
	// persistence.
	UserReader interface {
		Get(ctx context.Context, email string) (*domain.User, error)
		List(
			ctx context.Context,
			filter domain.UserFilter,
			params domain.UserListParams,
		) ([]*domain.User, int, error)
		Upsert(ctx context.Context, user *domain.User) error
		Delete(ctx context.Context, email string) error
		SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
		SetMembershipVersion(ctx context.Context, email string, version int64) error
	}

	// GroupReader is the narrow read-only port the usecase needs over the group
	// repo.
	GroupReader interface {
		Get(ctx context.Context, id string) (*domain.Group, error)
		FindByName(ctx context.Context, name string) (*domain.Group, error)
	}

	sessionsService interface {
		RevokeAllForUser(ctx context.Context, userID, revokedBy, reason string) error
	}
)

type Service struct {
	txm      storage.Manager
	store    UserReader
	groups   GroupReader
	sessions sessionsService
	pdp      *authz.PDP
	pap      *authz.PAP
	scope    *authz.Scope
}

func New(
	txm storage.Manager,
	store UserReader,
	groups GroupReader,
	sessions sessionsService,
	pdp *authz.PDP,
	pap *authz.PAP,
	scope *authz.Scope,
) *Service {
	return &Service{
		txm:      txm,
		store:    store,
		groups:   groups,
		sessions: sessions,
		pdp:      pdp,
		pap:      pap,
		scope:    scope,
	}
}
