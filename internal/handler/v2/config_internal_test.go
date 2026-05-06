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

// -----------------------------------------------------------------------------
// GetConfig
// -----------------------------------------------------------------------------

func TestConfigHandler_GetConfig_Success(t *testing.T) {
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
	assert.Equal(t, "prod", resp.Msg.GetConfig().GetNamespace())
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

// -----------------------------------------------------------------------------
// CreateConfig
// -----------------------------------------------------------------------------

func TestConfigHandler_CreateConfig_Success(t *testing.T) {
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

func TestConfigHandler_CreateConfig_NamespaceNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockcreateEnforcer(ctrl)
	nsChecker := mock_config.NewMockcreateNSChecker(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(nil, domain.ErrNotFound)

	h := &ConfigHandler{create: configuc.NewCreateUseCase(enforcer, nil, nil, nil, nsChecker, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.CreateConfig(ctx, connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// DeleteConfig
// -----------------------------------------------------------------------------

func TestConfigHandler_DeleteConfig_Success(t *testing.T) {
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
