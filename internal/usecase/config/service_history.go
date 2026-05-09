package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const defaultHistoryLimit = 20

type HistoryInput struct {
	Namespace string
	Path      string
	Limit     int
}

func (s *Service) History(ctx context.Context, in HistoryInput) ([]*domain.HistoryEntry, error) {
	if in.Namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, in.Namespace, "config", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	entries, err := s.storage.GetConfigHistory(ctx, in.Path, in.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("get config history: %w", err)
	}

	return entries, nil
}
