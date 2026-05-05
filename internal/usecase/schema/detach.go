package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type detachEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type schemaDetacher interface {
	Detach(ctx context.Context, namespace, pathPattern string) error
}

type detachNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type DetachUseCase struct {
	enforcer   detachEnforcer
	store      schemaDetacher
	namespaces detachNSChecker
}

func NewDetachUseCase(enforcer detachEnforcer, store schemaDetacher, namespaces detachNSChecker) *DetachUseCase {
	return &DetachUseCase{enforcer: enforcer, store: store, namespaces: namespaces}
}

func (uc *DetachUseCase) Execute(ctx context.Context, namespace, pathPattern string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "schema", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	ns, err := uc.namespaces.Get(ctx, namespace)
	if err != nil {
		return fmt.Errorf("get namespace: %w", err)
	}

	if ns.Locked {
		return fmt.Errorf("namespace %q: %w", namespace, domain.ErrNamespaceLocked)
	}

	if err := uc.store.Detach(ctx, namespace, pathPattern); err != nil {
		return fmt.Errorf("detach schema: %w", err)
	}

	return nil
}
