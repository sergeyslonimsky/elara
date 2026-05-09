package clients_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		mockFunc   func(ctx context.Context, m mocks) context.Context
		wantClient *domain.Client
		wantEvents []domain.ClientEvent
		errIs      error
		wantErr    string
	}{
		{
			name: "active client success",
			id:   "active-id",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.enforcer.EXPECT().
					Enforce("test@example.com", auth.ObjectAll, auth.ObjectClient, auth.ActionRead).
					Return(true, nil)

				m.active.EXPECT().Get("active-id").Return(&domain.Client{ID: "active-id"})
				m.active.EXPECT().RecentEvents("active-id").Return([]domain.ClientEvent{{Method: "Put"}})

				return ctx
			},
			wantClient: &domain.Client{ID: "active-id"},
			wantEvents: []domain.ClientEvent{{Method: "Put"}},
		},
		{
			name: "fallback to history success",
			id:   "historical-id",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.enforcer.EXPECT().
					Enforce("test@example.com", auth.ObjectAll, auth.ObjectClient, auth.ActionRead).
					Return(true, nil)

				m.active.EXPECT().Get("historical-id").Return(nil)
				m.history.EXPECT().List(ctx, 1000).Return([]*domain.Client{
					{ID: "other-id"},
					{ID: "historical-id"},
				}, nil)

				return ctx
			},
			wantClient: &domain.Client{ID: "historical-id"},
			wantEvents: nil,
		},
		{
			name: "not found anywhere",
			id:   "missing-id",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.enforcer.EXPECT().
					Enforce("test@example.com", auth.ObjectAll, auth.ObjectClient, auth.ActionRead).
					Return(true, nil)

				m.active.EXPECT().Get("missing-id").Return(nil)
				m.history.EXPECT().List(ctx, 1000).Return([]*domain.Client{
					{ID: "other-id"},
				}, nil)

				return ctx
			},
			wantClient: nil,
			wantEvents: nil,
			errIs:      domain.ErrNotFound,
		},
		{
			name: "history error",
			id:   "some-id",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.enforcer.EXPECT().
					Enforce("test@example.com", auth.ObjectAll, auth.ObjectClient, auth.ActionRead).
					Return(true, nil)

				m.active.EXPECT().Get("some-id").Return(nil)
				m.history.EXPECT().List(ctx, 1000).Return(nil, errors.New("history boom"))

				return ctx
			},
			wantErr: "list historical clients: history boom",
		},
		{
			name: "unauthorized",
			id:   "some-id",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			client, events, err := svc.Get(ctx, tt.id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantClient, client)
			assert.Equal(t, tt.wantEvents, events)
		})
	}
}
