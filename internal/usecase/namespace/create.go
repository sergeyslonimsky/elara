package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type createEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type nsCreator interface {
	Create(ctx context.Context, ns *domain.Namespace) error
}

type nsGetterForCreate interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type CreateUseCase struct {
	enforcer   createEnforcer
	namespaces nsCreator
	getter     nsGetterForCreate
}

func NewCreateUseCase(enforcer createEnforcer, namespaces nsCreator, getter nsGetterForCreate) *CreateUseCase {
	return &CreateUseCase{enforcer: enforcer, namespaces: namespaces, getter: getter}
}

func (uc *CreateUseCase) Execute(ctx context.Context, ns *domain.Namespace) (*domain.Namespace, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, "*", "namespace", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

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
