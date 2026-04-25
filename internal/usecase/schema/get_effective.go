package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type GetEffectiveUseCase struct {
	repo schemaContentLister
}

func NewGetEffectiveUseCase(repo schemaContentLister) *GetEffectiveUseCase {
	return &GetEffectiveUseCase{repo: repo}
}

func (uc *GetEffectiveUseCase) Execute(
	ctx context.Context,
	namespace, path string,
) (*domain.SchemaAttachment, error) {
	schemas, err := uc.repo.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return findBestMatch(schemas, path), nil
}
