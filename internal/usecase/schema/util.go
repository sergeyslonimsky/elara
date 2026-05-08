package schema

import (
	"context"
	"strings"

	"github.com/gobwas/glob"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type schemaContentLister interface {
	List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
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
