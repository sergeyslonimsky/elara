package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// ListActivity returns the most recent changelog entries.
func (s *Service) ListActivity(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, "*", "dashboard", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	entries, err := s.activity.ListRecentChanges(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}

	return entries, nil
}
