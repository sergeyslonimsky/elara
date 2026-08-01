package interceptor

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestAuthInterceptor_CheckPasswordChangeRequired(t *testing.T) {
	t.Parallel()

	i := &AuthInterceptor{}

	t.Run("no user in context returns nil", func(t *testing.T) {
		t.Parallel()

		err := i.checkPasswordChangeRequired(t.Context(), "/some.Service/Method")
		require.NoError(t, err)
	})

	t.Run("user without password change requirement returns nil", func(t *testing.T) {
		t.Parallel()

		user := &domain.User{ID: uuid.New(), Email: "u@example.com", PasswordChangeRequired: false}
		ctx := authctx.WithSession(t.Context(), &domain.Session{ID: "s1"}, user)

		err := i.checkPasswordChangeRequired(ctx, "/some.Service/Method")
		require.NoError(t, err)
	})

	t.Run("password change required blocks disallowed procedure", func(t *testing.T) {
		t.Parallel()

		user := &domain.User{ID: uuid.New(), Email: "u@example.com", PasswordChangeRequired: true}
		ctx := authctx.WithSession(t.Context(), &domain.Session{ID: "s1"}, user)

		err := i.checkPasswordChangeRequired(ctx, "/some.Service/Method")
		require.ErrorIs(t, err, domain.ErrPasswordChangeRequired)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
	})

	t.Run("password change required allows whitelisted procedure", func(t *testing.T) {
		t.Parallel()

		user := &domain.User{ID: uuid.New(), Email: "u@example.com", PasswordChangeRequired: true}
		ctx := authctx.WithSession(t.Context(), &domain.Session{ID: "s1"}, user)

		err := i.checkPasswordChangeRequired(ctx, "/elara.profile.v1.ProfileService/ChangePassword")
		require.NoError(t, err)
	})
}

func TestAuthInterceptor_InjectBypassUser(t *testing.T) {
	t.Parallel()

	i := &AuthInterceptor{}
	ctx := i.injectBypassUser(t.Context())

	user, ok := authctx.UserFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, uuid.Nil, user.ID)
	assert.Equal(t, domain.UserStatusActive, user.Status)

	sess, ok := authctx.SessionFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, "bypass-session", sess.ID)
}

func TestAuthInterceptor_RejectOrBypass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		skipPermissions bool
		wantErr         bool
	}{
		{
			name:            "skip permissions injects bypass user and clears error",
			skipPermissions: true,
		},
		{
			name:            "no skip permissions propagates the error",
			skipPermissions: false,
			wantErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			i := &AuthInterceptor{skipPermissions: tc.skipPermissions}
			inErr := connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)

			ctx, err := i.rejectOrBypass(t.Context(), inErr)

			if tc.wantErr {
				require.ErrorIs(t, err, inErr)
				_, ok := authctx.UserFromContext(ctx)
				assert.False(t, ok)

				return
			}

			require.NoError(t, err)
			user, ok := authctx.UserFromContext(ctx)
			require.True(t, ok)
			assert.Equal(t, uuid.Nil, user.ID)
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bearer string
		cookie string
		want   string
	}{
		{
			name: "no headers returns empty",
			want: "",
		},
		{
			name:   "bearer header without prefix is ignored",
			bearer: "not-a-bearer-token",
			want:   "",
		},
		{
			name:   "bearer header with empty token after prefix is ignored",
			bearer: "Bearer ",
			want:   "",
		},
		{
			name:   "cookie is used when no bearer header",
			cookie: "cookie-session-id",
			want:   "cookie-session-id",
		},
		{
			name:   "bearer wins over cookie",
			bearer: "Bearer bearer-id",
			cookie: "cookie-session-id",
			want:   "bearer-id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			header := make(http.Header)
			if tc.bearer != "" {
				header.Set("Authorization", tc.bearer)
			}
			if tc.cookie != "" {
				header.Set("Cookie", sessionCookieName+"="+tc.cookie)
			}

			got := extractSessionID(header)
			assert.Equal(t, tc.want, got)
		})
	}
}
