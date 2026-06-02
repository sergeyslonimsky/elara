package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=user_mock -source=service.go

type (
	// UserReader is the narrow read surface the usecase needs from user
	// persistence. Writes go through UserManager so identity-uniqueness and
	// append-only invariants are enforced uniformly.
	UserReader interface {
		GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
		GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error)
		List(
			ctx context.Context,
			filter domain.UserFilter,
			params domain.UserListParams,
		) ([]*domain.User, int, error)
		Delete(ctx context.Context, id uuid.UUID) error
		SetPassword(ctx context.Context, userID uuid.UUID, hash string, changeRequired bool) error
		SetMembershipVersion(ctx context.Context, userID uuid.UUID, version int64) error
	}

	// UserManager is the service-layer entry point for validated user
	// mutations (create + status transitions). Field-level updates outside
	// of Status (LinkIdentity, RecordLogin) live on *auth.UserService and
	// are consumed by the auth usecase directly — usecase/user only deals
	// with the API-side surface (Create + Deactivate/Reactivate + Delete
	// via UserReader). Backed by *auth.UserService at wire-time.
	UserManager interface {
		Create(ctx context.Context, user *domain.User) error
		Deactivate(ctx context.Context, userID uuid.UUID) (*domain.User, error)
		Reactivate(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	}

	sessionsService interface {
		RevokeAllForUser(ctx context.Context, userID, revokedBy, reason string) error
	}
)

type Service struct {
	txm      storage.Manager
	store    UserReader
	users    UserManager
	groups   domain.GroupReader
	sessions sessionsService
	pdp      *authz.PDP
	pap      *authz.PAP
	scope    *authz.Scope
}

func New(
	txm storage.Manager,
	store UserReader,
	users UserManager,
	groups domain.GroupReader,
	sessions sessionsService,
	pdp *authz.PDP,
	pap *authz.PAP,
	scope *authz.Scope,
) *Service {
	return &Service{
		txm:      txm,
		store:    store,
		users:    users,
		groups:   groups,
		sessions: sessions,
		pdp:      pdp,
		pap:      pap,
		scope:    scope,
	}
}

// transitionStatus is the shared lookup + self-guard + authorize + apply
// dance used by Deactivate and Reactivate. The two diverge only in the
// preCheck (last-admin for deactivate), the state-machine call (via
// UserManager), and the side-effect (session revoke for deactivate).
// Extracting the common path keeps both endpoints in sync — adding e.g.
// an audit-event step lands in one place, not two.
func (s *Service) transitionStatus(
	ctx context.Context,
	actor domain.AuthInfo,
	userID uuid.UUID,
	selfErrMsg string,
	preCheck func(targetID string) error,
	apply func(ctx context.Context, userID uuid.UUID) (*domain.User, error),
	sideEffect func(ctx context.Context, user *domain.User) error,
) (*domain.User, error) {
	var updated *domain.User

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		user, err := s.store.GetByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if actor.UserID == user.ID.String() {
			return domain.NewValidationError("user_id", selfErrMsg)
		}

		if err := s.authorizeUserWrite(ctx, actor, user.ID.String()); err != nil {
			return err
		}

		if preCheck != nil {
			if err := preCheck(user.ID.String()); err != nil {
				return err
			}
		}

		updated, err = apply(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("apply status transition: %w", err)
		}

		if sideEffect != nil {
			if err := sideEffect(ctx, updated); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("with tx: %w", err)
	}

	return updated, nil
}
