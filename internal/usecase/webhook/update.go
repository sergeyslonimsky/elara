package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_update.go -package=webhook_mock . webhookUpdater

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
	repo webhookUpdater
}

func NewUpdateUseCase(repo webhookUpdater) *UpdateUseCase {
	return &UpdateUseCase{repo: repo}
}

func (uc *UpdateUseCase) Execute(ctx context.Context, id string, params UpdateParams) (*domain.Webhook, error) {
	existing, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
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
