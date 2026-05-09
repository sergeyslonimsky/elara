package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) List(ctx context.Context) ([]*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	webhooks, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	// Filter silently: only return webhooks the caller can read.
	filtered := webhooks[:0]
	for _, w := range webhooks {
		ns := w.NamespaceFilter
		if ns == "" {
			ns = "*"
		}

		allowed, _ := s.enforcer.Enforce(claims.Email, ns, "webhook", "read")
		if allowed {
			filtered = append(filtered, w)
		}
	}

	return filtered, nil
}
