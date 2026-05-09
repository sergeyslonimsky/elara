package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type WatchInput struct {
	Namespace  string
	PathPrefix string
}

func (s *Service) Watch(ctx context.Context, in WatchInput) (<-chan domain.WatchEvent, func(), error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, in.Namespace, "config", "read")
	if err != nil {
		return nil, nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, nil, domain.ErrForbidden
	}

	pathPrefix := in.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}

	ch, cancel := s.watcher.Subscribe(ctx, pathPrefix, in.Namespace)

	return ch, cancel, nil
}
