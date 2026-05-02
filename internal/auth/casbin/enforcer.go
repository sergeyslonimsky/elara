package casbin

import (
	"fmt"

	gocasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
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
m = g(r.sub, p.sub, r.dom) && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

const (
	roleAdmin  = "role:admin"
	roleEditor = "role:editor"
	roleViewer = "role:viewer"

	objectAll       = "*"
	objectConfig    = "config"
	objectNamespace = "namespace"

	actionAll   = "*"
	actionRead  = "read"
	actionWrite = "write"
)

// pRuleLen is the number of elements in a serialized p rule: [type, sub, dom, obj, act].
const pRuleLen = 5

// gRuleLen is the number of elements in a serialized g rule (with type prefix): [type, user, role, domain].
const gRuleLen = 4

// gRuleNativeLen is the number of elements returned by GetGroupingPolicy() (without type prefix): [user, role, domain].
const gRuleNativeLen = 3

// domainIdx is the index of the domain field in a native g rule [user, role, domain].
const domainIdx = 2

// PolicyLoader is satisfied by bbolt.PolicyRepo (already implemented).
type PolicyLoader interface {
	Load() ([][]string, error)
	Save(rules [][]string) error
}

// Enforcer wraps the Casbin enforcer with domain-aware RBAC.
type Enforcer struct {
	e *gocasbin.Enforcer
}

// NewEnforcer creates a new Enforcer using the given PolicyLoader.
// If the loaded policy is empty, built-in role policies are seeded and saved.
func NewEnforcer(loader PolicyLoader) (*Enforcer, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("build casbin model: %w", err)
	}

	// Pass only the model — casbin skips LoadPolicy when no adapter is provided.
	// We populate rules manually from the PolicyLoader below.
	e, err := gocasbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	e.EnableAutoSave(false)

	rules, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	enforcer := &Enforcer{e: e}

	if len(rules) == 0 {
		if err := enforcer.seedBuiltinPolicies(); err != nil {
			return nil, err
		}

		if err := enforcer.SavePolicy(loader); err != nil {
			return nil, err
		}

		return enforcer, nil
	}

	if err := enforcer.loadRules(rules); err != nil {
		return nil, err
	}

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

		if rule[domainIdx] == domain || domain == "*" {
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

// SavePolicy persists the current in-memory policy state to the given loader.
func (e *Enforcer) SavePolicy(loader PolicyLoader) error {
	pRules, err := e.e.GetPolicy()
	if err != nil {
		return fmt.Errorf("get policy for save: %w", err)
	}

	gRules, err := e.e.GetGroupingPolicy()
	if err != nil {
		return fmt.Errorf("get grouping policy for save: %w", err)
	}

	rules := make([][]string, 0, len(pRules)+len(gRules))

	for _, r := range pRules {
		rules = append(rules, append([]string{"p"}, r...))
	}

	for _, r := range gRules {
		rules = append(rules, append([]string{"g"}, r...))
	}

	if err = loader.Save(rules); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}

func (e *Enforcer) seedBuiltinPolicies() error {
	policies := [][]string{
		{roleAdmin, "*", objectAll, actionAll},
		{roleEditor, "*", objectConfig, actionRead},
		{roleEditor, "*", objectConfig, actionWrite},
		{roleViewer, "*", objectConfig, actionRead},
		{roleEditor, "*", objectNamespace, actionRead},
		{roleViewer, "*", objectNamespace, actionRead},
	}

	for _, p := range policies {
		if _, err := e.e.AddPolicy(p[0], p[1], p[2], p[3]); err != nil {
			return fmt.Errorf("seed built-in policy %v: %w", p, err)
		}
	}

	return nil
}

func (e *Enforcer) loadRules(rules [][]string) error {
	for _, rule := range rules {
		if len(rule) == 0 {
			continue
		}

		switch rule[0] {
		case "p":
			if len(rule) < pRuleLen {
				continue
			}

			if _, err := e.e.AddPolicy(rule[1], rule[2], rule[3], rule[4]); err != nil {
				return fmt.Errorf("load policy rule %v: %w", rule, err)
			}
		case "g":
			if len(rule) < gRuleLen {
				continue
			}

			if _, err := e.e.AddGroupingPolicy(rule[1], rule[2], rule[3]); err != nil {
				return fmt.Errorf("load grouping rule %v: %w", rule, err)
			}
		}
	}

	return nil
}
