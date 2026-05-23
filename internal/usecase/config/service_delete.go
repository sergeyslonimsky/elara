package config

import (
	"context"
	"fmt"
)

type DeleteInput struct {
	Namespace string
	Path      string
}

func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	revision, err := s.storage.Delete(ctx, in.Path, in.Namespace)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	s.watcher.NotifyDeleted(ctx, in.Path, in.Namespace, revision)

	return nil
}
