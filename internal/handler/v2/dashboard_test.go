package v2

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

func TestDashboardHandler_GetStats(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_dashboard.NewMockdashboardEnforcer(ctrl)
	ns := mock_dashboard.NewMocknsLister(ctrl)
	configs := mock_dashboard.NewMockconfigCounter(ctrl)
	clients := mock_dashboard.NewMockactiveClientsSource(ctrl)

	uc := dashboarduc.NewUseCase(enforcer, ns, configs, nil, clients)
	h := NewDashboardHandler(uc)

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	enforcer.EXPECT().Enforce("admin@example.com", "*", "dashboard", "read").Return(true, nil)
	ns.EXPECT().List(gomock.Any()).Return([]*domain.Namespace{{Name: "n1"}}, nil)
	configs.EXPECT().CountByNamespace(gomock.Any(), "n1").Return(10, nil)
	configs.EXPECT().CurrentRevision(gomock.Any()).Return(int64(123), nil)
	clients.EXPECT().ListActive().Return([]*domain.Client{{ID: "c1"}})

	req := connect.NewRequest(&dashboardv1.GetStatsRequest{})

	resp, err := h.GetStats(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.GetNamespaceCount())
	assert.Equal(t, int32(10), resp.Msg.GetConfigCount())
	assert.Equal(t, int64(123), resp.Msg.GetGlobalRevision())
}
