package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type GetAtRevisionInput struct {
	Namespace string
	Path      string
	Revision  int64
}

func (s *Service) GetAtRevision(ctx context.Context, in GetAtRevisionInput) (*domain.HistoryEntry, error) {
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

	entry, err := s.storage.GetAtRevision(ctx, in.Path, in.Namespace, in.Revision)
	if err != nil {
		return nil, fmt.Errorf("get config at revision: %w", err)
	}

	return entry, nil
}
