package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// Delete removes a webhook if the caller holds (Webhook, Write) on its
// namespace.
func (s *Service) Delete(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	webhook, err := s.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get webhook: %w", err)
	}

	if !s.pdp.Has(claims.Email, domain.Permission{
		Object: domain.ObjectWebhook,
		Action: domain.ActionWrite,
		Domain: webhookDomain(webhook),
	}) {
		return domain.ErrForbidden
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	s.dispatcher.ClearHistory(id)

	return nil
}
