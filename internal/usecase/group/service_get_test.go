package group_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// TestService_Get covers the composed read view: bbolt entity + filtered
// members + full permission set, plus the not-found wrapping.
func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, st testStack) (domain.AuthInfo, string)
		assert  func(t *testing.T, got *group.GetResult)
		wantErr string
	}{
		{
			name: "wildcard admin sees all members and permissions",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, string) {
				t.Helper()

				seedAdminWildcard(t, st)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com", "bob@example.com"},
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
				})
				require.NoError(t, err)

				return adminAuth(), created.Group.ID
			},
			assert: func(t *testing.T, got *group.GetResult) {
				t.Helper()

				assert.Equal(t, "g1", got.Group.Name)
				assert.ElementsMatch(
					t,
					[]string{"alice@example.com", "bob@example.com"},
					got.VisibleMembers,
				)
				assert.ElementsMatch(t, []domain.Permission{
					{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
				}, got.Permissions)
			},
		},
		{
			// VisibleMembers is the derived User:Read filter (authz.Scope).
			// In Get's call site every member is — by definition — a member
			// of the group being read; once the caller holds Group:Read on
			// that group (which is what makes Get callable in the first
			// place), every member becomes visible via that same group's
			// readability. The case below pins down the symmetric case for
			// a caller WITHOUT User:Read * and WITHOUT Group:Read on the
			// queried group's id: scope returns an empty list, but Get
			// itself does not enforce read authorization (that's the handler
			// layer), so we observe the filtered-empty membership.
			name: "no read scope: members hidden",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, string) {
				t.Helper()

				seedAdminWildcard(t, st)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "hidden",
					InitialMembers: []string{"alice@example.com", "bob@example.com"},
				})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "nobody@example.com"}, created.Group.ID
			},
			assert: func(t *testing.T, got *group.GetResult) {
				t.Helper()

				assert.Equal(t, "hidden", got.Group.Name)
				assert.Empty(
					t,
					got.VisibleMembers,
					"caller without read scope sees no members through the derived User:Read filter",
				)
			},
		},
		{
			name: "permissions field reflects current p-rules on the subject",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, string) {
				t.Helper()

				seedAdminWildcard(t, st)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "with-perms",
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
				})
				require.NoError(t, err)

				return adminAuth(), created.Group.ID
			},
			assert: func(t *testing.T, got *group.GetResult) {
				t.Helper()

				assert.ElementsMatch(t, []domain.Permission{
					{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
				}, got.Permissions)
			},
		},
		{
			name: "not found wrapped",
			setup: func(_ *testing.T, _ testStack) (domain.AuthInfo, string) {
				return adminAuth(), "missing-id"
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, id := tt.setup(t, st)

			got, err := st.svc.Get(t.Context(), auth, id)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			if tt.assert != nil {
				tt.assert(t, got)
			}
		})
	}
}
