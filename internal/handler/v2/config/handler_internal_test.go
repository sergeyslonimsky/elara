package config

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

type mocks struct {
	enforcer          *mock_config.Mockenforcer
	storage           *mock_config.Mockstorage
	watcher           *mock_config.Mockwatcher
	namespaceProvider *mock_config.MocknamespaceProvider
	schemaValidator   *mock_config.MockschemaValidator
}

func setupHandler(t *testing.T) (*Handler, *mocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := &mocks{
		enforcer:          mock_config.NewMockenforcer(ctrl),
		storage:           mock_config.NewMockstorage(ctrl),
		watcher:           mock_config.NewMockwatcher(ctrl),
		namespaceProvider: mock_config.NewMocknamespaceProvider(ctrl),
		schemaValidator:   mock_config.NewMockschemaValidator(ctrl),
	}

	svc := configuc.New(m.enforcer, m.storage, m.watcher, m.namespaceProvider, m.schemaValidator)
	h := New(svc)

	return h, m
}

func TestConfigHandler_GetConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	resp, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_GetConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)
	ctx := t.Context() // no claims

	_, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_Forbidden(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, nil)

	_, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(nil, domain.ErrNotFound)

	_, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_CreateConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.namespaceProvider.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	m.schemaValidator.EXPECT().Validate(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	m.storage.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	m.namespaceProvider.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
	m.watcher.EXPECT().NotifyCreated(gomock.Any(), gomock.Any())

	resp, err := h.CreateConfig(ctx, connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_CreateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.CreateConfig(t.Context(), connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(&domain.Config{Format: domain.FormatJSON}, nil)
	m.schemaValidator.EXPECT().Validate(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	m.storage.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	m.namespaceProvider.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
	m.watcher.EXPECT().NotifyUpdated(gomock.Any(), gomock.Any())

	resp, err := h.UpdateConfig(ctx, connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_UpdateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.UpdateConfig(t.Context(), connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(nil, domain.ErrNotFound)

	_, err := h.UpdateConfig(ctx, connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(1), nil)
	m.watcher.EXPECT().NotifyDeleted(gomock.Any(), "/a.json", "prod", int64(1))

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_DeleteConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(0), domain.ErrNotFound)

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig_Locked(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(0), domain.ErrLocked)

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestConfigHandler_ListConfigs(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.storage.EXPECT().ListSummariesByPrefix(gomock.Any(), "/", "prod").Return([]*domain.ConfigSummary{
		{Path: "/a.json", Namespace: "prod"},
	}, nil)
	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

	resp, err := h.ListConfigs(ctx, connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_ListConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.ListConfigs(t.Context(), connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigHistory(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.storage.EXPECT().
		GetConfigHistory(gomock.Any(), "/a.json", "prod", gomock.Any()).
		Return([]*domain.HistoryEntry{{Revision: 1}}, nil)

	resp, err := h.GetConfigHistory(
		ctx,
		connect.NewRequest(&configv1.GetConfigHistoryRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_GetConfigHistory_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.GetConfigHistory(t.Context(), connect.NewRequest(&configv1.GetConfigHistoryRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_SearchConfigs(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.storage.EXPECT().
		SearchByPath(gomock.Any(), "app", "prod").
		Return([]*domain.ConfigSummary{{Path: "/app/1.json", Namespace: "prod"}}, nil)
	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

	resp, err := h.SearchConfigs(
		ctx,
		connect.NewRequest(&configv1.SearchConfigsRequest{Query: "app", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetResults(), 1)
}

func TestConfigHandler_SearchConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.SearchConfigs(t.Context(), connect.NewRequest(&configv1.SearchConfigsRequest{
		Query: "app", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_CopyConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "ns2", "config", "write").Return(true, nil)
	m.storage.EXPECT().
		Get(gomock.Any(), "/src.json", "ns1").
		Return(&domain.Config{Path: "/src.json", Namespace: "ns1"}, nil)
	m.namespaceProvider.EXPECT().Get(gomock.Any(), "ns2").Return(&domain.Namespace{Name: "ns2"}, nil)
	m.storage.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	m.namespaceProvider.EXPECT().UpdateTimestamp(gomock.Any(), "ns2").Return(nil)
	m.watcher.EXPECT().NotifyCreated(gomock.Any(), gomock.Any())

	req := connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath:           "/src.json",
		SourceNamespace:      "ns1",
		DestinationPath:      "/dest.json",
		DestinationNamespace: "ns2",
	})

	resp, err := h.CopyConfig(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "/dest.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_CopyConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.CopyConfig(t.Context(), connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath: "/src.json", SourceNamespace: "ns1", DestinationPath: "/dst.json", DestinationNamespace: "ns2",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_ValidateConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.schemaValidator.EXPECT().Validate(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)

	resp, err := h.ValidateConfig(ctx, connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.GetResult().GetValid())
}

func TestConfigHandler_ValidateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.ValidateConfig(t.Context(), connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigDiff(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.storage.EXPECT().
		GetAtRevision(gomock.Any(), "/a.json", "prod", int64(1)).
		Return(&domain.HistoryEntry{Revision: 1, Content: "v1"}, nil)
	m.storage.EXPECT().
		GetAtRevision(gomock.Any(), "/a.json", "prod", int64(2)).
		Return(&domain.HistoryEntry{Revision: 2, Content: "v2"}, nil)

	resp, err := h.GetConfigDiff(ctx, connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Msg.GetFromRevision())
}

func TestConfigHandler_GetConfigDiff_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.GetConfigDiff(t.Context(), connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().LockConfig(gomock.Any(), "prod", "/a.json").Return(nil)
	m.storage.EXPECT().
		Get(gomock.Any(), "/a.json", "prod").
		Return(&domain.Config{Path: "/a.json", Namespace: "prod", Locked: true}, nil)
	m.watcher.EXPECT().NotifyConfigLocked(gomock.Any(), gomock.Any())

	_, err := h.LockConfig(ctx, connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_LockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.LockConfig(t.Context(), connect.NewRequest(&configv1.LockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().LockConfig(gomock.Any(), "prod", "/a.json").Return(domain.ErrNotFound)

	_, err := h.LockConfig(ctx, connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_UnlockConfig(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	m.storage.EXPECT().UnlockConfig(gomock.Any(), "prod", "/a.json").Return(nil)
	m.storage.EXPECT().
		Get(gomock.Any(), "/a.json", "prod").
		Return(&domain.Config{Path: "/a.json", Namespace: "prod", Locked: false}, nil)
	m.watcher.EXPECT().NotifyConfigUnlocked(gomock.Any(), gomock.Any())

	_, err := h.UnlockConfig(ctx, connect.NewRequest(&configv1.UnlockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_UnlockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.UnlockConfig(t.Context(), connect.NewRequest(&configv1.UnlockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigAtRevision(t *testing.T) {
	t.Parallel()

	h, m := setupHandler(t)
	ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})

	m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	m.storage.EXPECT().
		GetAtRevision(gomock.Any(), "/a.json", "prod", int64(1)).
		Return(&domain.HistoryEntry{Revision: 1, Content: "v1"}, nil)

	resp, err := h.GetConfigAtRevision(ctx, connect.NewRequest(&configv1.GetConfigAtRevisionRequest{
		Path:      "/a.json",
		Namespace: "prod",
		Revision:  1,
	}))
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Msg.GetEntry().GetRevision())
}

func TestConfigHandler_GetConfigAtRevision_Unauthorized(t *testing.T) {
	t.Parallel()

	h, _ := setupHandler(t)

	_, err := h.GetConfigAtRevision(t.Context(), connect.NewRequest(&configv1.GetConfigAtRevisionRequest{
		Path:      "/a.json",
		Namespace: "prod",
		Revision:  1,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
