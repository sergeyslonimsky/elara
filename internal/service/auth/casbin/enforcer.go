package casbin

import (
	"context"
	"fmt"

	gocasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// The matcher relies on a domain matching function registered below
// (AddNamedDomainMatchingFunc) so that a g-rule stored with domain "*" matches
// any queried domain. This is what lets transitive chains such as
//
//	g, alice, group:devs, *      (membership, global)
//	g, group:devs, admin, prod   (group role in prod)
//
// resolve via Casbin's native recursion without application-side fan-out.
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

// gRuleNativeLen is the number of elements returned by GetGroupingPolicy() (without type prefix): [user, role, domain].
const gRuleNativeLen = 3

// domainIdx is the index of the domain field in a native g rule [user, role, domain].
const domainIdx = 2

// Enforcer wraps the Casbin enforcer with domain-aware RBAC.
//
// AutoSave is permanently disabled: all runtime mutations must go through
// WithTx/WriteTx so writes are routed through PolicyRepo bound to the caller's
// transaction. The in-memory cache is synced post-commit via applyOpsToCache.
// This is what enables atomicity invariant §4 level 2 — a usecase can mutate
// domain entities and Casbin rules inside the same bbolt transaction.
type Enforcer struct {
	e        *gocasbin.Enforcer
	policies *bbolt.PolicyRepo
}

// NewEnforcer creates a new Enforcer using the given PolicyRepo.
// PolicyRepo implements persist.Adapter and is also used per-transaction via
// WithTx. If the policy is empty after loading, built-in role policies are
// seeded and saved.
func NewEnforcer(policies *bbolt.PolicyRepo) (*Enforcer, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("build casbin model: %w", err)
	}

	e, err := gocasbin.NewEnforcer(m, policies)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}

	// Register a domain matching function on the "g" role manager so that "*"
	// stored in the domain field of a g-rule acts as a wildcard during recursive
	// role traversal. Without this, a membership rule g(alice, group:devs, "*")
	// would not match a query g(alice, group:devs, "prod").
	e.AddNamedDomainMatchingFunc("g", "keyMatch", util.KeyMatch)

	// Disable AutoSave during initial seeding so we can do a single atomic save.
	e.EnableAutoSave(false)

	enforcer := &Enforcer{e: e, policies: policies}

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

	// AutoSave stays off permanently. Runtime mutations flow through
	// WithTx/WriteTx; in-memory model is updated via applyOpsToCache after
	// the underlying transaction commits successfully.
	return enforcer, nil
}

// WithTx returns a per-transaction view of the enforcer. All mutations on the
// returned TxEnforcer write through PolicyRepo bound to tx and record ops to
// be applied to the in-memory cache after commit.
func (e *Enforcer) WithTx(tx storage.Tx) *TxEnforcer {
	return &TxEnforcer{
		parent:   e,
		policies: e.policies.WithTx(tx),
	}
}

// WriteTx opens a write transaction via txm and invokes fn with the tx and a
// per-tx enforcer view. On success, the recorded ops are applied to the
// in-memory cache. On error, the cache is left untouched (the bbolt tx is
// rolled back by TxManager, so persisted state matches the cache).
func (e *Enforcer) WriteTx(
	ctx context.Context,
	txm storage.TxManager,
	fn func(storage.Tx, *TxEnforcer) error,
) error {
	var txe *TxEnforcer

	err := txm.Write(ctx, func(tx storage.Tx) error {
		txe = e.WithTx(tx)

		return fn(tx, txe)
	})
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}

	if txe != nil {
		if err := e.applyOpsToCache(txe.ops); err != nil {
			return fmt.Errorf("sync casbin cache: %w", err)
		}
	}

	return nil
}

// LoadPolicy reloads the in-memory policy model from the persistence adapter.
// Used to resync the enforcer cache after out-of-band writes that bypass the
// enforcer (e.g. AdminBootstrap writes raw rules directly to the policy
// repository in a shared transaction).
func (e *Enforcer) LoadPolicy() error {
	if err := e.e.LoadPolicy(); err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	return nil
}

// Enforce checks whether subject can perform action on object in domain.
func (e *Enforcer) Enforce(subject, domainStr, object, action string) (bool, error) {
	ok, err := e.e.Enforce(subject, domainStr, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce: %w", err)
	}

	return ok, nil
}

// GetRolesForUser returns the roles assigned to a user in the given domain.
func (e *Enforcer) GetRolesForUser(user, domainStr string) ([]string, error) {
	roles, err := e.e.GetRolesForUser(user, domainStr)
	if err != nil {
		return nil, fmt.Errorf("get roles for user: %w", err)
	}

	return roles, nil
}

// GetAllRoles returns all roles that have assignments in the given domain.
func (e *Enforcer) GetAllRoles(domainStr string) ([]string, error) {
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

		if rule[domainIdx] == domainStr || domainStr == domain.DomainAll {
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

// GetImplicitPermissionsForUser returns all permissions inherited by the user,
// including those through roles. Returns rules in format [sub, dom, obj, act].
func (e *Enforcer) GetImplicitPermissionsForUser(user string) ([][]string, error) {
	rules, err := e.e.GetImplicitPermissionsForUser(user)
	if err != nil {
		return nil, fmt.Errorf("get implicit permissions for user: %w", err)
	}

	return rules, nil
}

// GetMembersOfGroup returns the subjects (users) that belong to the given
// group subject, i.e. the first column of every g-rule where the second
// column equals groupSubject. groupSubject must be the fully-prefixed group
// subject (use domain.GroupSubject).
func (e *Enforcer) GetMembersOfGroup(groupSubject string) []string {
	all, _ := e.e.GetGroupingPolicy()

	var members []string

	for _, rule := range all {
		if len(rule) == gRuleNativeLen && rule[1] == groupSubject {
			members = append(members, rule[0])
		}
	}

	return members
}

// applyOpsToCache replays the ops recorded during a successful tx onto the
// in-memory casbin model. With AutoSave=off these calls do not re-hit the
// adapter — they only update the cached rules.
func (e *Enforcer) applyOpsToCache(ops []op) error { //nolint:cyclop //refactor
	for _, o := range ops {
		switch o.kind {
		case opAddP:
			if _, err := e.e.AddPolicy(o.args[0], o.args[1], o.args[2], o.args[3]); err != nil {
				return fmt.Errorf("cache add policy: %w", err)
			}
		case opRemoveP:
			if _, err := e.e.RemovePolicy(o.args[0], o.args[1], o.args[2], o.args[3]); err != nil {
				return fmt.Errorf("cache remove policy: %w", err)
			}
		case opAddG:
			if _, err := e.e.AddGroupingPolicy(o.args[0], o.args[1], o.args[2]); err != nil {
				return fmt.Errorf("cache add grouping policy: %w", err)
			}
		case opRemoveG:
			if _, err := e.e.RemoveGroupingPolicy(o.args[0], o.args[1], o.args[2]); err != nil {
				return fmt.Errorf("cache remove grouping policy: %w", err)
			}
		case opDeleteUser:
			if _, err := e.e.DeleteUser(o.user); err != nil {
				return fmt.Errorf("cache delete user: %w", err)
			}
		}
	}

	return nil
}

func (e *Enforcer) seedBuiltinPolicies() error {
	// Columns: {role, domain, object, action}. Roles are granted in every
	// domain via DomainAll; for the admin role the object is also a wildcard.
	policies := [][]string{
		// admin — wildcard covers everything
		{domain.RoleAdmin, domain.DomainAll, domain.ObjectAll, domain.ActionAll},

		// writer
		{domain.RoleWriter, domain.DomainAll, domain.ObjectConfig, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectConfig, domain.ActionWrite},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectNamespace, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectWebhook, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectSchema, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectClient, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectDashboard, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectToken, domain.ActionRead},
		{domain.RoleWriter, domain.DomainAll, domain.ObjectToken, domain.ActionWrite},

		// reader
		{domain.RoleReader, domain.DomainAll, domain.ObjectConfig, domain.ActionRead},
		{domain.RoleReader, domain.DomainAll, domain.ObjectNamespace, domain.ActionRead},
		{domain.RoleReader, domain.DomainAll, domain.ObjectClient, domain.ActionRead},
		{domain.RoleReader, domain.DomainAll, domain.ObjectDashboard, domain.ActionRead},
		{domain.RoleReader, domain.DomainAll, domain.ObjectToken, domain.ActionRead},
		{domain.RoleReader, domain.DomainAll, domain.ObjectToken, domain.ActionWrite},
	}

	for _, p := range policies {
		if _, err := e.e.AddPolicy(p[0], p[1], p[2], p[3]); err != nil {
			return fmt.Errorf("seed built-in policy %v: %w", p, err)
		}
	}

	return nil
}
