package sessions

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	sessions_mock "github.com/sergeyslonimsky/elara/internal/service/auth/sessions/mocks"
)

func TestService_Revoke(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	sessionID := "session-revoke"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
	}{
		{
			name: "happy path: updates session with RevokedAt and appends event",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					ExpiresAt:  now.Add(time.Hour),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				gomock.InOrder(
					repo.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).
						DoAndReturn(func(_ any, s *domain.Session) error {
							require.NotNil(t, s.RevokedAt)
							assert.Equal(t, now, *s.RevokedAt)
							assert.Equal(t, "admin-user", s.RevokedBy)

							return nil
						}),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).
						DoAndReturn(func(_ any, e *domain.SessionEvent) error {
							assert.Equal(t, domain.SessionEventRevokedByAdmin, e.Type)
							assert.Equal(t, sessionID, e.SessionID)
							assert.Equal(t, "user-1", e.UserID)
							assert.Equal(t, "security policy", e.Reason)
							assert.Equal(t, now, e.Timestamp)

							return nil
						}),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "repo Get returns error propagated",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(nil, domain.ErrSessionNotFound)

				return New(repo, nil, clk)
			},
			wantErr: "session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.Revoke(
				t.Context(),
				sessionID,
				"admin-user",
				"security policy",
				domain.SessionEventRevokedByAdmin,
			)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestService_RevokeAllForUser(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	userID := "user-cascade"
	revokedBy := "admin"
	reason := "account deactivated"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
	}{
		{
			name: "3 active sessions: ListActiveByUser then RevokeAllForUser then 3 Cascade events",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				activeSessions := []*domain.Session{
					{ID: "s1", UserID: userID},
					{ID: "s2", UserID: userID},
					{ID: "s3", UserID: userID},
				}

				gomock.InOrder(
					repo.EXPECT().ListActiveByUser(gomock.Any(), userID).Return(activeSessions, nil),
					repo.EXPECT().RevokeAllForUser(gomock.Any(), userID, revokedBy).Return(3, nil),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).
						DoAndReturn(func(_ any, e *domain.SessionEvent) error {
							assert.Equal(t, domain.SessionEventRevokedCascade, e.Type)
							assert.Equal(t, userID, e.UserID)
							assert.Equal(t, reason, e.Reason)
							assert.Equal(t, now, e.Timestamp)

							return nil
						}),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).
						DoAndReturn(func(_ any, e *domain.SessionEvent) error {
							assert.Equal(t, domain.SessionEventRevokedCascade, e.Type)

							return nil
						}),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).
						DoAndReturn(func(_ any, e *domain.SessionEvent) error {
							assert.Equal(t, domain.SessionEventRevokedCascade, e.Type)

							return nil
						}),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "zero active sessions: RevokeAllForUser called, no events appended",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				// evt.Append must NOT be called

				gomock.InOrder(
					repo.EXPECT().ListActiveByUser(gomock.Any(), userID).Return([]*domain.Session{}, nil),
					repo.EXPECT().RevokeAllForUser(gomock.Any(), userID, revokedBy).Return(0, nil),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "ListActiveByUser error propagated",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				repo.EXPECT().ListActiveByUser(gomock.Any(), userID).Return(nil, errors.New("db failure"))

				return New(
					repo,
					evt,
					clk,
				)
			},
			wantErr: "db failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.RevokeAllForUser(t.Context(), userID, revokedBy, reason)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
