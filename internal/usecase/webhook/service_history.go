package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) GetHistory(ctx context.Context, webhookID string) ([]domain.DeliveryAttempt, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	w, err := s.repo.Get(ctx, webhookID)
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

	return s.dispatcher.GetDeliveryHistory(webhookID), nil
}
