package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type webhookCreator interface {
	Create(ctx context.Context, w *domain.Webhook) error
}

type CreateUseCase struct {
	repo webhookCreator
}

func NewCreateUseCase(repo webhookCreator) *CreateUseCase {
	return &CreateUseCase{repo: repo}
}

func (uc *CreateUseCase) Execute(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error) {
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if err := uc.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return w, nil
}
