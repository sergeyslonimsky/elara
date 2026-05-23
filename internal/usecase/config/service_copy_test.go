package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Copy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.CopyInput
		mockFunc func(ctx context.Context, m mocks)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: config.CopyInput{
				SourcePath:      "/a.json",
				SourceNamespace: "src",
				DestPath:        "/b.json",
				DestNamespace:   "dst",
			},
			mockFunc: func(ctx context.Context, m mocks) {
				m.storage.EXPECT().
					Get(ctx, "/a.json", "src").
					Return(&domain.Config{Path: "/a.json", Namespace: "src", Content: "{}", Format: domain.FormatJSON}, nil)

				m.namespaceProvider.EXPECT().
					Get(ctx, "dst").
					Return(&domain.Namespace{Name: "dst"}, nil)

				m.storage.EXPECT().Create(ctx, gomock.Any()).Return(nil)
				m.namespaceProvider.EXPECT().UpdateTimestamp(ctx, "dst").Return(nil)
				m.watcher.EXPECT().NotifyCreated(ctx, gomock.Any())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := t.Context()
			tt.mockFunc(ctx, m)

			_, err := svc.Copy(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
