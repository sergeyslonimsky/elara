package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_get.go -package=webhook_mock . webhookGetter

type webhookGetter interface {
	Get(ctx context.Context, id string) (*domain.Webhook, error)
}

type GetUseCase struct {
	repo webhookGetter
}

func NewGetUseCase(repo webhookGetter) *GetUseCase {
	return &GetUseCase{repo: repo}
}

func (uc *GetUseCase) Execute(ctx context.Context, id string) (*domain.Webhook, error) {
	w, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	return w, nil
}
