package monitor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/monitor"
	mockmonitor "github.com/sergeyslonimsky/elara/internal/monitor/mocks"
)

func TestHistoryStore_Record(t *testing.T) {
	t.Parallel()

	now := time.Now()
	snap := &domain.Client{ID: "a", DisconnectedAt: &now}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*monitor.HistoryStore, context.Context)
	}{
		{
			name: "persists async and applies retention",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().Save(gomock.Any(), snap).Return(nil)
				repo.EXPECT().DeleteOlderThan(gomock.Any(), gomock.Any()).Return(0, nil)
				repo.EXPECT().Count(gomock.Any()).Return(1, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
		},
		{
			name: "tolerant of repo errors",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().Save(gomock.Any(), snap).Return(errors.New("boom"))
				repo.EXPECT().DeleteOlderThan(gomock.Any(), gomock.Any()).Return(0, nil)
				repo.EXPECT().Count(gomock.Any()).Return(0, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
		},
		{
			name: "skips age retention when MaxAge is negative",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().Save(gomock.Any(), snap).Return(nil)
				// DeleteOlderThan NOT expected
				repo.EXPECT().Count(gomock.Any()).Return(1, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{MaxAge: -1}, repo), ctx
			},
		},
		{
			name: "applies count retention when limit exceeded",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().Save(gomock.Any(), snap).Return(nil)
				repo.EXPECT().DeleteOlderThan(gomock.Any(), gomock.Any()).Return(0, nil)
				repo.EXPECT().Count(gomock.Any()).Return(10, nil)
				repo.EXPECT().DeleteOldest(gomock.Any(), 5).Return(5, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{MaxRecords: 5}, repo), ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, _ := tt.mockFunc(t.Context(), ctrl)

			sut.Record(snap)
			sut.Shutdown()
		})
	}
}

func TestHistoryStore_Record_NonBlocking(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mockmonitor.NewMockClientHistoryRepo(ctrl)

	block := make(chan struct{})
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, c *domain.Client) error {
		<-block

		return nil
	}).AnyTimes()
	repo.EXPECT().DeleteOlderThan(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	repo.EXPECT().Count(gomock.Any()).Return(1, nil).AnyTimes()

	sut := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{BufferSize: 1}, repo)
	t.Cleanup(func() {
		close(block)
		sut.Shutdown()
	})

	// 1st Record: picked up by run(), blocks in Save
	sut.Record(&domain.Client{ID: "1"})

	// Wait for it to be picked up
	time.Sleep(50 * time.Millisecond)

	// 2nd Record: fills the buffer
	sut.Record(&domain.Client{ID: "2"})

	// 3rd Record: should drop immediately
	done := make(chan struct{})
	go func() {
		sut.Record(&domain.Client{ID: "3"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Record() blocked on full buffer")
	}
}

func TestHistoryStore_Shutdown(t *testing.T) {
	t.Parallel()

	t.Run("drains pending", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := mockmonitor.NewMockClientHistoryRepo(ctrl)

		repo.EXPECT().Save(gomock.Any(), gomock.Any()).Times(5).Return(nil)
		repo.EXPECT().DeleteOlderThan(gomock.Any(), gomock.Any()).Times(5).Return(0, nil)
		repo.EXPECT().Count(gomock.Any()).Times(5).Return(1, nil)

		sut := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{BufferSize: 10}, repo)
		for range 5 {
			sut.Record(&domain.Client{ID: "x"})
		}
		sut.Shutdown()
	})

	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()
		repo := mockmonitor.NewMockClientHistoryRepo(gomock.NewController(t))
		sut := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
		sut.Shutdown()
		sut.Shutdown()
	})

	t.Run("prevents new records", func(t *testing.T) {
		t.Parallel()
		repo := mockmonitor.NewMockClientHistoryRepo(gomock.NewController(t))
		sut := monitor.NewHistoryStore(t.Context(), monitor.HistoryConfig{}, repo)
		sut.Shutdown()

		// Should not call repo
		sut.Record(&domain.Client{ID: "x"})
	})
}

func TestHistoryStore_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		limit    int
		mockFunc func(context.Context, *gomock.Controller) (*monitor.HistoryStore, context.Context)
		wantErr  string
		want     []*domain.Client
	}{
		{
			name:  "success",
			limit: 10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().List(ctx, 10).Return([]*domain.Client{{ID: "1"}}, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
			want: []*domain.Client{{ID: "1"}},
		},
		{
			name:  "error",
			limit: 10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().List(ctx, 10).Return(nil, errors.New("db failure"))

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
			wantErr: "list client history: db failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)
			defer sut.Shutdown()

			got, err := sut.List(ctx, tt.limit)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHistoryStore_ListByClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		client    string
		namespace string
		limit     int
		mockFunc  func(context.Context, *gomock.Controller) (*monitor.HistoryStore, context.Context)
		wantErr   string
		want      []*domain.Client
	}{
		{
			name:      "success",
			client:    "c1",
			namespace: "n1",
			limit:     10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().ListByClient(ctx, "c1", "n1", 10).Return([]*domain.Client{{ID: "1"}}, nil)

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
			want: []*domain.Client{{ID: "1"}},
		},
		{
			name:      "error",
			client:    "c1",
			namespace: "n1",
			limit:     10,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*monitor.HistoryStore, context.Context) {
				repo := mockmonitor.NewMockClientHistoryRepo(ctrl)
				repo.EXPECT().ListByClient(ctx, "c1", "n1", 10).Return(nil, errors.New("db failure"))

				return monitor.NewHistoryStore(ctx, monitor.HistoryConfig{}, repo), ctx
			},
			wantErr: "list client history by client: db failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)
			defer sut.Shutdown()

			got, err := sut.ListByClient(ctx, tt.client, tt.namespace, tt.limit)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
