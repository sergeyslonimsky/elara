package authctx_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestWithSession_And_Accessors(t *testing.T) {
	t.Parallel()

	sess := &domain.Session{ID: "sess-1", UserID: "user-1", CreatedAt: time.Now()}
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", DisplayName: "Alice"}

	ctx := auth2.WithSession(t.Context(), sess, user)

	gotSess, ok := auth2.SessionFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, sess, gotSess)

	gotUser, ok := auth2.UserFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, user, gotUser)
}

func TestSessionFromContext_Missing(t *testing.T) {
	t.Parallel()

	sess, ok := auth2.SessionFromContext(t.Context())

	assert.False(t, ok)
	assert.Nil(t, sess)
}

func TestUserFromContext_Missing(t *testing.T) {
	t.Parallel()

	user, ok := auth2.UserFromContext(t.Context())

	assert.False(t, ok)
	assert.Nil(t, user)
}

func TestAuthInfoFromContext(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	t.Run("from session user", func(t *testing.T) {
		t.Parallel()

		user := &domain.User{ID: userID, Email: "alice@example.com", DisplayName: "Alice"}
		ctx := auth2.WithSession(t.Context(), &domain.Session{ID: "sess-1"}, user)

		got, err := auth2.AuthInfoFromContext(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.AuthInfo{
			UserID: userID.String(),
			Email:  "alice@example.com",
			Name:   "Alice",
		}, got)
	})

	t.Run("from claims fallback", func(t *testing.T) {
		t.Parallel()

		claims := &auth2.Claims{
			Email:      "bob@example.com",
			Name:       "Bob",
			Namespaces: []string{"ns1"},
			Role:       "reader",
		}
		ctx := auth2.WithClaims(t.Context(), claims)

		got, err := auth2.AuthInfoFromContext(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.AuthInfo{
			Email:      "bob@example.com",
			Name:       "Bob",
			Namespaces: []string{"ns1"},
			Role:       "reader",
		}, got)
	})

	t.Run("no session and no claims returns unauthorized", func(t *testing.T) {
		t.Parallel()

		got, err := auth2.AuthInfoFromContext(t.Context())

		require.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Equal(t, domain.AuthInfo{}, got)
	})

	t.Run("session user takes priority over claims", func(t *testing.T) {
		t.Parallel()

		user := &domain.User{ID: userID, Email: "alice@example.com", DisplayName: "Alice"}
		ctx := auth2.WithSession(t.Context(), &domain.Session{ID: "sess-1"}, user)
		ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "bob@example.com", Name: "Bob"})

		got, err := auth2.AuthInfoFromContext(ctx)

		require.NoError(t, err)
		assert.Equal(t, domain.AuthInfo{
			UserID: userID.String(),
			Email:  "alice@example.com",
			Name:   "Alice",
		}, got)
	})
}
