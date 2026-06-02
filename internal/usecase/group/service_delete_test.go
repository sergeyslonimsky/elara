package group_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

func TestService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, st testStack) string
		assert  func(t *testing.T, st testStack)
		wantErr string
	}{
		{
			name: "delete removes group and wipes Casbin membership rules in one tx",
			setup: func(t *testing.T, st testStack) string {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "devops"},
				)
				require.NoError(t, err)

				_, err = st.svc.UpdateMembers(t.Context(), adminAuth(), group.UpdateMembersData{
					GroupName: created.Group.Name,
					AddEmails: []string{"user1@example.com"},
				})
				require.NoError(t, err)

				require.NotEmpty(t, st.enforcer.GetMembersOfGroup(domain.GroupResource("devops")),
					"precondition: group has members")

				return created.Group.Name
			},
			assert: func(t *testing.T, st testStack) {
				t.Helper()

				// DeleteUser in the cache only removes col-0 rules; col-1 rules are
				// removed from persistence but linger in the cache until the next
				// LoadPolicy. Resync to assert persistence-side correctness.
				require.NoError(t, st.enforcer.LoadPolicy())
				assert.Empty(t, st.enforcer.GetMembersOfGroup(domain.GroupResource("devops")),
					"membership rules must be wiped after delete (post-LoadPolicy resync)")
			},
		},
		{
			name: "group not found",
			setup: func(_ *testing.T, _ testStack) string {
				return "missing-id"
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			id := tt.setup(t, st)

			err := st.svc.Delete(t.Context(), adminAuth(), id)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)

			_, err = st.svc.Get(t.Context(), adminAuth(), id)
			require.ErrorContains(t, err, "get group")

			if tt.assert != nil {
				tt.assert(t, st)
			}
		})
	}
}
