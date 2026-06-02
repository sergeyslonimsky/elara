package group_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams)
		assert  func(t *testing.T, got *group.ListResult)
		wantErr string
	}{
		{
			name: "wildcard scope returns all",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams) {
				t.Helper()

				seedAdminWildcard(t, st)

				for _, n := range []string{"alpha", "beta", "gamma"} {
					_, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: n})
					require.NoError(t, err)
				}

				return adminAuth(), group.ListParams{}
			},
			assert: func(t *testing.T, got *group.ListResult) {
				t.Helper()

				assert.Equal(t, 3, got.Total)
				require.Len(t, got.Groups, 3)
				assert.Equal(t, []string{"alpha", "beta", "gamma"}, []string{
					got.Groups[0].Name, got.Groups[1].Name, got.Groups[2].Name,
				})
			},
		},
		{
			name: "explicit scope returns only readable groups",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams) {
				t.Helper()

				for _, n := range []string{"dev", "prod"} {
					_, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: n})
					require.NoError(t, err)
				}

				const delegatedID = "delegated-id"
				require.NoError(
					t,
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								delegatedID,
								domain.GroupResource("dev"),
								string(domain.ObjectGroup),
								string(domain.ActionRead),
							)
						},
					),
				)

				return domain.AuthInfo{UserID: delegatedID, Email: "delegated@example.com"}, group.ListParams{}
			},
			assert: func(t *testing.T, got *group.ListResult) {
				t.Helper()

				assert.Equal(t, 1, got.Total)
				require.Len(t, got.Groups, 1)
				assert.Equal(t, "dev", got.Groups[0].Name)
			},
		},
		{
			name: "empty scope returns empty list with default limit",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams) {
				t.Helper()

				for _, n := range []string{"dev", "prod"} {
					_, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: n})
					require.NoError(t, err)
				}

				return domain.AuthInfo{Email: "nobody@example.com"}, group.ListParams{}
			},
			assert: func(t *testing.T, got *group.ListResult) {
				t.Helper()

				assert.Equal(t, 0, got.Total)
				assert.Empty(t, got.Groups)
				assert.Equal(t, 20, got.Limit)
			},
		},
		{
			name: "default limit is 20",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams) {
				t.Helper()

				seedAdminWildcard(t, st)

				return adminAuth(), group.ListParams{Limit: 0}
			},
			assert: func(t *testing.T, got *group.ListResult) {
				t.Helper()

				assert.Equal(t, 20, got.Limit)
			},
		},
		{
			name: "pagination forwarded",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.ListParams) {
				t.Helper()

				seedAdminWildcard(t, st)

				for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
					_, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: n})
					require.NoError(t, err)
				}

				return adminAuth(), group.ListParams{Limit: 2, Offset: 2}
			},
			assert: func(t *testing.T, got *group.ListResult) {
				t.Helper()

				assert.Equal(t, 6, got.Total)
				assert.Equal(t, 2, got.Limit)
				assert.Equal(t, 2, got.Offset)
				require.Len(t, got.Groups, 2)
				assert.Equal(
					t,
					[]string{"c", "d"},
					[]string{got.Groups[0].Name, got.Groups[1].Name},
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, params := tt.setup(t, st)

			got, err := st.svc.List(t.Context(), auth, params)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.assert != nil {
				tt.assert(t, got)
			}
		})
	}
}
