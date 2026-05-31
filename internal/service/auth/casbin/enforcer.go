package casbin

import (
	"context"
	"fmt"

	gocasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/casbin/casbin/v2/util"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
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
m = g(r.sub, p.sub, r.dom) && (p.dom == "*" || r.dom == p.dom) && objGrants(p.obj, r.obj) && actGrants(p.act, r.act)
`

// gRuleNativeLen is the number of elements returned by GetGroupingPolicy() (without type prefix): [user, role, domain].
const gRuleNativeLen = 3

// domainIdx is the index of the domain field in a native g rule [user, role, domain].
const domainIdx = 2

type (
	// PolicyRepository is the persistence contract for Casbin rules.
	// It must implement persist.Adapter for Casbin's initial load,
	// and provide context-aware mutation methods for EL-51 transactions.
	PolicyRepository interface {
		persist.Adapter
		AddPolicyCtx(ctx context.Context, sec, ptype string, rule []string) error
		RemovePolicyCtx(ctx context.Context, sec, ptype string, rule []string) error
		RemoveFilteredPolicyCtx(ctx context.Context, sec, ptype string, fieldIndex int, fieldValues ...string) error
		ListPermissionsForSubject(ctx context.Context, subject string) ([][]string, error)
	}
)

// Enforcer wraps the Casbin enforcer with domain-aware RBAC.
//
// AutoSave is permanently disabled: all runtime mutations must go through
// WithTx/WriteTx so writes are routed through PolicyRepo bound to the caller's
// transaction. The in-memory cache is synced post-commit via applyOpsToCache.
// This is what enables atomicity invariant §4 level 2 — a usecase can mutate
// domain entities and Casbin rules inside the same bbolt transaction.
type Enforcer struct {
	e        *gocasbin.Enforcer
	policies PolicyRepository
}

// NewEnforcer creates a new Enforcer using the given PolicyRepo.
// PolicyRepo implements persist.Adapter and is also used per-transaction via
// WithTx. If the policy is empty after loading, built-in role policies are
// seeded and saved.
func NewEnforcer(policies PolicyRepository) (*Enforcer, error) {
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

	// Object/action matching is delegated to domain.ObjectGrants/ActionGrants so
	// the matcher and the PDP rule scan share a single source of truth (wildcard
	// objects/actions and the write⊇read rule). See internal/domain/rbac.go.
	e.AddFunction("objGrants", func(args ...any) (any, error) {
		granted, _ := args[0].(string)
		required, _ := args[1].(string)

		return domain.ObjectGrants(domain.Object(granted), domain.Object(required)), nil
	})
	e.AddFunction("actGrants", func(args ...any) (any, error) {
		granted, _ := args[0].(string)
		required, _ := args[1].(string)

		return domain.ActionGrants(domain.Action(granted), domain.Action(required)), nil
	})

	// AutoSave stays off permanently. Runtime mutations flow through
	// WithTx/WriteTx; in-memory model is updated via applyOpsToCache after
	// the underlying transaction commits successfully. The superadmin group and
	// its membership are seeded by AdminBootstrap, not here.
	e.EnableAutoSave(false)

	return &Enforcer{e: e, policies: policies}, nil
}

// WithTx returns a per-transaction view of the enforcer. All mutations on the
// returned TxEnforcer write through PolicyRepo and record ops to
// be applied to the in-memory cache after commit.
func (e *Enforcer) WithTx(ctx context.Context) *TxEnforcer {
	return &TxEnforcer{
		parent: e,
		ctx:    ctx,
	}
}

// WriteTx opens a write transaction via txm and invokes fn with the tx and a
// per-tx enforcer view. On success, the recorded ops are applied to the
// in-memory cache. On error, the cache is left untouched (the bbolt tx is
// rolled back by Manager, so persisted state matches the cache).
func (e *Enforcer) WriteTx(
	ctx context.Context,
	txm storage.Manager,
	fn func(context.Context, *TxEnforcer) error,
) error {
	var txe *TxEnforcer

	err := txm.WithTx(ctx, func(ctx context.Context) error {
		txe = e.WithTx(ctx)

		return fn(ctx, txe)
	})
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}

	if txe != nil {
		if err := e.applyOpsToCache(txe.ops); err != nil {
			// The bbolt tx is committed; the in-memory cache is now ahead
			// of or behind the persisted state. Full reload is the only
			// safe recovery — anything else risks privilege-evaluation
			// inconsistencies for the rest of the process lifetime.
			if reloadErr := e.LoadPolicy(); reloadErr != nil {
				// Reload failed too: persisted state is good but the
				// cache cannot be reconciled. Panic so the supervisor
				// restarts us, rather than serve stale authz answers.
				panic(fmt.Sprintf(
					"casbin: cache desync after commit and LoadPolicy failed: apply=%v reload=%v",
					err, reloadErr,
				))
			}

			return fmt.Errorf("sync casbin cache (recovered via LoadPolicy): %w", err)
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
// subject (use GroupSubject).
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
