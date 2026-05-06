package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_update.go -package=webhook_mock . webhookUpdater

type updateEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type webhookUpdater interface {
	Get(ctx context.Context, id string) (*domain.Webhook, error)
	Update(ctx context.Context, w *domain.Webhook) error
}

type UpdateParams struct {
	URL             string
	NamespaceFilter string
	PathPrefix      string
	Events          []domain.WebhookEventType
	Secret          string
	Enabled         bool
}

type UpdateUseCase struct {
	enforcer updateEnforcer
	repo     webhookUpdater
}

func NewUpdateUseCase(enforcer updateEnforcer, repo webhookUpdater) *UpdateUseCase {
	return &UpdateUseCase{enforcer: enforcer, repo: repo}
}

func (uc *UpdateUseCase) Execute(ctx context.Context, id string, params UpdateParams) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	existing, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	ns := existing.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, ns, "webhook", "write")
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

	if err := uc.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update webhook: %w", err)
	}

	return existing, nil
}
