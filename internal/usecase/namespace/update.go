package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type updateEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type nsUpdater interface {
	Update(ctx context.Context, ns *domain.Namespace) error
}

type nsGetterForUpdate interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type updateConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type UpdateUseCase struct {
	enforcer   updateEnforcer
	namespaces nsUpdater
	getter     nsGetterForUpdate
	counter    updateConfigCounter
}

func NewUpdateUseCase(
	enforcer updateEnforcer,
	namespaces nsUpdater,
	getter nsGetterForUpdate,
	counter updateConfigCounter,
) *UpdateUseCase {
	return &UpdateUseCase{
		enforcer:   enforcer,
		namespaces: namespaces,
		getter:     getter,
		counter:    counter,
	}
}

func (uc *UpdateUseCase) Execute(ctx context.Context, name, description string) (*domain.Namespace, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// domain = namespace name itself.
	allowed, err := uc.enforcer.Enforce(claims.Email, name, "namespace", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	ns := &domain.Namespace{
		Name:        name,
		Description: description,
	}

	if err := uc.namespaces.Update(ctx, ns); err != nil {
		return nil, fmt.Errorf("update namespace: %w", err)
	}

	updated, err := uc.getter.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get updated namespace: %w", err)
	}

	count, err := uc.counter.CountConfigs(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("count configs: %w", err)
	}

	updated.ConfigCount = count

	return updated, nil
}
