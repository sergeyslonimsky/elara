package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GetStats collects KPI numbers scoped to namespaces the caller can read.
// Authenticated users always get a response; per-namespace counts are filtered
// by config:read so users see their own scope rather than a forbidden error.
func (s *Service) GetStats(ctx context.Context) (*StatsResult, error) {
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
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
		if !s.pdp.HasNamespace(info.UserID, ns.Name, domain.ActionRead) {
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
