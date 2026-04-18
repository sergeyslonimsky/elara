package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type nsGetter interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type getConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type GetUseCase struct {
	namespaces nsGetter
	counter    getConfigCounter
}

func NewGetUseCase(namespaces nsGetter, counter getConfigCounter) *GetUseCase {
	return &GetUseCase{namespaces: namespaces, counter: counter}
}

func (uc *GetUseCase) Execute(ctx context.Context, name string) (*domain.Namespace, error) {
	ns, err := uc.namespaces.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	count, err := uc.counter.CountConfigs(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("count configs: %w", err)
	}

	ns.ConfigCount = count

	return ns, nil
}
