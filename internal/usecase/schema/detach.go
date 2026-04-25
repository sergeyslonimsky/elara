package schema

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type schemaDetacher interface {
	Detach(ctx context.Context, namespace, pathPattern string) error
}

type detachNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type DetachUseCase struct {
	store      schemaDetacher
	namespaces detachNSChecker
}

func NewDetachUseCase(store schemaDetacher, namespaces detachNSChecker) *DetachUseCase {
	return &DetachUseCase{store: store, namespaces: namespaces}
}

func (uc *DetachUseCase) Execute(ctx context.Context, namespace, pathPattern string) error {
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
