package casbin

import (
	"fmt"

	gocasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"

	authpkg "github.com/sergeyslonimsky/elara/internal/auth"
)

const casbinModel = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub, r.dom) || g(r.sub, p.sub, "*")) && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

// gRuleNativeLen is the number of elements returned by GetGroupingPolicy() (without type prefix): [user, role, domain].
const gRuleNativeLen = 3

// domainIdx is the index of the domain field in a native g rule [user, role, domain].
const domainIdx = 2

// Enforcer wraps the Casbin enforcer with domain-aware RBAC.
type Enforcer struct {
	e *gocasbin.Enforcer
}

// NewEnforcer creates a new Enforcer using the given persist.Adapter.
// If the policy is empty after loading, built-in role policies are seeded and saved.
func NewEnforcer(adapter persist.Adapter) (*Enforcer, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("build casbin model: %w", err)
	}

	// Disable AutoSave before creating so no incremental writes happen during initial load/seed.
	// We build without an adapter first, then re-initialize to control the flow.
	// Instead: create with adapter (LoadPolicy is called internally by NewEnforcer),
	// then disable AutoSave only for the seeding phase.
	e, err := gocasbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	// Disable AutoSave during initial seeding so we can do a single atomic save.
	e.EnableAutoSave(false)

	enforcer := &Enforcer{e: e}

	pRules, _ := e.GetPolicy()
	gRules, _ := e.GetGroupingPolicy()

	if len(pRules) == 0 && len(gRules) == 0 {
		if err := enforcer.seedBuiltinPolicies(); err != nil {
			return nil, err
		}

		if err := e.SavePolicy(); err != nil {
			return nil, fmt.Errorf("save seeded policy: %w", err)
		}
	}

	// Enable AutoSave for all runtime mutations.
	e.EnableAutoSave(true)

	return enforcer, nil
}

// Enforce checks whether subject can perform action on object in domain.
func (e *Enforcer) Enforce(subject, domain, object, action string) (bool, error) {
	ok, err := e.e.Enforce(subject, domain, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce: %w", err)
	}

	return ok, nil
}

// AddPolicy adds a permission rule (p rule) to the enforcer.
func (e *Enforcer) AddPolicy(sub, dom, obj, act string) error {
	if _, err := e.e.AddPolicy(sub, dom, obj, act); err != nil {
		return fmt.Errorf("add policy: %w", err)
	}

	return nil
}

// RemovePolicy removes a permission rule (p rule) from the enforcer.
func (e *Enforcer) RemovePolicy(sub, dom, obj, act string) error {
	if _, err := e.e.RemovePolicy(sub, dom, obj, act); err != nil {
		return fmt.Errorf("remove policy: %w", err)
	}

	return nil
}

// AddRoleForUser assigns a role to a user within a domain.
func (e *Enforcer) AddRoleForUser(user, role, domain string) error {
	if _, err := e.e.AddGroupingPolicy(user, role, domain); err != nil {
		return fmt.Errorf("add role for user: %w", err)
	}

	return nil
}

// RemoveRoleForUser removes a role assignment from a user within a domain.
func (e *Enforcer) RemoveRoleForUser(user, role, domain string) error {
	if _, err := e.e.RemoveGroupingPolicy(user, role, domain); err != nil {
		return fmt.Errorf("remove role for user: %w", err)
	}

	return nil
}

// GetRolesForUser returns the roles assigned to a user in the given domain.
func (e *Enforcer) GetRolesForUser(user, domain string) ([]string, error) {
	roles, err := e.e.GetRolesForUser(user, domain)
	if err != nil {
		return nil, fmt.Errorf("get roles for user: %w", err)
	}

	return roles, nil
}

// GetAllRoles returns all roles that have assignments in the given domain.
func (e *Enforcer) GetAllRoles(domain string) ([]string, error) {
	grouping, err := e.e.GetGroupingPolicy()
	if err != nil {
		return nil, fmt.Errorf("get grouping policy: %w", err)
	}

	seen := make(map[string]struct{})
	var roles []string

	for _, rule := range grouping {
		// GetGroupingPolicy returns native rules without type prefix: [user, role, domain]
		if len(rule) < gRuleNativeLen {
			continue
		}

		if rule[domainIdx] == domain || domain == authpkg.ObjectAll {
			role := rule[1]
			if _, exists := seen[role]; !exists {
				seen[role] = struct{}{}
				roles = append(roles, role)
			}
		}
	}

	return roles, nil
}

// GetPolicy returns all p (permission) rules.
func (e *Enforcer) GetPolicy() [][]string {
	rules, _ := e.e.GetPolicy()

	return rules
}

// GetGroupingPolicy returns all g (role assignment) rules.
func (e *Enforcer) GetGroupingPolicy() [][]string {
	rules, _ := e.e.GetGroupingPolicy()

	return rules
}

// GetRulesForSubject returns all [subject, role, domain] g-rules where subject matches.
func (e *Enforcer) GetRulesForSubject(subject string) [][]string {
	all, _ := e.e.GetGroupingPolicy()

	var result [][]string

	for _, rule := range all {
		if len(rule) == gRuleNativeLen && rule[0] == subject {
			result = append(result, rule)
		}
	}

	return result
}

// SeedPassthroughAdmin adds the g-rule for the passthrough user used when auth is disabled.
func (e *Enforcer) SeedPassthroughAdmin() error {
	if _, err := e.e.AddGroupingPolicy("local-admin@elara.internal", authpkg.RoleAdmin, authpkg.ObjectAll); err != nil {
		return fmt.Errorf("seed passthrough admin: %w", err)
	}

	return nil
}

func (e *Enforcer) seedBuiltinPolicies() error {
	policies := [][]string{
		// admin — wildcard covers everything
		{authpkg.RoleAdmin, authpkg.ObjectAll, authpkg.ObjectAll, authpkg.ActionAll},

		// writer
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectConfig, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectConfig, authpkg.ActionWrite},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectNamespace, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectWebhook, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectSchema, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectClient, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectDashboard, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectToken, authpkg.ActionRead},
		{authpkg.RoleWriter, authpkg.ObjectAll, authpkg.ObjectToken, authpkg.ActionWrite},

		// reader
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectConfig, authpkg.ActionRead},
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectNamespace, authpkg.ActionRead},
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectClient, authpkg.ActionRead},
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectDashboard, authpkg.ActionRead},
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectToken, authpkg.ActionRead},
		{authpkg.RoleReader, authpkg.ObjectAll, authpkg.ObjectToken, authpkg.ActionWrite},
	}

	for _, p := range policies {
		if _, err := e.e.AddPolicy(p[0], p[1], p[2], p[3]); err != nil {
			return fmt.Errorf("seed built-in policy %v: %w", p, err)
		}
	}

	return nil
}
