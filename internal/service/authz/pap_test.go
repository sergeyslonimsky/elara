package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

func TestPAP_Write(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fn      func(ctx context.Context, w *authz.PAPTx) error
		wantErr string
	}{
		{
			name: "success",
			fn: func(_ context.Context, w *authz.PAPTx) error {
				return w.ApplyPermissionDeltas(
					"grp",
					[]domain.Permission{{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "*"}},
					nil,
				)
			},
		},
		{
			name: "callback error is wrapped",
			fn: func(context.Context, *authz.PAPTx) error {
				return errors.New("boom")
			},
			wantErr: "pap write: write tx: bbolt: with tx: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pap, _, _ := newTestPAP(t)

			err := pap.Write(t.Context(), tt.fn)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPAP_GroupNamesFromScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope authz.DomainSet
		want  map[string]struct{}
	}{
		{
			name:  "group subjects extracted",
			scope: authz.NewDomainSet(domain.GroupResource("devs"), domain.GroupResource("ops")),
			want:  map[string]struct{}{"devs": {}, "ops": {}},
		},
		{
			name:  "non-group domains skipped",
			scope: authz.NewDomainSet("namespace:prod", domain.GroupResource("devs")),
			want:  map[string]struct{}{"devs": {}},
		},
		{
			name:  "empty scope",
			scope: authz.NewDomainSet(),
			want:  map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pap, _, _ := newTestPAP(t)

			got := pap.GroupNamesFromScope(tt.scope)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPAP_MembersOfScope(t *testing.T) {
	t.Parallel()

	t.Run("returns members of groups in scope, filters nested groups", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
			return w.ApplyMemberDeltas("devs", []string{"alice", "bob"}, nil)
		}))

		scope := authz.NewDomainSet(domain.GroupResource("devs"))
		got := pap.MembersOfScope(scope)

		assert.Equal(t, map[string]struct{}{"alice": {}, "bob": {}}, got)
	})

	t.Run("non-group domains in scope contribute nothing", func(t *testing.T) {
		t.Parallel()

		pap, _, _ := newTestPAP(t)

		scope := authz.NewDomainSet("namespace:prod")
		got := pap.MembersOfScope(scope)

		assert.Empty(t, got)
	})
}

func TestPAP_GroupPermissions(t *testing.T) {
	t.Parallel()

	pap, _, _ := newTestPAP(t)

	require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
		return w.ApplyPermissionDeltas(
			"devs",
			[]domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"},
			},
			nil,
		)
	}))

	got := pap.GroupPermissions("devs")
	assert.Equal(t, []domain.Permission{
		{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "prod"},
	}, got)

	assert.Empty(t, pap.GroupPermissions("does-not-exist"))
}

func TestPAP_GroupMembers(t *testing.T) {
	t.Parallel()

	pap, _, _ := newTestPAP(t)

	require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
		return w.ApplyMemberDeltas("devs", []string{"alice", "bob"}, nil)
	}))

	got := pap.GroupMembers("devs")
	assert.ElementsMatch(t, []string{"alice", "bob"}, got)
	assert.Empty(t, pap.GroupMembers("does-not-exist"))
}

func TestPAP_UserGroupNames(t *testing.T) {
	t.Parallel()

	pap, _, _ := newTestPAP(t)

	require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
		return w.ApplyUserMembershipDeltas("alice", []string{"devs", "ops"}, nil)
	}))

	got, err := pap.UserGroupNames("alice")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"devs", "ops"}, got)

	got, err = pap.UserGroupNames("nobody")
	require.NoError(t, err)
	assert.Empty(t, got)
}
