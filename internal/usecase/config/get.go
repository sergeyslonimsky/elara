package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type configGetter interface {
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type GetUseCase struct {
	configs configGetter
}

func NewGetUseCase(configs configGetter) *GetUseCase {
	return &GetUseCase{configs: configs}
}

func (uc *GetUseCase) Execute(ctx context.Context, path, namespace string) (*domain.Config, error) {
	if namespace == "" {
		namespace = domain.DefaultNamespace
	}

	cfg, err := uc.configs.Get(ctx, path, namespace)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}
