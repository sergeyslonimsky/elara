package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestSession_IsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		revokedAt *time.Time
		want      bool
	}{
		{
			name:      "nil RevokedAt is active",
			revokedAt: nil,
			want:      true,
		},
		{
			name:      "non-nil RevokedAt is not active",
			revokedAt: new(time.Now()),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := domain.Session{RevokedAt: tt.revokedAt}
			assert.Equal(t, tt.want, s.IsActive())
		})
	}
}

func TestSession_IsExpired(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "ExpiresAt in the future is not expired",
			expiresAt: now.Add(time.Hour),
			now:       now,
			want:      false,
		},
		{
			name:      "ExpiresAt in the past is expired",
			expiresAt: now.Add(-time.Hour),
			now:       now,
			want:      true,
		},
		{
			name:      "ExpiresAt equal to now is not expired",
			expiresAt: now,
			now:       now,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := domain.Session{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, s.IsExpired(tt.now))
		})
	}
}

func TestSession_EnsureActive(t *testing.T) {
	t.Parallel()

	now := time.Now()
	revokedAt := now.Add(-time.Minute)

	tests := []struct {
		name    string
		session domain.Session
		now     time.Time
		errIs   error
	}{
		{
			name: "active and not expired returns nil",
			session: domain.Session{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: nil,
			},
			now:   now,
			errIs: nil,
		},
		{
			name: "revoked returns ErrSessionRevoked",
			session: domain.Session{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: &revokedAt,
			},
			now:   now,
			errIs: domain.ErrSessionRevoked,
		},
		{
			name: "active but expired returns ErrSessionExpired",
			session: domain.Session{
				ExpiresAt: now.Add(-time.Hour),
				RevokedAt: nil,
			},
			now:   now,
			errIs: domain.ErrSessionExpired,
		},
		{
			name: "revoked and expired returns ErrSessionRevoked (revoked wins)",
			session: domain.Session{
				ExpiresAt: now.Add(-time.Hour),
				RevokedAt: &revokedAt,
			},
			now:   now,
			errIs: domain.ErrSessionRevoked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.session.EnsureActive(tt.now)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewSessionID(t *testing.T) {
	t.Parallel()

	t.Run("produces non-empty base64 URL output", func(t *testing.T) {
		t.Parallel()

		id, err := domain.NewSessionID()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("length is deterministic 44 chars", func(t *testing.T) {
		t.Parallel()

		id, err := domain.NewSessionID()
		require.NoError(t, err)
		// 32 random bytes → base64 URLEncoding with padding = ceil(32/3)*4 = 44 chars
		assert.Len(t, id, 44)
	})

	t.Run("uniqueness across 1000 calls", func(t *testing.T) {
		t.Parallel()

		seen := make(map[string]struct{}, 1000)
		for range 1000 {
			id, err := domain.NewSessionID()
			require.NoError(t, err)
			seen[id] = struct{}{}
		}
		assert.Len(t, seen, 1000)
	})
}
