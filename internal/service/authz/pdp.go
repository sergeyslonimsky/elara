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
// Sibling gRuleNativeLen in pap.go covers g-rules.
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

func (p *PDP) EffectiveDomains(principal, object, action string) DomainSet {
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

		obj := rule[2]
		act := rule[3]

		if (obj == object || obj == "*") && (act == action || act == "*") {
			domains = append(domains, rule[1])
		}
	}

	return NewDomainSet(domains...)
}

func (p *PDP) Has(principal string, perm domain.Permission) bool {
	ok, err := p.enforcer.Enforce(principal, perm.Domain, perm.Object, perm.Action)
	if err != nil {
		slog.Error("pdp: enforce error", "principal", principal, "perm", perm, "err", err)

		return false
	}

	return ok
}

// HasGroupRead reports whether actor holds Group:Read on group:<id>.
func (p *PDP) HasGroupRead(actor, groupID string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectGroup,
		Action: domain.ActionRead,
		Domain: domain.GroupResource(groupID),
	})
}

// HasGroupWrite reports whether actor holds Group:Write on group:<id>.
func (p *PDP) HasGroupWrite(actor, groupID string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectGroup,
		Action: domain.ActionWrite,
		Domain: domain.GroupResource(groupID),
	})
}

// HasUserReadGlobal reports whether actor holds the global User:Read *.
// (User permissions are global by convention — see permission.proto.)
func (p *PDP) HasUserReadGlobal(actor string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectUser,
		Action: domain.ActionRead,
		Domain: domain.DomainAll,
	})
}

// HasUserWriteGlobal reports whether actor holds the global User:Write *.
func (p *PDP) HasUserWriteGlobal(actor string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectUser,
		Action: domain.ActionWrite,
		Domain: domain.DomainAll,
	})
}

// HasGroupCreate reports whether actor holds the global Group:Create *.
// Group creation has no scoped variant in our model (see proto comment).
func (p *PDP) HasGroupCreate(actor string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectGroup,
		Action: domain.ActionCreate,
		Domain: domain.DomainAll,
	})
}

// HasUserCreateGlobal reports whether actor holds the global User:Create *.
// User creation is either fully privileged (this) or derived through the
// caller supplying initial_group_ids backed by Group:Write — see CreateUser.
func (p *PDP) HasUserCreateGlobal(actor string) bool {
	return p.Has(actor, domain.Permission{
		Object: domain.ObjectUser,
		Action: domain.ActionCreate,
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
			Object: rule[2],
			Action: rule[3],
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
