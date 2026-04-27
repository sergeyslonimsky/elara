package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type webhookLister interface {
	List(ctx context.Context) ([]*domain.Webhook, error)
}

type ListUseCase struct {
	repo webhookLister
}

func NewListUseCase(repo webhookLister) *ListUseCase {
	return &ListUseCase{repo: repo}
}

func (uc *ListUseCase) Execute(ctx context.Context) ([]*domain.Webhook, error) {
	webhooks, err := uc.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	return webhooks, nil
}
