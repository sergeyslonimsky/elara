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

func TestService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		params     CreateParams
		mockFunc   func(*gomock.Controller) *Service
		wantErr    string
		wantTTL    time.Duration
		wantClient domain.ClientType
	}{
		{
			name: "web client creates session with 8h TTL and Created event",
			params: CreateParams{
				UserID:     "user-1",
				ClientType: "web",
				IP:         "127.0.0.1",
				UserAgent:  "Mozilla/5.0",
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				gomock.InOrder(
					repo.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).Return(nil),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).Return(nil),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
			wantTTL:    8 * time.Hour,
			wantClient: domain.ClientTypeWeb,
		},
		{
			name: "cli client creates session with 30d TTL",
			params: CreateParams{
				UserID:     "user-2",
				ClientType: "cli",
				IP:         "10.0.0.1",
				UserAgent:  "elara-cli/1.0",
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				gomock.InOrder(
					repo.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&domain.Session{})).Return(nil),
					evt.EXPECT().Append(gomock.Any(), gomock.AssignableToTypeOf(&domain.SessionEvent{})).Return(nil),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
			wantTTL:    30 * 24 * time.Hour,
			wantClient: domain.ClientTypeCLI,
		},
		{
			name: "repo Create error propagated, event Append not called",
			params: CreateParams{
				UserID:     "user-3",
				ClientType: "web",
				IP:         "127.0.0.1",
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

				return New(
					repo,
					evt,
					clk,
				)
			},
			wantErr: "db error",
		},
		{
			name: "event Append error propagated",
			params: CreateParams{
				UserID:     "user-4",
				ClientType: "web",
				IP:         "127.0.0.1",
			},
			mockFunc: func(ctrl *gomock.Controller) *Service {
				repo := sessions_mock.NewMocksessionRepository(ctrl)
				evt := sessions_mock.NewMocksessionEventRepository(ctrl)
				clk := sessions_mock.NewMockClock(ctrl)

				clk.EXPECT().Now().Return(now)

				gomock.InOrder(
					repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil),
					evt.EXPECT().Append(gomock.Any(), gomock.Any()).Return(errors.New("append error")),
				)

				return New(
					repo,
					evt,
					clk,
				)
			},
			wantErr: "append error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.Create(t.Context(), tt.params)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.params.UserID, got.UserID)
			assert.Equal(t, tt.wantClient, got.ClientType)
			assert.Equal(t, tt.params.IP, got.IP)
			assert.Equal(t, tt.params.UserAgent, got.UserAgent)
			assert.Equal(t, now, got.CreatedAt)
			assert.Equal(t, now, got.LastSeenAt)
			assert.Equal(t, tt.wantTTL, got.ExpiresAt.Sub(got.CreatedAt))
		})
	}
}
