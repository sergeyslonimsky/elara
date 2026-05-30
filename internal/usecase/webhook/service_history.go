package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// GetHistory returns delivery attempts for a webhook if the caller can read it.
func (s *Service) GetHistory(
	ctx context.Context,
	webhookID string,
) ([]domain.DeliveryAttempt, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	w, err := s.repo.Get(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	if !s.pdp.Has(claims.Email, domain.Permission{
		Object: domain.ObjectWebhook,
		Action: domain.ActionRead,
		Domain: webhookDomain(w),
	}) {
		return nil, domain.ErrForbidden
	}

	return s.dispatcher.GetDeliveryHistory(webhookID), nil
}
