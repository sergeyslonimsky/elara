package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GetHistory returns delivery attempts for a webhook if the caller can read it.
func (s *Service) GetHistory(
	ctx context.Context,
	webhookID string,
) ([]domain.DeliveryAttempt, error) {
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	w, err := s.repo.Get(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	if !s.pdp.Has(info.UserID, domain.Permission{
		Object: domain.ObjectWebhook,
		Action: domain.ActionRead,
		Domain: webhookDomain(w),
	}) {
		return nil, domain.ErrForbidden
	}

	return s.dispatcher.GetDeliveryHistory(webhookID), nil
}
