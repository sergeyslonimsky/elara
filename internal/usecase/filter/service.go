package filter

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=filter_mock -source=service.go

type (
	// permissions enumerates a principal's effective RBAC permissions. Backed
	// by authz.PDP.ListPermissions.
	permissions interface {
		ListPermissions(principal string) ([]domain.Permission, error)
	}

	namespaceLister interface {
		List(
			ctx context.Context,
			filter domain.NamespaceFilter,
			params domain.NamespaceListParams,
		) ([]*domain.Namespace, int, error)
	}

	groupLister interface {
		List(
			ctx context.Context,
			filter domain.GroupFilter,
			params domain.GroupListParams,
		) ([]*domain.Group, int, error)
	}

	userLister interface {
		List(
			ctx context.Context,
			filter domain.UserFilter,
			params domain.UserListParams,
		) ([]*domain.User, int, error)
	}
)

// Item is a single selectable filter option. Actions is the full set of
// actions the caller holds on the underlying resource (collapsed to a single
// ActionAll when the caller holds a wildcard grant).
type Item struct {
	Key     string
	Value   string
	Actions []domain.Action
}

// Query carries the per-request inputs shared by every Filter method: the
// action set to gate visibility by (OR semantics, see grants) and an optional
// case-insensitive substring search applied by the repository.
type Query struct {
	Actions []domain.Action
	Search  string
}

// Service answers "which namespaces/groups/users can this caller act on, and
// with which actions" — the data backing the UI's filter selects. It is a
// pure read service: visibility is enforced entirely by the caller's
// effective permissions, so it never gates on a single Require check.
type Service struct {
	perms      permissions
	namespaces namespaceLister
	groups     groupLister
	users      userLister
}

func New(
	perms permissions,
	namespaces namespaceLister,
	groups groupLister,
	users userLister,
) *Service {
	return &Service{
		perms:      perms,
		namespaces: namespaces,
		groups:     groups,
		users:      users,
	}
}

// actionSet is a small set of domain actions with set-algebra helpers used
// while aggregating a caller's effective actions on a resource.
type actionSet map[domain.Action]struct{}

func (s actionSet) add(a domain.Action) { s[a] = struct{}{} }

func (s actionSet) union(other actionSet) {
	for a := range other {
		s[a] = struct{}{}
	}
}

func (s actionSet) empty() bool { return len(s) == 0 }

// scopedActions splits a principal's permissions for the given object into:
//   - wildcard: actions granted on every resource of that object (domain "*")
//   - explicit: per-domain actions, keyed by the raw Casbin domain string
//     (a namespace name, or "group:<id>" for groups)
//
// Object matching uses domain.ObjectGrants so an ObjectAll ("*") grant
// contributes to every object, mirroring the Casbin enforcement path.
func scopedActions(perms []domain.Permission, obj domain.Object) (actionSet, map[string]actionSet) {
	wildcard := actionSet{}
	explicit := map[string]actionSet{}

	for _, p := range perms {
		if !domain.ObjectGrants(p.Object, obj) {
			continue
		}

		if p.Domain == domain.DomainAll {
			wildcard.add(p.Action)

			continue
		}

		set, ok := explicit[p.Domain]
		if !ok {
			set = actionSet{}
			explicit[p.Domain] = set
		}

		set.add(p.Action)
	}

	return wildcard, explicit
}

// grants reports whether the actions the caller holds satisfy at least one of
// the requested actions, using domain.ActionGrants (write ⊇ read, "*" ⊇
// anything). An empty request matches any non-empty holding — "anything I can
// touch".
func grants(have actionSet, requested []domain.Action) bool {
	if have.empty() {
		return false
	}

	if len(requested) == 0 {
		return true
	}

	for _, req := range requested {
		for held := range have {
			if domain.ActionGrants(held, req) {
				return true
			}
		}
	}

	return false
}

// resolve renders the caller's held actions for the response. A wildcard
// holding collapses to a single ActionAll (per spec: if the caller holds
// "all", return "all"); otherwise actions are emitted in a canonical,
// deterministic order.
func resolve(have actionSet) []domain.Action {
	if _, ok := have[domain.ActionAll]; ok {
		return []domain.Action{domain.ActionAll}
	}

	order := []domain.Action{
		domain.ActionRead,
		domain.ActionWrite,
		domain.ActionCreate,
		domain.ActionDelete,
	}

	out := make([]domain.Action, 0, len(have))
	for _, a := range order {
		if _, ok := have[a]; ok {
			out = append(out, a)
		}
	}

	return out
}
