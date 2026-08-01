package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

func TestPAPTx_GroupPermissions(t *testing.T) {
	t.Parallel()

	t.Run("reads in-tx state after mutation in same write", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		var got []domain.Permission

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			if err := w.ApplyPermissionDeltas(
				"devs",
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
				nil,
			); err != nil {
				return err
			}

			var err error
			got, err = w.GroupPermissions("devs")

			return err
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.Permission{
			{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"},
		}, got)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			_, err := w.GroupPermissions("devs")

			return err
		})

		require.ErrorContains(t, err, "get group permissions:")
	})
}

func TestPAPTx_ApplyPermissionDeltas(t *testing.T) {
	t.Parallel()

	t.Run("success adds and removes", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyPermissionDeltas(
				"devs",
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
				nil,
			)
		}))

		assert.Equal(t, []domain.Permission{
			{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"},
		}, pap.GroupPermissions("devs"))

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyPermissionDeltas(
				"devs",
				nil,
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
			)
		}))

		assert.Empty(t, pap.GroupPermissions("devs"))
	})

	t.Run("add error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyPermissionDeltas(
				"devs",
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
				nil,
			)
		})

		require.ErrorContains(t, err, "add policy:")
	})

	t.Run("remove error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyPermissionDeltas(
				"devs",
				nil,
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
			)
		})

		require.ErrorContains(t, err, "remove policy:")
	})
}

func TestPAPTx_ApplyMemberDeltas(t *testing.T) {
	t.Parallel()

	t.Run("success adds and removes members", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyMemberDeltas("devs", []string{"alice", "bob"}, nil)
		}))
		assert.ElementsMatch(t, []string{"alice", "bob"}, pap.GroupMembers("devs"))

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyMemberDeltas("devs", nil, []string{"alice"})
		}))
		assert.Equal(t, []string{"bob"}, pap.GroupMembers("devs"))
	})

	t.Run("add error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyMemberDeltas("devs", []string{"alice"}, nil)
		})

		require.ErrorContains(t, err, "add membership:")
	})

	t.Run("remove error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyMemberDeltas("devs", nil, []string{"alice"})
		})

		require.ErrorContains(t, err, "remove membership:")
	})
}

func TestPAPTx_ApplyUserMembershipDeltas(t *testing.T) {
	t.Parallel()

	t.Run("success adds and removes group memberships", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyUserMembershipDeltas("alice", []string{"devs", "ops"}, nil)
		}))
		names, err := pap.UserGroupNames("alice")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"devs", "ops"}, names)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyUserMembershipDeltas("alice", nil, []string{"ops"})
		}))
		names, err = pap.UserGroupNames("alice")
		require.NoError(t, err)
		assert.Equal(t, []string{"devs"}, names)
	})

	t.Run("add error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyUserMembershipDeltas("alice", []string{"devs"}, nil)
		})

		require.ErrorContains(t, err, "add membership devs:")
	})

	t.Run("remove error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyUserMembershipDeltas("alice", nil, []string{"devs"})
		})

		require.ErrorContains(t, err, "remove membership devs:")
	})
}

func TestPAPTx_DeleteGroup(t *testing.T) {
	t.Parallel()

	t.Run("success removes group's permissions", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyPermissionDeltas(
				"devs",
				[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"}},
				nil,
			)
		}))
		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.DeleteGroup("devs")
		}))

		assert.Empty(t, pap.GroupPermissions("devs"))
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.DeleteGroup("devs")
		})

		require.ErrorContains(t, err, "delete group:")
	})
}

func TestPAPTx_DeleteUser(t *testing.T) {
	t.Parallel()

	t.Run("success removes user's rules", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyUserMembershipDeltas("alice", []string{"devs"}, nil)
		}))
		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.DeleteUser("alice")
		}))

		names, err := pap.UserGroupNames("alice")
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		t.Parallel()

		pap, _, db := newTestPAP(t)
		breakPolicyBucket(t, db)

		err := pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.DeleteUser("alice")
		})

		require.ErrorContains(t, err, "delete user:")
	})
}
