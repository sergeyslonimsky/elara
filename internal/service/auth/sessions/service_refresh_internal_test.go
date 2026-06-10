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

func TestService_Refresh(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	sessionID := "session-refresh"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
	}{
		{
			name: "throttle: seen less than 60s ago is a no-op",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				// LastSeenAt = 30s ago, below refreshThrottle of 60s
				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					CreatedAt:  now.Add(-time.Hour),
					LastSeenAt: now.Add(-30 * time.Second),
					ExpiresAt:  now.Add(time.Hour),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)
				// No Update or Append expected

				return New(repo, nil, clk)
			},
		},
		{
			name: "web sliding: extends ExpiresAt and appends Refreshed event when delta > 5min",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				// LastSeenAt = 5 min ago (> throttle); current ExpiresAt = 1h from now
				// newExpires = now + 8h; delta = 8h - 1h = 7h > 5min → extend + event
				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					CreatedAt:  now.Add(-time.Hour),
					LastSeenAt: now.Add(-5 * time.Minute),
					ExpiresAt:  now.Add(time.Hour),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				repo.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).
					DoAndReturn(func(_ any, s *domain.Session) error {
						assert.Equal(t, now, s.LastSeenAt)
						assert.Equal(t, now.Add(defaultWebTTL), s.ExpiresAt)

						return nil
					})
				evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).
					DoAndReturn(func(_ any, e *domain.SessionEvent) error {
						assert.Equal(t, domain.SessionEventRefreshed, e.Type)
						assert.Equal(t, sessionID, e.SessionID)

						return nil
					})

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "web sliding cap: ExpiresAt capped at CreatedAt + 30d",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				// CreatedAt = 29d23h ago → hardCap = CreatedAt + 30d = 1h from now
				// newExpires = now + 8h > hardCap → capped at CreatedAt + 30d
				createdAt := now.Add(-(29*24*time.Hour + 23*time.Hour))
				hardCap := createdAt.Add(maxWebSlidingTTL)

				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					CreatedAt:  createdAt,
					LastSeenAt: now.Add(-5 * time.Minute),
					ExpiresAt:  now.Add(-10 * time.Minute),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				repo.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).
					DoAndReturn(func(_ any, s *domain.Session) error {
						assert.Equal(t, hardCap, s.ExpiresAt, "ExpiresAt must be capped at CreatedAt+30d")

						return nil
					})
				evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).Return(nil)

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "web small delta (<= 5min): updates LastSeenAt only, no Refreshed event",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				// newExpires = now + 8h; current ExpiresAt = now + 7h59m
				// delta = 1min <= 5min → no extension, no event
				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeWeb,
					CreatedAt:  now.Add(-time.Hour),
					LastSeenAt: now.Add(-5 * time.Minute),
					ExpiresAt:  now.Add(defaultWebTTL - time.Minute),
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				// evt Append must NOT be called — no EXPECT registered

				repo.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).
					DoAndReturn(func(_ any, s *domain.Session) error {
						assert.Equal(t, now, s.LastSeenAt)
						assert.Equal(t, now.Add(defaultWebTTL-time.Minute), s.ExpiresAt)

						return nil
					})

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
		{
			name: "cli: only LastSeenAt updated, ExpiresAt unchanged, no event",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				originalExpires := now.Add(20 * 24 * time.Hour)
				sess := &domain.Session{
					ID:         sessionID,
					UserID:     "user-1",
					ClientType: domain.ClientTypeCLI,
					CreatedAt:  now.Add(-time.Hour),
					LastSeenAt: now.Add(-5 * time.Minute),
					ExpiresAt:  originalExpires,
				}

				repo.EXPECT().Get(gomock.Any(), sessionID).Return(sess, nil)
				clk.EXPECT().Now().Return(now)

				// evt Append must NOT be called for CLI

				repo.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).
					DoAndReturn(func(_ any, s *domain.Session) error {
						assert.Equal(t, now, s.LastSeenAt)
						assert.Equal(t, originalExpires, s.ExpiresAt, "CLI ExpiresAt must not change")

						return nil
					})

				return New(
					repo,
					evt,
					clk,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.Refresh(t.Context(), sessionID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
