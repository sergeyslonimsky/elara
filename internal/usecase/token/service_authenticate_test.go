package token_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestService_Authenticate(t *testing.T) {
	t.Parallel()

	const (
		username = "ci-abcdef"
		raw      = "elara_correct-secret"
	)

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name     string
		username string
		password string
		mockFunc func(m mocks)
		errIs    error
		wantErr  string
		want     *domain.Token
	}{
		{
			name:     "success",
			username: username,
			password: raw,
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByUsername(gomock.Any(), username).Return(&domain.Token{
					ID:        "t1",
					Username:  username,
					TokenHash: token.HashRawToken(raw),
					ExpiresAt: &future,
				}, nil)
			},
			want: &domain.Token{
				ID:        "t1",
				Username:  username,
				TokenHash: token.HashRawToken(raw),
				ExpiresAt: &future,
			},
		},
		{
			name:     "success without expiry",
			username: username,
			password: raw,
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByUsername(gomock.Any(), username).Return(&domain.Token{
					ID:        "t1",
					Username:  username,
					TokenHash: token.HashRawToken(raw),
				}, nil)
			},
			want: &domain.Token{
				ID:        "t1",
				Username:  username,
				TokenHash: token.HashRawToken(raw),
			},
		},
		{
			name:     "unknown username",
			username: "nobody-aaaaaa",
			password: raw,
			mockFunc: func(m mocks) {
				m.store.EXPECT().
					GetByUsername(gomock.Any(), "nobody-aaaaaa").
					Return(nil, fmt.Errorf("token username: %w", storage.ErrResourceNotFound))
			},
			errIs: domain.ErrInvalidToken,
		},
		{
			name:     "store failure still reads as invalid token",
			username: username,
			password: raw,
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByUsername(gomock.Any(), username).Return(nil, errors.New("db error"))
			},
			wantErr: `lookup "ci-abcdef": invalid token (db error)`,
		},
		{
			name:     "wrong secret",
			username: username,
			password: "elara_wrong-secret",
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByUsername(gomock.Any(), username).Return(&domain.Token{
					ID:        "t1",
					Username:  username,
					TokenHash: token.HashRawToken(raw),
				}, nil)
			},
			wantErr: "token t1: secret mismatch: invalid token",
		},
		{
			name:     "expired token",
			username: username,
			password: raw,
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByUsername(gomock.Any(), username).Return(&domain.Token{
					ID:        "t1",
					Username:  username,
					TokenHash: token.HashRawToken(raw),
					ExpiresAt: &past,
				}, nil)
			},
			wantErr: "token t1: expired: invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			tt.mockFunc(m)

			got, err := svc.Authenticate(t.Context(), tt.username, tt.password)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				assert.Nil(t, got)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, domain.ErrInvalidToken, "every failure must wrap ErrInvalidToken")
				assert.Nil(t, got)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
