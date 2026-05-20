package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// GetStats collects KPI numbers scoped to namespaces the caller can read.
// Authenticated users always get a response; per-namespace counts are filtered
// by config:read so users see their own scope rather than a forbidden error.
func (s *Service) GetStats(ctx context.Context) (*StatsResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	namespaces, err := s.namespaces.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var (
		accessibleNamespaces int
		totalConfigs         int
	)

	for _, ns := range namespaces {
		allowed, _ := s.enforcer.Enforce(claims.Email, ns.Name, domain.ObjectConfig, domain.ActionRead)
		if !allowed {
			continue
		}

		accessibleNamespaces++

		count, err := s.configs.CountByNamespace(ctx, ns.Name)
		if err != nil {
			return nil, fmt.Errorf("count configs for namespace %q: %w", ns.Name, err)
		}

		totalConfigs += count
	}

	revision, err := s.configs.CurrentRevision(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current revision: %w", err)
	}

	return &StatsResult{
		NamespaceCount:    accessibleNamespaces,
		ConfigCount:       totalConfigs,
		ActiveClientCount: len(s.clients.ListActive()),
		GlobalRevision:    revision,
	}, nil
}
