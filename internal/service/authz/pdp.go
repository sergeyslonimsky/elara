package authz

//go:generate mockgen -destination=mocks/pdp_mock.go -package=authz_mock -source=pdp.go

import (
	"log/slog"

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

func (p *PDP) EffectiveDomains(principal, object, action string) domain.DomainSet {
	rules, err := p.enforcer.GetImplicitPermissionsForUser(principal)
	if err != nil {
		slog.Error("pdp: failed to get implicit permissions", "principal", principal, "err", err)

		return domain.NewDomainSet()
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

	return domain.NewDomainSet(domains...)
}

func (p *PDP) Has(principal string, perm domain.Permission) bool {
	ok, err := p.enforcer.Enforce(principal, perm.Domain, perm.Object, perm.Action)
	if err != nil {
		slog.Error("pdp: enforce error", "principal", principal, "perm", perm, "err", err)

		return false
	}

	return ok
}
