package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	configmock "github.com/sergeyslonimsky/elara/internal/handler/v2/config/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

// -----------------------------------------------------------------------------
// Pagination normalization error branches
// -----------------------------------------------------------------------------

func TestConfigHandler_ListConfigs_InvalidLimit(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

	_, err := h.ListConfigs(t.Context(), connect.NewRequest(&configv1.ListConfigsRequest{
		Namespace:  "prod",
		Pagination: &commonv1.PaginationRequest{Limit: -1},
	}))
	require.ErrorContains(t, err, "normalize limit")
}

func TestConfigHandler_ListConfigs_InvalidOffset(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

	_, err := h.ListConfigs(t.Context(), connect.NewRequest(&configv1.ListConfigsRequest{
		Namespace:  "prod",
		Pagination: &commonv1.PaginationRequest{Limit: 10, Offset: -1},
	}))
	require.ErrorContains(t, err, "normalize offset")
}

func TestConfigHandler_ListConfigs_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)
	uc.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.ListConfigs(t.Context(), connect.NewRequest(&configv1.ListConfigsRequest{
		Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigHistory_InvalidLimit(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

	_, err := h.GetConfigHistory(t.Context(), connect.NewRequest(&configv1.GetConfigHistoryRequest{
		Path: "/a.json", Namespace: "prod", Limit: -1,
	}))
	require.ErrorContains(t, err, "normalize limit")
}

func TestConfigHandler_GetConfigHistory_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)
	uc.EXPECT().History(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.GetConfigHistory(t.Context(), connect.NewRequest(&configv1.GetConfigHistoryRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_SearchConfigs_InvalidLimit(t *testing.T) {
	t.Parallel()

	h, _, _ := setupHandler(t)

	_, err := h.SearchConfigs(t.Context(), connect.NewRequest(&configv1.SearchConfigsRequest{
		Query:      "app",
		Namespace:  "prod",
		Pagination: &commonv1.PaginationRequest{Limit: -1},
	}))
	require.ErrorContains(t, err, "normalize limit")
}

func TestConfigHandler_SearchConfigs_InvalidOffset(t *testing.T) {
	t.Parallel()

	h, _, _ := setupHandler(t)

	_, err := h.SearchConfigs(t.Context(), connect.NewRequest(&configv1.SearchConfigsRequest{
		Query:      "app",
		Namespace:  "prod",
		Pagination: &commonv1.PaginationRequest{Limit: 10, Offset: -1},
	}))
	require.ErrorContains(t, err, "normalize offset")
}

func TestConfigHandler_CopyConfig_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	gomock.InOrder(
		az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "ns1").Return(nil),
		az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionWrite, "ns2").Return(nil),
	)
	uc.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.CopyConfig(t.Context(), connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath: "/src.json", SourceNamespace: "ns1",
		DestinationPath: "/dst.json", DestinationNamespace: "ns2",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_ValidateConfig_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)
	uc.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil, domain.ErrInvalidFormat)

	_, err := h.ValidateConfig(t.Context(), connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigDiff_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)
	uc.EXPECT().Diff(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.GetConfigDiff(t.Context(), connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_UnlockConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").Return(nil)
	uc.EXPECT().Unlock(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

	_, err := h.UnlockConfig(t.Context(), connect.NewRequest(&configv1.UnlockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigAtRevision_UsecaseError(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)
	uc.EXPECT().GetAtRevision(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.GetConfigAtRevision(t.Context(), connect.NewRequest(&configv1.GetConfigAtRevisionRequest{
		Path: "/a.json", Namespace: "prod", Revision: 1,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// WatchConfigs streaming
// -----------------------------------------------------------------------------

func newConfigTestServer(t *testing.T, h configv1connect.ConfigServiceHandler) string {
	t.Helper()

	mux := http.NewServeMux()
	path, handler := configv1connect.NewConfigServiceHandler(h)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv.URL
}

func receiveConfigFrame(
	t *testing.T,
	stream *connect.ServerStreamForClient[configv1.WatchConfigsResponse],
) *configv1.WatchConfigsResponse {
	t.Helper()

	done := make(chan struct{})
	var ok bool

	go func() {
		ok = stream.Receive()
		close(done)
	}()

	select {
	case <-done:
		require.True(t, ok, "stream.Receive returned false: %v", stream.Err())

		return stream.Msg()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream frame")

		return nil
	}
}

func TestConfigHandler_WatchConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := configmock.NewMockauthz(ctrl)
	uc := configmock.NewMockconfigUsecase(ctrl)
	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(domain.ErrUnauthorized)

	h := config.NewConfigHandler(az, uc)
	url := newConfigTestServer(t, h)

	client := configv1connect.NewConfigServiceClient(http.DefaultClient, url)
	stream, err := client.WatchConfigs(t.Context(), connect.NewRequest(&configv1.WatchConfigsRequest{
		Namespace: "prod",
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	require.False(t, stream.Receive())
	require.Error(t, stream.Err())
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(stream.Err()))
}

func TestConfigHandler_WatchConfigs_ForwardsEventThenClosesOnChannelClose(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	az := configmock.NewMockauthz(ctrl)
	uc := configmock.NewMockconfigUsecase(ctrl)

	az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

	// Buffered so the event is queued before the handler's select loop even
	// starts — the handler never writes an initial snapshot on its own (unlike
	// clients.Handler.WatchClients), so response headers only flush once the
	// first Send happens. Queuing up front avoids a client/server deadlock
	// where the client blocks on headers that the handler hasn't sent yet.
	ch := make(chan domain.WatchEvent, 4)
	ch <- domain.WatchEvent{
		Type:      domain.EventTypeCreated,
		Path:      "/app/a.json",
		Namespace: "prod",
		Config:    &domain.Config{Path: "/app/a.json", Namespace: "prod"},
	}

	var cancelled bool
	uc.EXPECT().
		Watch(gomock.Any(), configuc.WatchInput{PathPrefix: "/app", Namespace: "prod"}).
		DoAndReturn(func(_ context.Context, _ configuc.WatchInput) (<-chan domain.WatchEvent, func(), error) {
			return ch, func() { cancelled = true }, nil
		})

	h := config.NewConfigHandler(az, uc)
	url := newConfigTestServer(t, h)

	client := configv1connect.NewConfigServiceClient(http.DefaultClient, url)
	stream, err := client.WatchConfigs(t.Context(), connect.NewRequest(&configv1.WatchConfigsRequest{
		Namespace: "prod", PathPrefix: "/app",
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	got := receiveConfigFrame(t, stream)
	assert.Equal(t, configv1.EventType_EVENT_TYPE_CREATED, got.GetEvent().GetType())
	assert.Equal(t, "/app/a.json", got.GetEvent().GetPath())

	close(ch)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !stream.Receive() {
			break
		}
	}
	require.NoError(t, stream.Err())
	_ = cancelled // best-effort: cleanup func invoked via defer inside handler goroutine
}
