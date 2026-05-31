package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Get returns the webhook if the caller holds (Webhook, Read) on the webhook's
// namespace filter (or "*" for global webhooks). Load-then-check: we must fetch
// the webhook before we know which namespace to gate on.
func (s *Service) Get(ctx context.Context, id string) (*domain.Webhook, error) {
	claims, ok := authctx.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	webhook, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	if !s.pdp.Has(claims.Email, domain.Permission{
		Object: domain.ObjectWebhook,
		Action: domain.ActionRead,
		Domain: webhookDomain(webhook),
	}) {
		return nil, domain.ErrForbidden
	}

	return webhook, nil
}

// webhookDomain returns the Casbin domain for a webhook. An empty
// NamespaceFilter means the webhook fires for every namespace; we map that to
// the global domain so the check is "can the caller read/write *any* namespace".
func webhookDomain(w *domain.Webhook) string {
	if w.NamespaceFilter == "" {
		return domain.DomainAll
	}

	return w.NamespaceFilter
}
