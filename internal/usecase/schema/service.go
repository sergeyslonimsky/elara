package schema

import (
	"context"
	"strings"

	"github.com/gobwas/glob"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=schema_mock -source=service.go

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
	}

	store interface {
		Attach(ctx context.Context, s *domain.SchemaAttachment) error
		Detach(ctx context.Context, namespace, pathPattern string) error
		Get(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error)
		List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
	}

	nsProvider interface {
		Get(ctx context.Context, name string) (*domain.Namespace, error)
	}
)

type Service struct {
	pdp        pdp
	store      store
	namespaces nsProvider
}

func New(pdp pdp, store store, namespaces nsProvider) *Service {
	return &Service{
		pdp:        pdp,
		store:      store,
		namespaces: namespaces,
	}
}

// findBestMatch returns the most specific matching schema (fewest wildcard chars).
// On equal specificity, the oldest CreatedAt wins.
func findBestMatch(schemas []*domain.SchemaAttachment, configPath string) *domain.SchemaAttachment {
	var best *domain.SchemaAttachment
	bestScore := -1

	for _, s := range schemas {
		g, err := glob.Compile(s.PathPattern, '/')
		if err != nil {
			continue
		}

		if !g.Match(configPath) {
			continue
		}

		score := specificity(s.PathPattern)
		if best == nil || score > bestScore || (score == bestScore && s.CreatedAt.Before(best.CreatedAt)) {
			best = s
			bestScore = score
		}
	}

	return best
}

// specificity returns a score inversely proportional to wildcard count.
// Higher score = more specific = better match.
func specificity(pattern string) int {
	wildcards := strings.Count(pattern, "*") + strings.Count(pattern, "?") + strings.Count(pattern, "[")

	return -wildcards
}
