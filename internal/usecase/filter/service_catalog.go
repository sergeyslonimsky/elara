package filter

import (
	"sync"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ObjectScope mirrors filterv1.ObjectScope at the domain layer so the catalog
// stays free of proto imports. The handler converts to the proto enum.
type ObjectScope int

const (
	ScopeUnspecified ObjectScope = iota
	// ScopeGlobal: assignment.Domain must be domain.DomainAll ("*"). Granted
	// permission carries no resource identity (User, Token, Webhook, Client).
	ScopeGlobal
	// ScopeNamespace: assignment.Domain is a namespace name, or "*" for every
	// namespace. UI picks via FilterService.GetNamespaces.
	ScopeNamespace
	// ScopeGroup: assignment.Domain is domain.GroupResource(id), or "*" for
	// every group. UI picks via FilterService.GetGroups.
	ScopeGroup
)

// CatalogEntry describes one PermissionObject: how its domain field is
// interpreted, and which actions are meaningful on it.
type CatalogEntry struct {
	Object  domain.Object
	Scope   ObjectScope
	Actions []domain.Action
}

// buildCatalog returns the static source-of-truth table that drives both the
// UI form and server-side assignment validation.
//
// Entries deliberately omit domain.ObjectAll / domain.ObjectPolicy /
// domain.ObjectDashboard — those are either internal wildcards or have no
// admin-facing assignment flow yet.
//
// Action choices reflect what handlers actually enforce today:
//   - Namespace / Group are full CRUD + read/write.
//   - User: no Delete (deletion is a soft, group-based flow today).
//   - Token / Webhook: full CRUD.
//   - Client: read-only — connected-clients monitor is observation only.
func buildCatalog() []CatalogEntry {
	return []CatalogEntry{
		{
			Object: domain.ObjectNamespace,
			Scope:  ScopeNamespace,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectGroup,
			Scope:  ScopeGroup,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectUser,
			Scope:  ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectToken,
			Scope:  ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectWebhook,
			Scope:  ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectClient,
			Scope:  ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
			},
		},
	}
}

// loadCatalog memoizes buildCatalog so the table is constructed exactly once.
// sync.OnceValue is the idiomatic substitute for eager init here: the table is
// immutable and shared by the package-level LookupCatalogEntry helper, which
// has no Service receiver to attach state to.
//
//nolint:gochecknoglobals // immutable memoized lookup; see comment above.
var loadCatalog = sync.OnceValue(buildCatalog)

// Catalog returns the permission catalog. The returned slice is a shallow
// copy of the memoized table; callers may freely append but must not mutate
// entry fields in place.
func (s *Service) Catalog() []CatalogEntry {
	cat := loadCatalog()
	out := make([]CatalogEntry, len(cat))
	copy(out, cat)

	return out
}

// LookupCatalogEntry returns the entry for obj, or false if obj has no
// admin-assignable permissions (e.g. ObjectAll, ObjectPolicy). Exported so
// server-side validators (usecase/group) can share the same source of truth.
func LookupCatalogEntry(obj domain.Object) (CatalogEntry, bool) {
	for _, e := range loadCatalog() {
		if e.Object == obj {
			return e, true
		}
	}

	return CatalogEntry{}, false
}
