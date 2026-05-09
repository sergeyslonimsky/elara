package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type DeleteInput struct {
	Namespace string
	Path      string
}

func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, auth.ObjectConfig, auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	revision, err := s.storage.Delete(ctx, in.Path, in.Namespace)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	s.watcher.NotifyDeleted(ctx, in.Path, in.Namespace, revision)

	return nil
}
