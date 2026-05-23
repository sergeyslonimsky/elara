package clients_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_ListActive(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		want     []*domain.Client
		errIs    error
		wantErr  string
	}{
		{
			name: "success sorted by connected_at",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.active.EXPECT().ListActive().Return([]*domain.Client{
					{ID: "b", ConnectedAt: now.Add(time.Second)},
					{ID: "a", ConnectedAt: now},
					{ID: "c", ConnectedAt: now.Add(2 * time.Second)},
				})

				return ctx
			},
			want: []*domain.Client{
				{ID: "a", ConnectedAt: now},
				{ID: "b", ConnectedAt: now.Add(time.Second)},
				{ID: "c", ConnectedAt: now.Add(2 * time.Second)},
			},
		},
		{
			name: "unauthorized",
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

			got, err := svc.ListActive(ctx)

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

func TestService_ListHistorical(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		limit    int
		mockFunc func(ctx context.Context, m mocks) context.Context
		want     []*domain.Client
		errIs    error
		wantErr  string
	}{
		{
			name:  "success default limit",
			limit: 0,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().List(ctx, 100).Return([]*domain.Client{
					{ID: "h1", DisconnectedAt: &now},
				}, nil)

				return ctx
			},
			want: []*domain.Client{
				{ID: "h1", DisconnectedAt: &now},
			},
		},
		{
			name:  "respects limit",
			limit: 5,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().List(ctx, 5).Return([]*domain.Client{
					{ID: "h1", DisconnectedAt: &now},
				}, nil)

				return ctx
			},
			want: []*domain.Client{
				{ID: "h1", DisconnectedAt: &now},
			},
		},
		{
			name:  "history error",
			limit: 10,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().List(ctx, 10).Return(nil, errors.New("db error"))

				return ctx
			},
			wantErr: "list historical connections: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.ListHistorical(ctx, tt.limit)

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

func TestService_ListSessions(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name         string
		clientName   string
		k8sNamespace string
		currentID    string
		limit        int
		mockFunc     func(ctx context.Context, m mocks) context.Context
		want         []*domain.Client
		errIs        error
		wantErr      string
	}{
		{
			name:         "matches name+namespace",
			clientName:   "order-service",
			k8sNamespace: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().ListByClient(ctx, "order-service", "prod", 51).Return([]*domain.Client{
					{ID: "a", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
					{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
				}, nil)

				return ctx
			},
			want: []*domain.Client{
				{ID: "a", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
				{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
			},
		},
		{
			name:         "excludes currentID",
			clientName:   "order-service",
			k8sNamespace: "prod",
			currentID:    "a",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().ListByClient(ctx, "order-service", "prod", 51).Return([]*domain.Client{
					{ID: "a", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
					{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
				}, nil)

				return ctx
			},
			want: []*domain.Client{
				{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
			},
		},
		{
			name:       "empty clientName returns nothing",
			clientName: "",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				return ctx
			},
			want: nil,
		},
		{
			name:         "respects limit after exclusion",
			clientName:   "order-service",
			k8sNamespace: "prod",
			currentID:    "a",
			limit:        2,
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().ListByClient(ctx, "order-service", "prod", 3).Return([]*domain.Client{
					{ID: "a", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
					{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
					{ID: "c", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
				}, nil)

				return ctx
			},
			want: []*domain.Client{
				{ID: "b", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
				{ID: "c", ClientName: "order-service", K8sNamespace: "prod", DisconnectedAt: &now},
			},
		},
		{
			name:         "history error",
			clientName:   "order-service",
			k8sNamespace: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
				m.pdp.EXPECT().
					Has("test@example.com", domain.Permission{Object: domain.ObjectClient, Action: domain.ActionRead, Domain: domain.DomainAll}).
					Return(true)

				m.history.EXPECT().ListByClient(ctx, "order-service", "prod", 51).Return(nil, errors.New("db error"))

				return ctx
			},
			wantErr: "list sessions by client: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.ListSessions(ctx, tt.clientName, tt.k8sNamespace, tt.currentID, tt.limit)

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
