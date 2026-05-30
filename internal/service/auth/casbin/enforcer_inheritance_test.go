package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

// TestEnforcer_DynamicGroupInheritance verifies that role inheritance through
// groups is handled by Casbin's recursive role manager when membership rules
// are stored in the wildcard domain "*". No application-side fan-out is needed
// — each canonical edge is a single g-rule.
//
// Canonical layout under test:
//
//	g, alice@x, group:devs, *      // membership, global
//	g, group:devs, admin, prod     // group role in a single domain
//
// Enforce("alice@x", "prod", *, *) must resolve via the chain
//
//	alice@x -> group:devs (any domain) -> admin (prod)
//
// purely through Casbin's domain-aware role traversal.
func TestEnforcer_DynamicGroupInheritance(t *testing.T) {
	t.Parallel()

	const (
		alice = "alice@example.com"
		bob   = "bob@example.com"
		devs  = "devs"
	)

	type request struct {
		domain string
		object domain.Object
		action domain.Action
		want   bool
	}

	// extraCheck lets a test case assert Enforce results for subjects other
	// than alice (e.g. proving a group-level mutation propagates to every
	// member through Casbin's recursion alone).
	type extraCheck struct {
		subject string
		domain  string
		object  domain.Object
		action  domain.Action
		want    bool
	}

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) *casbin.Enforcer
		requests      []request
		extraSubjects []extraCheck
	}{
		{
			name: "member inherits group role in target domain",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e, txm := newTestEnforcerWithTxM(t, nil)
				seedRoleTemplates(t, e, txm)
				seedRole(t, e, txm, alice, casbin.GroupSubject(devs), domain.MembershipDomain)
				seedRole(t, e, txm, casbin.GroupSubject(devs), string(domain.RoleAdmin), "prod")

				return e
			},
			requests: []request{
				{
					domain: "prod",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   true,
				},
				// group has no role in staging -> no access there even though
				// the membership rule is in domain "*".
				{
					domain: "staging",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   false,
				},
			},
		},
		{
			name: "revoking role at group level removes access for every member without touching membership",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e, txm := newTestEnforcerWithTxM(t, nil)

				// Two memberships in domain "*" (global) — one Casbin mutation each.
				seedRole(t, e, txm, alice, casbin.GroupSubject(devs), domain.MembershipDomain)
				seedRole(t, e, txm, bob, casbin.GroupSubject(devs), domain.MembershipDomain)

				// Group gets admin in prod, then loses it. Exactly one
				// AddRoleForUser + one RemoveRoleForUser — no per-member loop.
				seedRole(t, e, txm, casbin.GroupSubject(devs), string(domain.RoleAdmin), "prod")
				removeRole(t, e, txm, casbin.GroupSubject(devs), string(domain.RoleAdmin), "prod")

				// Membership rules in domain "*" remain in place — verify the
				// invariant rather than trusting the absence of a sync loop.
				rules := e.GetRulesForSubject(alice)
				require.Len(t, rules, 1)
				assert.Equal(
					t,
					[]string{alice, casbin.GroupSubject(devs), domain.MembershipDomain},
					rules[0],
				)

				return e
			},
			requests: []request{
				// alice loses access via the group revoke...
				{
					domain: "prod",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   false,
				},
			},
			extraSubjects: []extraCheck{
				// ...and so does bob, through the same single mutation.
				{
					subject: bob,
					domain:  "prod",
					object:  domain.ObjectNamespace,
					action:  domain.ActionWrite,
					want:    false,
				},
			},
		},
		{
			name: "removing membership revokes access without touching group's role",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e, txm := newTestEnforcerWithTxM(t, nil)
				seedRoleTemplates(t, e, txm)
				seedRole(t, e, txm, alice, casbin.GroupSubject(devs), domain.MembershipDomain)
				seedRole(t, e, txm, casbin.GroupSubject(devs), string(domain.RoleAdmin), "prod")
				removeRole(t, e, txm, alice, casbin.GroupSubject(devs), domain.MembershipDomain)

				return e
			},
			requests: []request{
				{
					domain: "prod",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   false,
				},
			},
		},
		{
			name: "direct user role coexists with group-derived role",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e, txm := newTestEnforcerWithTxM(t, nil)
				seedRoleTemplates(t, e, txm)
				seedRole(t, e, txm, alice, string(domain.RoleReader), domain.DomainAll)
				seedRole(t, e, txm, alice, casbin.GroupSubject(devs), domain.MembershipDomain)
				seedRole(t, e, txm, casbin.GroupSubject(devs), string(domain.RoleAdmin), "prod")

				return e
			},
			requests: []request{
				// Direct reader role grants read in any domain.
				{
					domain: "staging",
					object: domain.ObjectNamespace,
					action: domain.ActionRead,
					want:   true,
				},
				// Group-derived admin role grants write in prod only.
				{
					domain: "prod",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   true,
				},
				// Neither path grants write in staging.
				{
					domain: "staging",
					object: domain.ObjectNamespace,
					action: domain.ActionWrite,
					want:   false,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := tc.setupFunc(t)

			for _, req := range tc.requests {
				got, err := e.Enforce(alice, req.domain, string(req.object), string(req.action))
				require.NoError(t, err)
				assert.Equalf(t, req.want, got,
					"Enforce(%q, %q, %q, %q)", alice, req.domain, req.object, req.action)
			}

			for _, chk := range tc.extraSubjects {
				got, err := e.Enforce(
					chk.subject,
					chk.domain,
					string(chk.object),
					string(chk.action),
				)
				require.NoError(t, err)
				assert.Equalf(t, chk.want, got,
					"Enforce(%q, %q, %q, %q)", chk.subject, chk.domain, chk.object, chk.action)
			}
		})
	}
}
