package dashboard

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	dashboardmock "github.com/sergeyslonimsky/elara/internal/handler/v2/dashboard/mocks"
	dashboardv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
)

func TestDashboardHandler_GetStats_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	uc := dashboardmock.NewMockusecase(ctrl)
	uc.EXPECT().GetStats(gomock.Any()).Return(&dashboarduc.StatsResult{
		NamespaceCount:    1,
		ConfigCount:       10,
		ActiveClientCount: 1,
		GlobalRevision:    123,
	}, nil)

	h := New(uc)

	resp, err := h.GetStats(t.Context(), connect.NewRequest(&dashboardv1.GetStatsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.GetNamespaceCount())
	assert.Equal(t, int32(10), resp.Msg.GetConfigCount())
	assert.Equal(t, int64(123), resp.Msg.GetGlobalRevision())
	assert.Equal(t, int32(1), resp.Msg.GetActiveClientCount())
}

func TestDashboardHandler_GetStats_UsecaseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	uc := dashboardmock.NewMockusecase(ctrl)
	uc.EXPECT().GetStats(gomock.Any()).Return(nil, domain.ErrUnauthorized)

	h := New(uc)

	_, err := h.GetStats(t.Context(), connect.NewRequest(&dashboardv1.GetStatsRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
