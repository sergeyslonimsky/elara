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
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestConfigHandler_GetConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockgetEnforcer(ctrl)
	getter := mock_config.NewMockconfigGetter(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	getter.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	h := &ConfigHandler{get: configuc.NewGetUseCase(enforcer, getter)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_GetConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{get: configuc.NewGetUseCase(nil, nil)}

	_, err := h.GetConfig(
		context.Background(),
		connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_Forbidden(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockgetEnforcer(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(false, nil)

	h := &ConfigHandler{get: configuc.NewGetUseCase(enforcer, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockgetEnforcer(ctrl)
	getter := mock_config.NewMockconfigGetter(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	getter.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(nil, domain.ErrNotFound)

	h := &ConfigHandler{get: configuc.NewGetUseCase(enforcer, getter)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.GetConfig(ctx, connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_CreateConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockcreateEnforcer(ctrl)
	configs := mock_config.NewMockconfigCreator(ctrl)
	nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
	schemaValidator := mock_config.NewMockcreateSchemaValidator(ctrl)
	namespaces := mock_config.NewMockcreateNSTimestampUpdater(ctrl)
	watch := mock_config.NewMockcreateWatchNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	schemaValidator.EXPECT().Execute(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	configs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	namespaces.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
	watch.EXPECT().NotifyCreated(gomock.Any(), gomock.Any())

	h := &ConfigHandler{
		create: configuc.NewCreateUseCase(enforcer, configs, watch, namespaces, nsChecker, schemaValidator),
	}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.CreateConfig(ctx, connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_CreateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{create: configuc.NewCreateUseCase(nil, nil, nil, nil, nil, nil)}

	_, err := h.CreateConfig(context.Background(), connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockupdateEnforcer(ctrl)
	configs := mock_config.NewMockconfigUpdater(ctrl)
	getter := mock_config.NewMockupdateConfigGetter(ctrl)
	schemaValidator := mock_config.NewMockupdateSchemaValidator(ctrl)
	namespaces := mock_config.NewMockupdateNSTimestampUpdater(ctrl)
	watch := mock_config.NewMockupdateWatchNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	schemaValidator.EXPECT().Execute(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	configs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	namespaces.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
	watch.EXPECT().NotifyUpdated(gomock.Any(), gomock.Any())

	h := &ConfigHandler{
		update: configuc.NewUpdateUseCase(enforcer, configs, getter, watch, namespaces, schemaValidator),
	}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.UpdateConfig(ctx, connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_UpdateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{update: configuc.NewUpdateUseCase(nil, nil, nil, nil, nil, nil)}

	_, err := h.UpdateConfig(context.Background(), connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockupdateEnforcer(ctrl)
	getter := mock_config.NewMockupdateConfigGetter(ctrl)
	configs := mock_config.NewMockconfigUpdater(ctrl)
	schemaValidator := mock_config.NewMockupdateSchemaValidator(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	// No Format in request — usecase calls getter to detect existing format.
	getter.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(&domain.Config{Format: domain.FormatJSON}, nil)
	schemaValidator.EXPECT().Execute(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	configs.EXPECT().Update(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

	h := &ConfigHandler{update: configuc.NewUpdateUseCase(enforcer, configs, getter, nil, nil, schemaValidator)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.UpdateConfig(ctx, connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
	configs := mock_config.NewMockconfigDeleter(ctrl)
	watch := mock_config.NewMockdeleteWatchNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	configs.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(1), nil)
	watch.EXPECT().NotifyDeleted(gomock.Any(), "/a.json", "prod", int64(1))

	h := &ConfigHandler{del: configuc.NewDeleteUseCase(enforcer, configs, watch)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_DeleteConfig_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
	configs := mock_config.NewMockconfigDeleter(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	configs.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(0), domain.ErrNotFound)

	h := &ConfigHandler{del: configuc.NewDeleteUseCase(enforcer, configs, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig_Locked(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
	configs := mock_config.NewMockconfigDeleter(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	configs.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(0), domain.ErrLocked)

	h := &ConfigHandler{del: configuc.NewDeleteUseCase(enforcer, configs, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.DeleteConfig(ctx, connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestConfigHandler_ListConfigs(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMocklistEnforcer(ctrl)
	lister := mock_config.NewMockconfigLister(ctrl)

	lister.EXPECT().ListSummariesByPrefix(gomock.Any(), "/", "prod").Return([]*domain.ConfigSummary{
		{Path: "/a.json", Namespace: "prod"},
	}, nil)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

	h := &ConfigHandler{list: configuc.NewListUseCase(enforcer, lister)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.ListConfigs(ctx, connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_ListConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{list: configuc.NewListUseCase(nil, nil)}

	_, err := h.ListConfigs(context.Background(), connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigHistory(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockhistoryEnforcer(ctrl)
	reader := mock_config.NewMockconfigHistoryReader(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	reader.EXPECT().
		GetConfigHistory(gomock.Any(), "/a.json", "prod", gomock.Any()).
		Return([]*domain.HistoryEntry{{Revision: 1}}, nil)

	h := &ConfigHandler{history: configuc.NewHistoryUseCase(enforcer, reader)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.GetConfigHistory(
		ctx,
		connect.NewRequest(&configv1.GetConfigHistoryRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_GetConfigHistory_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{history: configuc.NewHistoryUseCase(nil, nil)}

	_, err := h.GetConfigHistory(context.Background(), connect.NewRequest(&configv1.GetConfigHistoryRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_SearchConfigs(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMocksearchEnforcer(ctrl)
	searcher := mock_config.NewMockconfigSearcher(ctrl)

	searcher.EXPECT().
		SearchByPath(gomock.Any(), "app", "prod").
		Return([]*domain.ConfigSummary{{Path: "/app/1.json", Namespace: "prod"}}, nil)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

	h := &ConfigHandler{search: configuc.NewSearchUseCase(enforcer, searcher)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.SearchConfigs(
		ctx,
		connect.NewRequest(&configv1.SearchConfigsRequest{Query: "app", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetResults(), 1)
}

func TestConfigHandler_SearchConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{search: configuc.NewSearchUseCase(nil, nil)}

	_, err := h.SearchConfigs(context.Background(), connect.NewRequest(&configv1.SearchConfigsRequest{
		Query: "app", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_CopyConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	enforcer := mock_config.NewMockcopyEnforcer(ctrl)
	getter := mock_config.NewMockcopyConfigGetter(ctrl)
	creator := mock_config.NewMockcopyConfigCreator(ctrl)
	nsChecker := mock_config.NewMockcopyNSChecker(ctrl)
	timestampUpdater := mock_config.NewMockcopyNSTimestampUpdater(ctrl)
	watch := mock_config.NewMockcopyWatchNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "ns2", "config", "write").Return(true, nil)
	getter.EXPECT().
		Get(gomock.Any(), "/src.json", "ns1").
		Return(&domain.Config{Path: "/src.json", Namespace: "ns1"}, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "ns2").Return(&domain.Namespace{Name: "ns2"}, nil)
	creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	timestampUpdater.EXPECT().UpdateTimestamp(gomock.Any(), "ns2").Return(nil)
	watch.EXPECT().NotifyCreated(gomock.Any(), gomock.Any())

	copyUC := configuc.NewCopyUseCase(enforcer, getter, creator, watch, nsChecker, timestampUpdater)
	h := NewConfigHandler(nil, nil, nil, nil, nil, nil, nil, copyUC, nil, nil, nil, nil, nil)
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

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

	h := NewConfigHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		configuc.NewCopyUseCase(nil, nil, nil, nil, nil, nil),
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := h.CopyConfig(context.Background(), connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath: "/src.json", SourceNamespace: "ns1", DestinationPath: "/dst.json", DestinationNamespace: "ns2",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_ValidateConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockvalidateEnforcer(ctrl)
	schema := mock_config.NewMockvalidateSchemaChecker(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	schema.EXPECT().Execute(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)

	h := &ConfigHandler{validate: configuc.NewValidateUseCase(enforcer, schema)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.ValidateConfig(ctx, connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.GetResult().GetValid())
}

func TestConfigHandler_ValidateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{validate: configuc.NewValidateUseCase(nil, nil)}

	_, err := h.ValidateConfig(context.Background(), connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigDiff(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockdiffEnforcer(ctrl)
	reader := mock_config.NewMockconfigDiffReader(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	reader.EXPECT().
		GetAtRevision(gomock.Any(), "/a.json", "prod", int64(1)).
		Return(&domain.HistoryEntry{Revision: 1, Content: "v1"}, nil)
	reader.EXPECT().
		GetAtRevision(gomock.Any(), "/a.json", "prod", int64(2)).
		Return(&domain.HistoryEntry{Revision: 2, Content: "v2"}, nil)

	h := &ConfigHandler{diff: configuc.NewDiffUseCase(enforcer, reader)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.GetConfigDiff(ctx, connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Msg.GetFromRevision())
}

func TestConfigHandler_GetConfigDiff_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{diff: configuc.NewDiffUseCase(nil, nil)}

	_, err := h.GetConfigDiff(context.Background(), connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMocklockEnforcer(ctrl)
	store := mock_config.NewMockLockStore(ctrl)
	notifier := mock_config.NewMockLockNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	store.EXPECT().LockConfig(gomock.Any(), "prod", "/a.json").Return(nil)
	store.EXPECT().
		Get(gomock.Any(), "/a.json", "prod").
		Return(&domain.Config{Path: "/a.json", Namespace: "prod", Locked: true}, nil)
	notifier.EXPECT().NotifyConfigLocked(gomock.Any(), gomock.Any())

	h := &ConfigHandler{lock: configuc.NewLockUseCase(enforcer, store, notifier)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.LockConfig(ctx, connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_LockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{lock: configuc.NewLockUseCase(nil, nil, nil)}

	_, err := h.LockConfig(context.Background(), connect.NewRequest(&configv1.LockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMocklockEnforcer(ctrl)
	store := mock_config.NewMockLockStore(ctrl)
	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	store.EXPECT().LockConfig(gomock.Any(), "prod", "/a.json").Return(domain.ErrNotFound)

	h := &ConfigHandler{lock: configuc.NewLockUseCase(enforcer, store, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.LockConfig(ctx, connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_UnlockConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockunlockEnforcer(ctrl)
	store := mock_config.NewMockUnlockStore(ctrl)
	notifier := mock_config.NewMockUnlockNotifier(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	store.EXPECT().UnlockConfig(gomock.Any(), "prod", "/a.json").Return(nil)
	store.EXPECT().
		Get(gomock.Any(), "/a.json", "prod").
		Return(&domain.Config{Path: "/a.json", Namespace: "prod", Locked: false}, nil)
	notifier.EXPECT().NotifyConfigUnlocked(gomock.Any(), gomock.Any())

	h := &ConfigHandler{unlock: configuc.NewUnlockUseCase(enforcer, store, notifier)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.UnlockConfig(ctx, connect.NewRequest(&configv1.UnlockConfigRequest{Path: "/a.json", Namespace: "prod"}))
	require.NoError(t, err)
}

func TestConfigHandler_UnlockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &ConfigHandler{unlock: configuc.NewUnlockUseCase(nil, nil, nil)}

	_, err := h.UnlockConfig(context.Background(), connect.NewRequest(&configv1.UnlockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
