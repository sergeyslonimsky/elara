package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforcer_GetImplicitPermissionsForUser(t *testing.T) {
	t.Parallel()

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRoleTemplates(t, e, txm)
	seedRole(t, e, txm, "alice", "writer", "*")

	perms, err := e.GetImplicitPermissionsForUser("alice")
	require.NoError(t, err)

	found := false
	for _, p := range perms {
		if len(p) >= 4 && p[2] == "config" && p[3] == "write" {
			found = true

			break
		}
	}
	assert.True(t, found, "expected implicit config/write permission for alice via writer role")
}

func TestEnforcer_GetImplicitPermissionsForUser_NoPermissions(t *testing.T) {
	t.Parallel()

	e, _ := newTestEnforcerWithTxM(t, nil)

	perms, err := e.GetImplicitPermissionsForUser("nobody")
	require.NoError(t, err)
	assert.Empty(t, perms)
}

func TestEnforcer_GetMembersOfGroup(t *testing.T) {
	t.Parallel()

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "alice", "group:devs", "*")
	seedRole(t, e, txm, "bob", "group:devs", "*")
	seedRole(t, e, txm, "carol", "group:ops", "*")

	members := e.GetMembersOfGroup("group:devs")

	assert.ElementsMatch(t, []string{"alice", "bob"}, members)
}

func TestEnforcer_GetMembersOfGroup_Empty(t *testing.T) {
	t.Parallel()

	e, _ := newTestEnforcerWithTxM(t, nil)

	members := e.GetMembersOfGroup("group:nonexistent")

	assert.Empty(t, members)
}
