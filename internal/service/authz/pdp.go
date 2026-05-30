package authz

//go:generate mockgen -destination=mocks/pdp_mock.go -package=authz_mock -source=pdp.go

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

// pRuleLen is the column count of a Casbin p-rule projected by
// GetImplicitPermissionsForUser / GetPolicy: [sub, dom, obj, act].
const pRuleLen = 4

type enforcer interface {
	GetImplicitPermissionsForUser(user string) ([][]string, error)
	Enforce(subject, domainStr, object, action string) (bool, error)
}

type PDP struct {
	enforcer enforcer
}

func NewPDP(e enforcer) *PDP {
	return &PDP{enforcer: e}
}

func (p *PDP) EffectiveDomains(principal string, object domain.Object, action domain.Action) DomainSet {
	rules, err := p.enforcer.GetImplicitPermissionsForUser(principal)
	if err != nil {
		slog.Error("pdp: failed to get implicit permissions", "principal", principal, "err", err)

		return NewDomainSet()
	}

	var domains []string
	for _, rule := range rules {
		// rule format: [sub, dom, obj, act]
		if len(rule) < pRuleLen {
			continue
		}

		// Same matching semantics as the Casbin matcher (domain.ObjectGrants /
		// ActionGrants): wildcards and write⊇read. Keeps this scan and Enforce
		// in agreement — e.g. a namespace:write grant surfaces for a read query.
		if domain.ObjectGrants(domain.Object(rule[2]), object) &&
			domain.ActionGrants(domain.Action(rule[3]), action) {
			domains = append(domains, rule[1])
		}
	}

	return NewDomainSet(domains...)
}

func (p *PDP) Has(principal string, perm domain.Permission) bool {
	ok, err := p.enforcer.Enforce(principal, perm.Domain, string(perm.Object), string(perm.Action))
	if err != nil {
		slog.Error("pdp: enforce error", "principal", principal, "perm", perm, "err", err)

		return false
	}

	return ok
}

// HasGroup reports whether actor holds the given action on group:<id>.
// Hides the group:<id> domain convention from callers.
func (p *PDP) HasGroup(actor, groupID string, action domain.Action) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectGroup,
		Action: action,
		Domain: domain.GroupResource(groupID),
	})
}

// HasGlobal reports whether actor holds the given (object, action) on the
// global "*" domain. Used for objects whose permissions are global by
// convention — e.g. User:Read/Write/Create and Group:Create (see
// permission.proto). User creation may also be derived through the caller
// supplying initial_group_ids backed by Group:Write — see CreateUser.
func (p *PDP) HasGlobal(actor string, object domain.Object, action domain.Action) bool {
	return p.Has(actor, domain.Permission{
		Object: object,
		Action: action,
		Domain: domain.DomainAll,
	})
}

// HasForGroup reports whether the group (as a Casbin subject) holds `perm`.
// Used by anti-escalation cascade checks where the principal is a group
// rather than a user — hides the subject-string convention from callers.
func (p *PDP) HasForGroup(groupName string, perm domain.Permission) bool {
	return p.Has(casbin.GroupSubject(groupName), perm)
}

// ListPermissions returns all effective permissions for the given principal,
// deduplicated and sorted deterministically by (Object, Action, Domain).
// Wildcards in any field are returned as-is using domain.ObjectAll / ActionAll
// / DomainAll. Returns a non-nil empty slice when there are no rules.
func (p *PDP) ListPermissions(principal string) ([]domain.Permission, error) {
	rules, err := p.enforcer.GetImplicitPermissionsForUser(principal)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	seen := make(map[domain.Permission]struct{}, len(rules))
	permissions := make([]domain.Permission, 0, len(rules))

	for _, rule := range rules {
		// rule format: [sub, dom, obj, act]; skip malformed.
		if len(rule) < pRuleLen {
			continue
		}

		perm := domain.Permission{
			Domain: rule[1],
			Object: domain.Object(rule[2]),
			Action: domain.Action(rule[3]),
		}
		if _, dup := seen[perm]; dup {
			continue
		}

		seen[perm] = struct{}{}
		permissions = append(permissions, perm)
	}

	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Object != permissions[j].Object {
			return permissions[i].Object < permissions[j].Object
		}

		if permissions[i].Action != permissions[j].Action {
			return permissions[i].Action < permissions[j].Action
		}

		return permissions[i].Domain < permissions[j].Domain
	})

	return permissions, nil
}
