package policy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/policy"
)

// Authorization for policy.{AssignRole,RevokeRole,List} is enforced by the
// RBAC interceptor; these tests cover only the business logic that remains
// in the usecase. The Casbin enforcer is exercised end-to-end against a real
// bbolt-backed PolicyRepo so we lock in the canonical g-rule layout
// (group: prefix, single write, no per-member fan-out).

func TestService_AssignRole(t *testing.T) {
	t.Parallel()

	const (
		groupName = "devops"
		dom       = "prod"
		role      = "admin"
	)

	tests := []struct {
		name      string
		setupMock func(ctx context.Context, m *mocks)
		// verify lets the success case assert the persisted g-rule layout.
		verify  func(t *testing.T, e *casbin.Enforcer)
		errIs   error
		wantErr string
	}{
		{
			name: "writes a single g-rule with the group: prefix; no per-member fan-out",
			setupMock: func(ctx context.Context, m *mocks) {
				m.groups.EXPECT().FindByName(ctx, groupName).Return(&domain.Group{Name: groupName}, nil)
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
				t.Helper()

				want := []string{domain.GroupSubject(groupName), role, dom}

				rules := e.GetGroupingPolicy()
				found := 0
				for _, r := range rules {
					if len(r) == 3 && r[0] == want[0] && r[1] == want[1] && r[2] == want[2] {
						found++
					}
				}

				assert.Equal(t, 1, found, "expected exactly one g-rule %v, got rules=%v", want, rules)
			},
		},
		{
			name: "non-existent group -> ErrNotFound, no Casbin mutation",
			setupMock: func(ctx context.Context, m *mocks) {
				m.groups.EXPECT().
					FindByName(ctx, groupName).
					Return(nil, domain.NewNotFoundError("group", groupName))
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
				t.Helper()
				// No g-rule for this group should exist after the failed call.
				for _, r := range e.GetGroupingPolicy() {
					if len(r) == 3 && r[0] == domain.GroupSubject(groupName) {
						t.Errorf("unexpected g-rule for group: %v", r)
					}
				}
			},
			errIs: domain.ErrNotFound,
		},
		{
			name: "FindByName unexpected error -> wrapped",
			setupMock: func(ctx context.Context, m *mocks) {
				m.groups.EXPECT().FindByName(ctx, groupName).Return(nil, errors.New("db error"))
			},
			wantErr: "find group by name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, e, _, m := setupService(t)
			ctx := t.Context()
			tt.setupMock(ctx, m)

			err := sut.AssignRole(ctx, groupName, dom, role)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				if tt.verify != nil {
					tt.verify(t, e)
				}

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, e)
			}
		})
	}
}

func TestService_RevokeRole(t *testing.T) {
	t.Parallel()

	const (
		groupName = "devops"
		dom       = "prod"
		role      = "admin"
	)

	tests := []struct {
		name      string
		preSeed   bool // if true, seed the g-rule before invoking RevokeRole.
		setupMock func(ctx context.Context, m *mocks)
		errIs     error
		wantErr   string
	}{
		{
			name:    "single RemoveRoleForUser with group: prefix; all members lose access via casbin recursion",
			preSeed: true,
			setupMock: func(ctx context.Context, m *mocks) {
				m.groups.EXPECT().FindByName(ctx, groupName).Return(&domain.Group{Name: groupName}, nil)
			},
		},
		{
			name: "non-existent group -> ErrNotFound",
			setupMock: func(ctx context.Context, m *mocks) {
				m.groups.EXPECT().
					FindByName(ctx, groupName).
					Return(nil, domain.NewNotFoundError("group", groupName))
			},
			errIs: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, e, txm, m := setupService(t)
			ctx := t.Context()

			if tt.preSeed {
				require.NoError(t, e.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddRoleForUser(domain.GroupSubject(groupName), role, dom)
				}))
			}

			tt.setupMock(ctx, m)

			err := sut.RevokeRole(ctx, groupName, dom, role)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			// After successful revoke the g-rule must be gone.
			want := []string{domain.GroupSubject(groupName), role, dom}
			for _, r := range e.GetGroupingPolicy() {
				if len(r) == 3 && r[0] == want[0] && r[1] == want[1] && r[2] == want[2] {
					t.Errorf("g-rule %v should be removed", want)
				}
			}
		})
	}
}

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// seed g-rules added through WriteTx prior to invoking List.
		seedG [][]string
		want  []policy.Rule
	}{
		{
			name: "only group->role rules are surfaced; memberships and direct user grants are filtered",
			seedG: [][]string{
				// Direct user grant (plain email subject) — filtered.
				{"admin@example.com", "admin", "*"},
				// Canonical group->role rule.
				{domain.GroupSubject("devops"), "admin", "prod"},
				// Membership rule — column 0 is a plain user, filtered.
				{"alice@example.com", domain.GroupSubject("devops"), "*"},
			},
			want: []policy.Rule{
				{Subject: "devops", Role: "admin", Domain: "prod"},
			},
		},
		{
			name:  "empty grouping policy -> empty result",
			seedG: nil,
			want:  []policy.Rule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, e, txm, _ := setupService(t)
			ctx := t.Context()

			for _, g := range tt.seedG {
				rule := g
				require.NoError(t, e.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddRoleForUser(rule[0], rule[1], rule[2])
				}))
			}

			got, err := sut.List(ctx)
			require.NoError(t, err)

			// Compare contents irrespective of slice order — List does not
			// promise stable ordering and the seed order is incidental.
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
