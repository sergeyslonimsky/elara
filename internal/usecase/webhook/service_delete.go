package webhook

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Delete removes a webhook if the caller holds (Webhook, Write) on its
// namespace.
func (s *Service) Delete(ctx context.Context, id string) error {
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return domain.ErrUnauthorized
	}

	webhook, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return fmt.Errorf("get webhook: %w", domain.ErrNotFound)
		}

		return fmt.Errorf("get webhook: %w", err)
	}

	if !s.pdp.Has(info.UserID, domain.Permission{
		Object: domain.ObjectWebhook,
		Action: domain.ActionWrite,
		Domain: webhookDomain(webhook),
	}) {
		return domain.ErrForbidden
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return fmt.Errorf("delete webhook: %w", domain.ErrNotFound)
		}

		return fmt.Errorf("delete webhook: %w", err)
	}

	s.dispatcher.ClearHistory(id)

	return nil
}
