package clients

import (
	"strings"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// scopeChecker decides which clients a caller is allowed to see.
//
// Admins (anyone with the global client:read permission) bypass filtering.
// Everyone else only sees clients with at least one ActiveWatch in a namespace
// they can read; the readable-namespace set is resolved once via
// PDP.EffectiveDomains instead of probing each namespace individually. Clients
// with no live watches (e.g. historical entries) are invisible to non-admins
// because we have no per-namespace info to scope by.
type scopeChecker struct {
	admin bool
	// nsScope is the caller's readable namespaces (object=namespace,
	// action=read). Only consulted for non-admins; left zero-valued (empty,
	// non-wildcard) for admins since the admin fast-path short-circuits first.
	nsScope authz.DomainSet
}

func newScopeChecker(p pdp, email string) *scopeChecker {
	admin := p.Has(email, domain.Permission{
		Object: domain.ObjectClient,
		Action: domain.ActionRead,
		Domain: domain.DomainAll,
	})
	if admin {
		return &scopeChecker{admin: true}
	}

	return &scopeChecker{
		nsScope: p.EffectiveNamespaces(email, domain.ActionRead),
	}
}

func (s *scopeChecker) visible(c *domain.Client) bool {
	if s.admin {
		return true
	}

	for _, w := range c.ActiveWatchList {
		ns := watchKeyNamespace(w.StartKey)
		if ns != "" && s.nsScope.Contains(ns) {
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
