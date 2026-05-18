package group_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// Authorization for GroupService.* is enforced by the RBAC interceptor;
// these tests cover only the business logic remaining in the usecase.

func contextWithAdmin(ctx context.Context) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		groupName string
		wantErr   string
	}{
		{name: "success", groupName: "test-group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, _, _, _ := newTestService(t)

			got, err := sut.Create(t.Context(), tt.groupName)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.groupName, got.Name)
			assert.NotEmpty(t, got.ID)
		})
	}
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := newTestService(t)

		created, err := sut.Create(t.Context(), "g1")
		require.NoError(t, err)

		got, err := sut.Get(t.Context(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := newTestService(t)

		_, err := sut.Get(t.Context(), "missing-id")
		require.ErrorContains(t, err, "get group")
	})
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	ctx := contextWithAdmin(context.Background())

	t.Run("name unchanged -> store update only, no Casbin mutation", func(t *testing.T) {
		t.Parallel()

		sut, _, enforcer, _ := newTestService(t)

		created, err := sut.Create(ctx, "old-name")
		require.NoError(t, err)

		before := enforcer.GetGroupingPolicy()
		updated, err := sut.Update(ctx, created.ID, "old-name", "desc", nil, nil, created.Version)
		require.NoError(t, err)
		assert.Equal(t, "old-name", updated.Name)
		assert.Equal(t, "desc", updated.Description)

		assert.Equal(t, before, enforcer.GetGroupingPolicy(),
			"unchanged name must not mutate Casbin")
	})

	t.Run("rename rewrites memberships under the new prefix", func(t *testing.T) {
		t.Parallel()

		sut, _, enforcer, _ := newTestService(t)

		created, err := sut.Create(ctx, "old-name")
		require.NoError(t, err)

		_, err = sut.Update(ctx, created.ID, "old-name", "", nil, []string{"user1@example.com"}, created.Version)
		require.NoError(t, err)

		oldSub := domain.GroupSubject("old-name")
		newSub := domain.GroupSubject("new-name")

		require.NotEmpty(t, enforcer.GetMembersOfGroup(oldSub),
			"precondition: membership rule exists under old name")

		updated, err := sut.Update(
			ctx,
			created.ID,
			"new-name",
			"",
			nil,
			[]string{"user1@example.com"},
			created.Version+1,
		)
		require.NoError(t, err)
		assert.Equal(t, "new-name", updated.Name)

		assert.Empty(t, enforcer.GetMembersOfGroup(oldSub),
			"old subject must have no remaining members")
		assert.Contains(t, enforcer.GetMembersOfGroup(newSub), "user1@example.com",
			"new subject must inherit memberships")
	})

	t.Run("group not found", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := newTestService(t)

		_, err := sut.Update(ctx, "missing-id", "any-name", "", nil, nil, 0)
		require.ErrorContains(t, err, "get group")
	})
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	ctx := contextWithAdmin(context.Background())

	t.Run("delete removes group and wipes Casbin membership rules in one tx", func(t *testing.T) {
		t.Parallel()

		sut, _, enforcer, _ := newTestService(t)

		created, err := sut.Create(ctx, "devops")
		require.NoError(t, err)

		_, err = sut.Update(ctx, created.ID, "devops", "", nil, []string{"user1@example.com"}, created.Version)
		require.NoError(t, err)

		groupSub := domain.GroupSubject("devops")
		require.NotEmpty(t, enforcer.GetMembersOfGroup(groupSub),
			"precondition: group has members")

		require.NoError(t, sut.Delete(ctx, created.ID))

		_, err = sut.Get(ctx, created.ID)
		require.ErrorContains(t, err, "get group")

		// DeleteUser in the cache only removes col-0 rules; col-1 rules are
		// removed from persistence but linger in the cache until the next
		// LoadPolicy. Resync to assert persistence-side correctness.
		require.NoError(t, enforcer.LoadPolicy())
		assert.Empty(t, enforcer.GetMembersOfGroup(groupSub),
			"membership rules must be wiped after delete (post-LoadPolicy resync)")
	})

	t.Run("group not found", func(t *testing.T) {
		t.Parallel()

		sut, _, _, _ := newTestService(t)

		err := sut.Delete(ctx, "missing-id")
		require.ErrorContains(t, err, "get group")
	})
}

func TestService_List(t *testing.T) {
	t.Parallel()

	sut, _, _, _ := newTestService(t)

	_, err := sut.Create(t.Context(), "g1")
	require.NoError(t, err)
	_, err = sut.Create(t.Context(), "g2")
	require.NoError(t, err)

	got, err := sut.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}
