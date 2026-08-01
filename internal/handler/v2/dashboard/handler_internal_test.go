package dashboard

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	dashboardmock "github.com/sergeyslonimsky/elara/internal/handler/v2/dashboard/mocks"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
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

func TestDashboardHandler_ListActivity(t *testing.T) {
	t.Parallel()

	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		limit    int32
		mockFunc func(uc *dashboardmock.Mockusecase)
		wantErr  string
		want     []*dashboardv1.ActivityEntry
	}{
		{
			name:  "maps all changelog event types to proto",
			limit: 10,
			mockFunc: func(uc *dashboardmock.Mockusecase) {
				uc.EXPECT().ListActivity(gomock.Any(), 10).Return([]*domain.ChangelogEntry{
					{
						Revision:  1,
						Path:      "/a",
						Namespace: "ns",
						Version:   1,
						Timestamp: ts,
						Type:      domain.EventTypeCreated,
					},
					{
						Revision:  2,
						Path:      "/a",
						Namespace: "ns",
						Version:   2,
						Timestamp: ts,
						Type:      domain.EventTypeUpdated,
					},
					{
						Revision:  3,
						Path:      "/a",
						Namespace: "ns",
						Version:   3,
						Timestamp: ts,
						Type:      domain.EventTypeDeleted,
					},
					{
						Revision:  4,
						Path:      "/a",
						Namespace: "ns",
						Version:   4,
						Timestamp: ts,
						Type:      domain.EventTypeLocked,
					},
					{
						Revision:  5,
						Path:      "/a",
						Namespace: "ns",
						Version:   5,
						Timestamp: ts,
						Type:      domain.EventTypeUnlocked,
					},
					{
						Revision:  6,
						Path:      "/a",
						Namespace: "ns",
						Version:   6,
						Timestamp: ts,
						Type:      domain.EventTypeNamespaceLocked,
					},
					{
						Revision:  7,
						Path:      "/a",
						Namespace: "ns",
						Version:   7,
						Timestamp: ts,
						Type:      domain.EventTypeNamespaceUnlocked,
					},
				}, nil)
			},
			want: []*dashboardv1.ActivityEntry{
				{
					Revision:  1,
					Path:      "/a",
					Namespace: "ns",
					Version:   1,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_CREATED,
				},
				{
					Revision:  2,
					Path:      "/a",
					Namespace: "ns",
					Version:   2,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_UPDATED,
				},
				{
					Revision:  3,
					Path:      "/a",
					Namespace: "ns",
					Version:   3,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_DELETED,
				},
				{
					Revision:  4,
					Path:      "/a",
					Namespace: "ns",
					Version:   4,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_LOCKED,
				},
				{
					Revision:  5,
					Path:      "/a",
					Namespace: "ns",
					Version:   5,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_UNLOCKED,
				},
				{
					Revision:  6,
					Path:      "/a",
					Namespace: "ns",
					Version:   6,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_NAMESPACE_LOCKED,
				},
				{
					Revision:  7,
					Path:      "/a",
					Namespace: "ns",
					Version:   7,
					Timestamp: timestamppb.New(ts),
					EventType: configv1.EventType_EVENT_TYPE_NAMESPACE_UNLOCKED,
				},
			},
		},
		{
			name:  "empty result returns empty slice",
			limit: 5,
			mockFunc: func(uc *dashboardmock.Mockusecase) {
				uc.EXPECT().ListActivity(gomock.Any(), 5).Return([]*domain.ChangelogEntry{}, nil)
			},
			want: []*dashboardv1.ActivityEntry{},
		},
		{
			name:     "negative limit returns invalid argument before calling usecase",
			limit:    -1,
			mockFunc: func(*dashboardmock.Mockusecase) {},
			wantErr:  "normalize limit",
		},
		{
			name:  "usecase error is propagated",
			limit: 5,
			mockFunc: func(uc *dashboardmock.Mockusecase) {
				uc.EXPECT().ListActivity(gomock.Any(), 5).Return(nil, domain.ErrUnauthorized)
			},
			wantErr: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := dashboardmock.NewMockusecase(ctrl)
			tc.mockFunc(uc)

			h := New(uc)

			resp, err := h.ListActivity(
				t.Context(),
				connect.NewRequest(&dashboardv1.ListActivityRequest{Limit: tc.limit}),
			)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, resp.Msg.GetEntries())
		})
	}
}
