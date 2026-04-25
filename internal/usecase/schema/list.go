package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type schemaLister interface {
	List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
}

type ListUseCase struct {
	store schemaLister
}

func NewListUseCase(store schemaLister) *ListUseCase {
	return &ListUseCase{store: store}
}

func (uc *ListUseCase) Execute(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	schemas, err := uc.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return schemas, nil
}
