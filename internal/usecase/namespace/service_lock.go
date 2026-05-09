package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Lock(ctx context.Context, name string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, name, auth.ObjectNamespace, auth.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.store.LockNamespace(ctx, name); err != nil {
		return fmt.Errorf("lock namespace: %w", err)
	}

	s.notifier.NotifyNamespaceLocked(ctx, name)

	return nil
}

func (s *Service) Unlock(ctx context.Context, name string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, name, auth.ObjectNamespace, auth.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.store.UnlockNamespace(ctx, name); err != nil {
		return fmt.Errorf("unlock namespace: %w", err)
	}

	s.notifier.NotifyNamespaceUnlocked(ctx, name)

	return nil
}
