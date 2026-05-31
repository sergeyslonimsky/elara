package sessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	sessions_mock "github.com/sergeyslonimsky/elara/internal/service/auth/sessions/mocks"
)

func TestService_Validate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	sessionID := "session-abc"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		errIs    error
		wantSess bool
	}{
		{
			name: "active non-expired session returns session",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					ExpiresAt:  now.Add(time.Hour),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				return New(repo, nil, clk)
			},
			wantSess: true,
		},
		{
			name: "repo Get returns ErrSessionNotFound propagated",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(nil, domain.ErrSessionNotFound)

				return New(repo, nil, clk)
			},
			errIs: domain.ErrSessionNotFound,
		},
		{
			name: "revoked session returns ErrSessionRevoked",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				sess := &domain.Session{
					ID:        sessionID,
					UserID:    "user-1",
					ExpiresAt: now.Add(time.Hour),
					RevokedAt: new(now.Add(-time.Hour)),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				return New(repo, nil, clk)
			},
			errIs: domain.ErrSessionRevoked,
		},
		{
			name: "expired session returns ErrSessionExpired",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				sess := &domain.Session{
					ID:        sessionID,
					UserID:    "user-1",
					ExpiresAt: now.Add(-time.Minute), // already expired
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				return New(repo, nil, clk)
			},
			errIs: domain.ErrSessionExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.Validate(t.Context(), sessionID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
			if tt.wantSess {
				assert.NotNil(t, got)
				assert.Equal(t, sessionID, got.ID)
			}
		})
	}
}
