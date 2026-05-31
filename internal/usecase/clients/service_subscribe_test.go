package clients_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_SubscribeChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has(
						"test@example.com",
						domain.Permission{
							Object: domain.ObjectClient,
							Action: domain.ActionRead,
							Domain: domain.DomainAll,
						}).
					Return(true)

				ch := make(chan domain.ClientChange)
				m.active.EXPECT().Subscribe().Return(ch, func() { close(ch) })

				return ctx, ch
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange) {
				return ctx, nil
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx, expectedCh := tt.mockFunc(t.Context(), m)

			ch, cleanup, err := svc.SubscribeChanges(ctx)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, expectedCh, ch)

			cleanup()
			_, ok := <-ch
			assert.False(t, ok, "channel should be closed after cleanup")
		})
	}
}

func TestService_SubscribeClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		connID   string
		mockFunc func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange)
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			connID: "conn-1",
			mockFunc: func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has(
						"test@example.com",
						domain.Permission{
							Object: domain.ObjectClient,
							Action: domain.ActionRead,
							Domain: domain.DomainAll,
						}).
					Return(true)

				ch := make(chan domain.ClientChange)
				m.active.EXPECT().SubscribeClient("conn-1").Return(ch, func() { close(ch) })

				return ctx, ch
			},
		},
		{
			name:   "unauthorized",
			connID: "conn-1",
			mockFunc: func(ctx context.Context, m mocks) (context.Context, <-chan domain.ClientChange) {
				return ctx, nil
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx, expectedCh := tt.mockFunc(t.Context(), m)

			ch, cleanup, err := svc.SubscribeClient(ctx, tt.connID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, expectedCh, ch)

			cleanup()
			_, ok := <-ch
			assert.False(t, ok, "channel should be closed after cleanup")
		})
	}
}
