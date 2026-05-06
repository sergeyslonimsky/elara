package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_get.go -package=mock_namespace . getEnforcer,nsGetter,getConfigCounter

type getEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type nsGetter interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type getConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type GetUseCase struct {
	enforcer   getEnforcer
	namespaces nsGetter
	counter    getConfigCounter
}

func NewGetUseCase(enforcer getEnforcer, namespaces nsGetter, counter getConfigCounter) *GetUseCase {
	return &GetUseCase{enforcer: enforcer, namespaces: namespaces, counter: counter}
}

func (uc *GetUseCase) Execute(ctx context.Context, name string) (*domain.Namespace, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// domain = namespace name itself.
	allowed, err := uc.enforcer.Enforce(claims.Email, name, "namespace", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

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
