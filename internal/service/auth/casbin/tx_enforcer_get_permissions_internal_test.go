package casbin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTxEnforcer_GetPermissionsForSubject(t *testing.T) {
	t.Parallel()

	e, txm, _ := newInternalTestEnforcer(t)

	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ context.Context, txe *TxEnforcer) error {
		return txe.AddPolicy("group:devs", "*", "config", "write")
	}))

	var perms [][]string
	err := e.WriteTx(t.Context(), txm, func(_ context.Context, txe *TxEnforcer) error {
		var err error
		perms, err = txe.GetPermissionsForSubject("group:devs")

		return err
	})
	require.NoError(t, err)

	found := false
	for _, p := range perms {
		if len(p) >= 4 && p[0] == "group:devs" && p[1] == "*" && p[2] == "config" && p[3] == "write" {
			found = true

			break
		}
	}
	assert.True(t, found, "expected permission rule for group:devs")
}

func TestTxEnforcer_GetPermissionsForSubject_NoPermissions(t *testing.T) {
	t.Parallel()

	e, txm, _ := newInternalTestEnforcer(t)

	var perms [][]string
	err := e.WriteTx(t.Context(), txm, func(_ context.Context, txe *TxEnforcer) error {
		var err error
		perms, err = txe.GetPermissionsForSubject("group:nonexistent")

		return err
	})
	require.NoError(t, err)
	assert.Empty(t, perms)
}
