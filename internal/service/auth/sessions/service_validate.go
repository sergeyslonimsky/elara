package sessions

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Validate retrieves the session by id and checks that it is neither revoked
// nor expired. It does NOT update LastSeenAt — that is Refresh's job.
// Sentinel errors (domain.ErrSessionNotFound, ErrSessionRevoked,
// ErrSessionExpired) are wrapped transparently so callers can use errors.Is.
func (s *Service) Validate(ctx context.Context, id string) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("sessions: validate: %w", err)
	}

	if err = sess.EnsureActive(s.clock.Now()); err != nil {
		return nil, fmt.Errorf("sessions: validate: %w", err)
	}

	return sess, nil
}
