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
	enforcer enforcer
	email    string
	admin    bool
	cache    map[string]bool // namespace → allowed
}

func newScopeChecker(enforcer enforcer, email string) *scopeChecker {
	admin, _ := enforcer.Enforce(email, domain.DomainAll, domain.ObjectClient, domain.ActionRead)

	return &scopeChecker{
		enforcer: enforcer,
		email:    email,
		admin:    admin,
		cache:    map[string]bool{},
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
			allowed, _ = s.enforcer.Enforce(s.email, ns, domain.ObjectConfig, domain.ActionRead)
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
