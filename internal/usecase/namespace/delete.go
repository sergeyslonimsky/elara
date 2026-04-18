package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type nsDeleter interface {
	Delete(ctx context.Context, name string) error
}

type nsConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type DeleteUseCase struct {
	namespaces nsDeleter
	counter    nsConfigCounter
}

func NewDeleteUseCase(namespaces nsDeleter, counter nsConfigCounter) *DeleteUseCase {
	return &DeleteUseCase{namespaces: namespaces, counter: counter}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, name string) error {
	ns := &domain.Namespace{Name: name}
	if ns.IsDefault() {
		return domain.NewValidationError("name", "cannot delete default namespace")
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
