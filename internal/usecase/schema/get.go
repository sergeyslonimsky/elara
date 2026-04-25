package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type schemaGetter interface {
	Get(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error)
}

type GetUseCase struct {
	store schemaGetter
}

func NewGetUseCase(store schemaGetter) *GetUseCase {
	return &GetUseCase{store: store}
}

func (uc *GetUseCase) Execute(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error) {
	s, err := uc.store.Get(ctx, namespace, pathPattern)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return s, nil
}
