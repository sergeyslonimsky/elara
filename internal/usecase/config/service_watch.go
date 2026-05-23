package config

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type WatchInput struct {
	Namespace  string
	PathPrefix string
}

func (s *Service) Watch(ctx context.Context, in WatchInput) (<-chan domain.WatchEvent, func(), error) {
	pathPrefix := in.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}

	ch, cancel := s.watcher.Subscribe(ctx, pathPrefix, in.Namespace)

	return ch, cancel, nil
}
