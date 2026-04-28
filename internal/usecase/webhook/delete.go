package webhook

import (
	"context"
	"fmt"
)

//go:generate mockgen -destination=mocks/mock_delete.go -package=webhook_mock . webhookDeleter,historyClearer

type webhookDeleter interface {
	Delete(ctx context.Context, id string) error
}

type historyClearer interface {
	ClearHistory(webhookID string)
}

type DeleteUseCase struct {
	repo       webhookDeleter
	dispatcher historyClearer
}

func NewDeleteUseCase(repo webhookDeleter, dispatcher historyClearer) *DeleteUseCase {
	return &DeleteUseCase{repo: repo, dispatcher: dispatcher}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	uc.dispatcher.ClearHistory(id)

	return nil
}
