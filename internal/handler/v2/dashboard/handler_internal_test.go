package dashboard

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	dashboardv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
	mock_dashboard "github.com/sergeyslonimsky/elara/internal/usecase/dashboard/mocks"
)

func TestDashboardHandler_GetStats_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
	ns := mock_dashboard.NewMocknsLister(ctrl)
	configs := mock_dashboard.NewMockconfigCounter(ctrl)
	clients := mock_dashboard.NewMockactiveClientsSource(ctrl)

	enforcer.EXPECT().Enforce("admin@example.com", "*", "dashboard", "read").Return(true, nil)
	ns.EXPECT().List(gomock.Any()).Return([]*domain.Namespace{{Name: "n1"}}, nil)
	configs.EXPECT().CountByNamespace(gomock.Any(), "n1").Return(10, nil)
	configs.EXPECT().CurrentRevision(gomock.Any()).Return(int64(123), nil)
	clients.EXPECT().ListActive().Return([]*domain.Client{{ID: "c1"}})

	h := &Handler{uc: dashboarduc.NewUseCase(enforcer, ns, configs, nil, clients)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	resp, err := h.GetStats(ctx, connect.NewRequest(&dashboardv1.GetStatsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.GetNamespaceCount())
	assert.Equal(t, int32(10), resp.Msg.GetConfigCount())
	assert.Equal(t, int64(123), resp.Msg.GetGlobalRevision())
	assert.Equal(t, int32(1), resp.Msg.GetActiveClientCount())
}

func TestDashboardHandler_GetStats_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &Handler{uc: dashboarduc.NewUseCase(nil, nil, nil, nil, nil)}

	_, err := h.GetStats(context.Background(), connect.NewRequest(&dashboardv1.GetStatsRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDashboardHandler_GetStats_Forbidden(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "*", "dashboard", "read").Return(false, nil)

	h := &Handler{uc: dashboarduc.NewUseCase(enforcer, nil, nil, nil, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.GetStats(ctx, connect.NewRequest(&dashboardv1.GetStatsRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
