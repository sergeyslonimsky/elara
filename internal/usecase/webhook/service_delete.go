package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Delete(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	w, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get webhook: %w", err)
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := s.enforcer.Enforce(claims.Email, ns, "webhook", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	s.dispatcher.ClearHistory(id)

	return nil
}
