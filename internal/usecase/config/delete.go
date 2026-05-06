package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_delete.go -package=config_mock . deleteEnforcer,configDeleter,deleteWatchNotifier

type deleteEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type configDeleter interface {
	Delete(ctx context.Context, path, namespace string) (int64, error)
}

type deleteWatchNotifier interface {
	NotifyDeleted(ctx context.Context, path, namespace string, revision int64)
}

type DeleteUseCase struct {
	enforcer deleteEnforcer
	configs  configDeleter
	watch    deleteWatchNotifier
}

func NewDeleteUseCase(enforcer deleteEnforcer, configs configDeleter, watch deleteWatchNotifier) *DeleteUseCase {
	return &DeleteUseCase{enforcer: enforcer, configs: configs, watch: watch}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, path, namespace string) error {
	if namespace == "" {
		return domain.NewValidationError("namespace", "namespace is required")
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	rev, err := uc.configs.Delete(ctx, path, namespace)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	uc.watch.NotifyDeleted(ctx, path, namespace, rev)

	return nil
}
