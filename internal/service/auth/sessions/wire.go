package sessions

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Repository is the exported view of sessionRepository used by DI to wire the
// Service from concrete repo implementations without exposing the production
// service to a private interface from outside the package.
type Repository interface {
	Get(ctx context.Context, id string) (*domain.Session, error)
	Create(ctx context.Context, s *domain.Session) error
	Update(ctx context.Context, s *domain.Session) error
	ListByUser(ctx context.Context, userID string) ([]*domain.Session, error)
	ListActiveByUser(ctx context.Context, userID string) ([]*domain.Session, error)
	RevokeAllForUser(ctx context.Context, userID, revokedBy string) (int, error)
}

// EventRepository is the exported view of sessionEventRepository for DI.
type EventRepository interface {
	Append(ctx context.Context, e *domain.SessionEvent) error
	ListBySession(ctx context.Context, sessionID string) ([]*domain.SessionEvent, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.SessionEvent, error)
}

// NewService is the DI-facing constructor. It accepts exported interfaces
// (Repository / EventRepository / Manager) so callers outside the package
// can construct a *Service without referencing the unexported test-facing
// interfaces used by mockgen.
func NewService(
	repo Repository,
	events EventRepository,
	clock Clock,
) *Service {
	return &Service{
		repo:   repo,
		events: events,
		clock:  clock,
	}
}
