package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestHistoryUseCase_GetHistory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		ns       string
		limit    int
		mockFunc func(context.Context, *gomock.Controller) (*config.HistoryUseCase, context.Context)
		errIs    error
		wantErr  string
		want     []*domain.HistoryEntry
	}{
		{
			name:  "success",
			path:  "/a.json",
			ns:    "prod",
			limit: 10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockhistoryEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				configs := mock_config.NewMockconfigHistoryReader(ctrl)
				entries := []*domain.HistoryEntry{{Revision: 1}}
				configs.EXPECT().GetConfigHistory(ctx, "/a.json", "prod", 10).Return(entries, nil)
				return config.NewHistoryUseCase(enforcer, configs), ctx
			},
			want: []*domain.HistoryEntry{{Revision: 1}},
		},
		{
			name: "missing namespace",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				return config.NewHistoryUseCase(nil, nil), ctx
			},
			wantErr: "namespace is required",
		},
		{
			name: "unauthorized",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				return config.NewHistoryUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "history reader error",
			path:  "/a.json",
			ns:    "prod",
			limit: 10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockhistoryEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				configs := mock_config.NewMockconfigHistoryReader(ctrl)
				configs.EXPECT().GetConfigHistory(ctx, "/a.json", "prod", 10).Return(nil, errors.New("db error"))
				return config.NewHistoryUseCase(enforcer, configs), ctx
			},
			wantErr: "get config history: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)
			got, err := sut.GetHistory(ctx, tt.path, tt.ns, tt.limit)
			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHistoryUseCase_GetAtRevision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		ns       string
		rev      int64
		mockFunc func(context.Context, *gomock.Controller) (*config.HistoryUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.HistoryEntry
	}{
		{
			name: "success",
			path: "/a.json",
			ns:   "prod",
			rev:  5,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockhistoryEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				configs := mock_config.NewMockconfigHistoryReader(ctrl)
				entry := &domain.HistoryEntry{Revision: 5}
				configs.EXPECT().GetAtRevision(ctx, "/a.json", "prod", int64(5)).Return(entry, nil)
				return config.NewHistoryUseCase(enforcer, configs), ctx
			},
			want: &domain.HistoryEntry{Revision: 5},
		},
		{
			name: "forbidden",
			path: "/a.json",
			ns:   "prod",
			rev:  5,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.HistoryUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockhistoryEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, nil)
				return config.NewHistoryUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)
			got, err := sut.GetAtRevision(ctx, tt.path, tt.ns, tt.rev)
			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
