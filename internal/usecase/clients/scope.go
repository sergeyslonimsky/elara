package clients

import (
	"strings"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// scopeChecker decides which clients a caller is allowed to see.
//
// Admins (anyone with the global client:read permission) bypass filtering.
// Everyone else only sees clients with at least one ActiveWatch in a namespace
// they can read. Clients with no live watches (e.g. historical entries) are
// invisible to non-admins because we have no per-namespace info to scope by.
type scopeChecker struct {
	pdp   pdp
	email string
	admin bool
	cache map[string]bool // namespace → allowed
}

func newScopeChecker(p pdp, email string) *scopeChecker {
	admin := p.Has(email, domain.Permission{
		Object: domain.ObjectClient,
		Action: domain.ActionRead,
		Domain: domain.DomainAll,
	})

	return &scopeChecker{
		pdp:   p,
		email: email,
		admin: admin,
		cache: map[string]bool{},
	}
}

func (s *scopeChecker) visible(c *domain.Client) bool {
	if s.admin {
		return true
	}

	for _, w := range c.ActiveWatchList {
		ns := watchKeyNamespace(w.StartKey)
		if ns == "" {
			continue
		}

		allowed, ok := s.cache[ns]
		if !ok {
			allowed = s.pdp.Has(s.email, domain.Permission{
				Object: domain.ObjectConfig,
				Action: domain.ActionRead,
				Domain: ns,
			})
			s.cache[ns] = allowed
		}

		if allowed {
			return true
		}
	}

	return false
}

func (s *scopeChecker) filter(clients []*domain.Client) []*domain.Client {
	if s.admin {
		return clients
	}

	out := make([]*domain.Client, 0, len(clients))
	for _, c := range clients {
		if s.visible(c) {
			out = append(out, c)
		}
	}

	return out
}

// watchKeyNamespace extracts the namespace from an etcd-encoded watch key
// like "/prod/api.json" → "prod". Returns "" for malformed or empty keys.
func watchKeyNamespace(key string) string {
	trimmed := strings.TrimPrefix(key, "/")
	if i := strings.Index(trimmed, "/"); i > 0 {
		return trimmed[:i]
	}

	return trimmed
}
