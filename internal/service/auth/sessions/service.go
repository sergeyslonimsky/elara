package sessions

//go:generate mockgen -destination=mocks/service_mock.go -package=sessions_mock -source=service.go

import (
	"context"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type (
	// sessionRepository is the read/write surface for individual sessions.
	sessionRepository interface {
		Get(ctx context.Context, id string) (*domain.Session, error)
		Create(ctx context.Context, s *domain.Session) error
		Update(ctx context.Context, s *domain.Session) error
		ListByUser(ctx context.Context, userID string) ([]*domain.Session, error)
		ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error)
		RevokeAllForUser(ctx context.Context, userID, revokedBy string) (int, error)
	}

	// sessionEventRepository is the append-only surface for session audit events.
	sessionEventRepository interface {
		Append(ctx context.Context, e *domain.SessionEvent) error
		ListBySession(ctx context.Context, sessionID string) ([]*domain.SessionEvent, error)
		ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.SessionEvent, error)
	}

	// Clock abstracts time.Now() for test injection.
	// A real implementation wraps time.Now(); tests inject a fixed value.
	Clock interface {
		Now() time.Time
	}
)

// Service implements session lifecycle operations: create, validate, refresh,
// revoke, and list.
//
// Following the EL-51 "Usecase-owned Transactions" design, the Service does
// NOT manage transaction boundaries itself. It relies on the provided context
// to carry any active transaction handle (flattening). Repositories called by
// the Service will either join the transaction in the context or perform
// an "auto-open" for single-repo operations.
type Service struct {
	repo   sessionRepository
	events sessionEventRepository
	clock  Clock
}

// New constructs a Service with the given dependencies.
func New(
	repo sessionRepository,
	events sessionEventRepository,
	clock Clock,
) *Service {
	return &Service{
		repo:   repo,
		events: events,
		clock:  clock,
	}
}
