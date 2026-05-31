package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// List returns webhooks the caller can read, scoped by (Webhook, Read).
//
// Filtering is silent: webhooks in domains the caller cannot read are dropped
// from the result rather than returning an error. Global webhooks (empty
// NamespaceFilter) require a (Webhook, Read, *) right to be visible.
func (s *Service) List(ctx context.Context) ([]*domain.Webhook, error) {
	claims, ok := authctx.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	webhooks, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	scope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectWebhook, domain.ActionRead)
	if scope.IsEmpty() {
		return []*domain.Webhook{}, nil
	}

	filtered := make([]*domain.Webhook, 0, len(webhooks))
	for _, w := range webhooks {
		if scope.Contains(webhookDomain(w)) {
			filtered = append(filtered, w)
		}
	}

	return filtered, nil
}
