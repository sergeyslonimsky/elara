package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type schemaGetEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type schemaGetter interface {
	Get(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error)
}

type GetUseCase struct {
	enforcer schemaGetEnforcer
	store    schemaGetter
}

func NewGetUseCase(enforcer schemaGetEnforcer, store schemaGetter) *GetUseCase {
	return &GetUseCase{enforcer: enforcer, store: store}
}

func (uc *GetUseCase) Execute(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "schema", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	s, err := uc.store.Get(ctx, namespace, pathPattern)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	return s, nil
}
