package dashboard

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// activityOverfetchMultiplier — backend-side overfetch so we can still hit `limit`
// after dropping entries from namespaces the caller cannot read.
const activityOverfetchMultiplier = 5

// ListActivity returns the most recent changelog entries scoped to namespaces
// the caller can read. Result may be shorter than `limit` if recent activity
// happened mostly in namespaces the caller has no access to.
func (s *Service) ListActivity(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	if limit <= 0 {
		limit = 1
	}

	entries, err := s.activity.ListRecentChanges(ctx, limit*activityOverfetchMultiplier)
	if err != nil {
		return nil, fmt.Errorf("list recent changes: %w", err)
	}

	allowedNamespace := make(map[string]bool)

	out := make([]*domain.ChangelogEntry, 0, len(entries))
	for _, e := range entries {
		allowed, ok := allowedNamespace[e.Namespace]
		if !ok {
			allowed = s.pdp.Has(claims.Email, domain.Permission{
				Object: domain.ObjectNamespace,
				Action: domain.ActionRead,
				Domain: e.Namespace,
			})
			allowedNamespace[e.Namespace] = allowed
		}

		if !allowed {
			continue
		}

		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}

	return out, nil
}
