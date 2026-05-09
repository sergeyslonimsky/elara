package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Create(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := s.enforcer.Enforce(claims.Email, ns, "webhook", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if err := s.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return w, nil
}
