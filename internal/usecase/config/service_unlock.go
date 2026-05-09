package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type UnlockInput struct {
	Namespace string
	Path      string
}

func (s *Service) Unlock(ctx context.Context, in UnlockInput) error {
	if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, auth.ObjectConfig, auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	if err := s.storage.UnlockConfig(ctx, in.Namespace, in.Path); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}

	cfg, err := s.storage.Get(ctx, in.Path, in.Namespace)
	if err != nil {
		return fmt.Errorf("get config after unlock: %w", err)
	}

	s.watcher.NotifyConfigUnlocked(ctx, cfg)

	return nil
}
