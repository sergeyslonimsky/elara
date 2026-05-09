package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type UpdateParams struct {
	URL             string
	NamespaceFilter string
	PathPrefix      string
	Events          []domain.WebhookEventType
	Secret          string
	Enabled         bool
}

func (s *Service) Update(ctx context.Context, id string, params UpdateParams) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	ns := existing.NamespaceFilter
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

	existing.URL = params.URL
	existing.NamespaceFilter = params.NamespaceFilter
	existing.PathPrefix = params.PathPrefix
	existing.Enabled = params.Enabled
	existing.Events = params.Events

	if params.Secret != "" {
		existing.Secret = params.Secret
	}

	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update webhook: %w", err)
	}

	return existing, nil
}
