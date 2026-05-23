package authz

//go:generate mockgen -destination=mocks/pdp_mock.go -package=authz_mock -source=pdp.go

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const casbinRuleLen = 4

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
		if len(rule) < casbinRuleLen {
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
		if len(rule) < casbinRuleLen {
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
