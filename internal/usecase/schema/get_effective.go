package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type getEffectiveEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type GetEffectiveUseCase struct {
	enforcer getEffectiveEnforcer
	repo     schemaContentLister
}

func NewGetEffectiveUseCase(enforcer getEffectiveEnforcer, repo schemaContentLister) *GetEffectiveUseCase {
	return &GetEffectiveUseCase{enforcer: enforcer, repo: repo}
}

func (uc *GetEffectiveUseCase) Execute(
	ctx context.Context,
	namespace, path string,
) (*domain.SchemaAttachment, error) {
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

	schemas, err := uc.repo.List(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	return findBestMatch(schemas, path), nil
}
