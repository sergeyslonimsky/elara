package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Get(ctx context.Context, id string) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// Fetch first to know the namespace filter, then enforce.
	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := s.enforcer.Enforce(claims.Email, ns, "webhook", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	return w, nil
}
