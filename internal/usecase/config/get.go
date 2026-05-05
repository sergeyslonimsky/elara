package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type getEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type configGetter interface {
	Get(ctx context.Context, path, namespace string) (*domain.Config, error)
}

type GetUseCase struct {
	enforcer getEnforcer
	configs  configGetter
}

func NewGetUseCase(enforcer getEnforcer, configs configGetter) *GetUseCase {
	return &GetUseCase{enforcer: enforcer, configs: configs}
}

func (uc *GetUseCase) Execute(ctx context.Context, path, namespace string) (*domain.Config, error) {
	if namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	cfg, err := uc.configs.Get(ctx, path, namespace)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return cfg, nil
}
