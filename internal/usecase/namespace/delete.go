package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_delete.go -package=mock_namespace . deleteEnforcer,nsDeleter,nsConfigCounter

type deleteEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type nsDeleter interface {
	Delete(ctx context.Context, name string) error
}

type nsConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type DeleteUseCase struct {
	enforcer   deleteEnforcer
	namespaces nsDeleter
	counter    nsConfigCounter
}

func NewDeleteUseCase(enforcer deleteEnforcer, namespaces nsDeleter, counter nsConfigCounter) *DeleteUseCase {
	return &DeleteUseCase{enforcer: enforcer, namespaces: namespaces, counter: counter}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, name string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	// domain = namespace name itself.
	allowed, err := uc.enforcer.Enforce(claims.Email, name, "namespace", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	count, err := uc.counter.CountConfigs(ctx, name)
	if err != nil {
		return fmt.Errorf("count configs in namespace: %w", err)
	}

	if count > 0 {
		return domain.NewValidationError("name", fmt.Sprintf("namespace %q contains %d config(s)", name, count))
	}

	if err := uc.namespaces.Delete(ctx, name); err != nil {
		return fmt.Errorf("delete namespace: %w", err)
	}

	return nil
}
