package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type LockInput struct {
	Namespace string
	Path      string
}

func (s *Service) Lock(ctx context.Context, in LockInput) error {
	if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, auth.ObjectConfig, auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	if err := s.storage.LockConfig(ctx, in.Namespace, in.Path); err != nil {
		return fmt.Errorf("lock: %w", err)
	}

	cfg, err := s.storage.Get(ctx, in.Path, in.Namespace)
	if err != nil {
		return fmt.Errorf("get config after lock: %w", err)
	}

	s.watcher.NotifyConfigLocked(ctx, cfg)

	return nil
}
