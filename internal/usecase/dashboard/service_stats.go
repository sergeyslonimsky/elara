package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// GetStats collects KPI numbers for the dashboard header.
func (s *Service) GetStats(ctx context.Context) (*StatsResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, "*", "dashboard", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	namespaces, err := s.namespaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var totalConfigs int
	for _, ns := range namespaces {
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
		NamespaceCount:    len(namespaces),
		ConfigCount:       totalConfigs,
		ActiveClientCount: len(s.clients.ListActive()),
		GlobalRevision:    revision,
	}, nil
}
