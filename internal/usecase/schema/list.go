package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_list.go -package=schema_mock . schemaListEnforcer,schemaLister

type schemaListEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type schemaLister interface {
	List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
}

type ListUseCase struct {
	enforcer schemaListEnforcer
	store    schemaLister
}

func NewListUseCase(enforcer schemaListEnforcer, store schemaLister) *ListUseCase {
	return &ListUseCase{enforcer: enforcer, store: store}
}

func (uc *ListUseCase) Execute(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// Filter silently by namespace.
	allowed, _ := uc.enforcer.Enforce(claims.Email, namespace, "schema", "read")
	if !allowed {
		return nil, nil
	}

	schemas, err := uc.store.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return schemas, nil
}
