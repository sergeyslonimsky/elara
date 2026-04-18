package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type nsCreator interface {
	Create(ctx context.Context, ns *domain.Namespace) error
}

type nsGetterForCreate interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type CreateUseCase struct {
	namespaces nsCreator
	getter     nsGetterForCreate
}

func NewCreateUseCase(namespaces nsCreator, getter nsGetterForCreate) *CreateUseCase {
	return &CreateUseCase{namespaces: namespaces, getter: getter}
}

func (uc *CreateUseCase) Execute(ctx context.Context, ns *domain.Namespace) (*domain.Namespace, error) {
	if err := ns.Validate(); err != nil {
		return nil, fmt.Errorf("validate namespace: %w", err)
	}

	if err := uc.namespaces.Create(ctx, ns); err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}

	created, err := uc.getter.Get(ctx, ns.Name)
	if err != nil {
		return nil, fmt.Errorf("get created namespace: %w", err)
	}

	return created, nil
}
