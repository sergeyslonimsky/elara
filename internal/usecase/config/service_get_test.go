package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.GetInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success",
			input: config.GetInput{
				Namespace: "prod",
				Path:      "/app/config.json",
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					Get(ctx, "/app/config.json", "prod").
					Return(&domain.Config{Path: "/app/config.json", Namespace: "prod"}, nil)
			},
			want: &domain.Config{Path: "/app/config.json", Namespace: "prod"},
		},
		{
			name:  "missing namespace",
			input: config.GetInput{Namespace: ""},
			mockFunc: func(_ context.Context, _ mocks) {
			},
			wantErr: "namespace is required",
		},
		{
			name:  "not found",
			input: config.GetInput{Namespace: "prod", Path: "/missing"},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					Get(ctx, "/missing", "prod").
					Return(nil, domain.ErrNotFound)
			},
			errIs: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := svc.Get(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Path, got.Path)
			assert.Equal(t, tt.want.Namespace, got.Namespace)
		})
	}
}

func TestService_GetAtRevision(t *testing.T) {
	t.Parallel()

	expectedEntry := &domain.HistoryEntry{
		Revision: 10,
		Content:  `{"key":"val"}`,
	}

	tests := []struct {
		name     string
		input    config.GetAtRevisionInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
		want     *domain.HistoryEntry
	}{
		{
			name: "success",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					GetAtRevision(ctx, "/db/config", "prod", int64(10)).
					Return(expectedEntry, nil)
			},
			want: expectedEntry,
		},
		{
			name: "empty namespace",
			input: config.GetAtRevisionInput{
				Path:     "/db/config",
				Revision: 10,
			},
			mockFunc: func(_ context.Context, _ mocks) {
			},
			wantErr: "namespace is required",
		},
		{
			name: "storage error",
			input: config.GetAtRevisionInput{
				Namespace: "prod",
				Path:      "/db/config",
				Revision:  10,
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					GetAtRevision(ctx, "/db/config", "prod", int64(10)).
					Return(nil, errors.New("not found"))
			},
			wantErr: "get config at revision: not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			got, err := svc.GetAtRevision(ctx, tt.input)

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
