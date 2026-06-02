package profile

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=profile_mock -source=service.go

type (
	pdp interface {
		ListPermissions(principal string) ([]domain.Permission, error)
	}

	userGetter interface {
		GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error)
	}

	passWriter interface {
		SetPassword(ctx context.Context, userID uuid.UUID, hash string, changeRequired bool) error
	}

	sessionsService interface {
		Revoke(ctx context.Context, id, revokedBy, reason string, eventType domain.SessionEventType) error
		Create(ctx context.Context, params sessions.CreateParams) (*domain.Session, error)
	}
)

type Service struct {
	txm      storage.Manager
	pdp      pdp
	users    userGetter
	pass     passWriter
	sessions sessionsService
}

func New(
	txm storage.Manager,
	pdp pdp,
	users userGetter,
	pass passWriter,
	sessionsSvc sessionsService,
) *Service {
	return &Service{
		txm:      txm,
		pdp:      pdp,
		users:    users,
		pass:     pass,
		sessions: sessionsSvc,
	}
}

// Logout revokes the provided session ID atomically.
// Cookie clearing is performed by the handler.
func (s *Service) Logout(ctx context.Context, sessionID, revokedBy string) error {
	if err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		return s.sessions.Revoke(
			ctx,
			sessionID,
			revokedBy,
			"user logout",
			domain.SessionEventRevokedByUser,
		)
	}); err != nil {
		return fmt.Errorf("logout tx: %w", err)
	}

	return nil
}
