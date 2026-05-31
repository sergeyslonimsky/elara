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

func TestService_ListMine(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	userID := "user-list"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
		want     []*domain.Session
	}{
		{
			name: "delegates to ListActiveByUser and returns sessions",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)

				sessions := []*domain.Session{
					{ID: "s1", UserID: userID, ExpiresAt: now.Add(time.Hour)},
					{ID: "s2", UserID: userID, ExpiresAt: now.Add(2 * time.Hour)},
				}

				repo.EXPECT().ListActiveByUser(gomock.Any(), userID).Return(sessions, nil)

				svc := New(repo, nil, nil)

				return svc
			},
			want: []*domain.Session{
				{ID: "s1", UserID: userID, ExpiresAt: now.Add(time.Hour)},
				{ID: "s2", UserID: userID, ExpiresAt: now.Add(2 * time.Hour)},
			},
		},
		{
			name: "repo error propagated",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)

				repo.EXPECT().ListActiveByUser(gomock.Any(), userID).Return(nil, errors.New("db error"))

				return New(repo, nil, nil)
			},
			wantErr: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.ListMine(t.Context(), userID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_ListByUser(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	targetID := "target-user"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
		want     []*domain.Session
	}{
		{
			name: "delegates to repo ListByUser (includes revoked) and returns sessions",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)

				sessions := []*domain.Session{
					{ID: "s1", UserID: targetID, ExpiresAt: now.Add(time.Hour)},
					{ID: "s2", UserID: targetID, RevokedAt: new(now.Add(-time.Hour))},
				}

				repo.EXPECT().ListByUser(gomock.Any(), targetID).Return(sessions, nil)

				svc := New(repo, nil, nil)

				return svc
			},
			want: func() []*domain.Session {
				return []*domain.Session{
					{ID: "s1", UserID: targetID, ExpiresAt: now.Add(time.Hour)},
					{ID: "s2", UserID: targetID, RevokedAt: new(now.Add(-time.Hour))},
				}
			}(),
		},
		{
			name: "repo error propagated",
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)

				repo.EXPECT().ListByUser(gomock.Any(), targetID).Return(nil, errors.New("storage error"))

				return New(repo, nil, nil)
			},
			wantErr: "storage error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.ListByUser(t.Context(), targetID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_ListEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	userID := "event-user"

	tests := []struct {
		name     string
		filter   EventFilter
		mockFunc func(*gomock.Controller) *Service
		wantErr  string
		want     []*domain.SessionEvent
	}{
		{
			name: "delegates to events ListByUser with UserID limit offset",
			filter: EventFilter{
				UserID: userID,
				Limit:  10,
				Offset: 5,
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)

				events := []*domain.SessionEvent{
					{ID: "e1", UserID: userID, Type: domain.SessionEventCreated, Timestamp: now},
					{ID: "e2", UserID: userID, Type: domain.SessionEventRefreshed, Timestamp: now.Add(time.Minute)},
				}

				evt.EXPECT().ListByUser(gomock.Any(), userID, 10, 5).Return(events, nil)

				svc := New(nil, evt, nil)

				return svc
			},
			want: []*domain.SessionEvent{
				{ID: "e1", UserID: userID, Type: domain.SessionEventCreated, Timestamp: now},
				{ID: "e2", UserID: userID, Type: domain.SessionEventRefreshed, Timestamp: now.Add(time.Minute)},
			},
		},
		{
			name: "events repo error propagated",
			filter: EventFilter{
				UserID: userID,
				Limit:  10,
				Offset: 0,
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)

				evt.EXPECT().ListByUser(gomock.Any(), userID, 10, 0).Return(nil, errors.New("event store error"))

				return New(nil, evt, nil)
			},
			wantErr: "event store error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.ListEvents(t.Context(), tt.filter)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
