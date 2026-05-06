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

	getUC := configuc.NewGetUseCase(enforcer, getter)
	h := &ConfigHandler{get: getUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
	getter.EXPECT().Get(gomock.Any(), "/a.json", "prod").Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	req := connect.NewRequest(&configv1.GetConfigRequest{
		Path:      "/a.json",
		Namespace: "prod",
	})

	resp, err := h.GetConfig(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
	assert.Equal(t, "prod", resp.Msg.GetConfig().GetNamespace())
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

	createUC := configuc.NewCreateUseCase(enforcer, configs, watch, namespaces, nsChecker, schemaValidator)
	h := &ConfigHandler{create: createUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	schemaValidator.EXPECT().Execute(gomock.Any(), "prod", "/a.json", gomock.Any(), domain.FormatJSON).Return(nil)
	configs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	namespaces.EXPECT().UpdateTimestamp(gomock.Any(), "prod").Return(nil)
	watch.EXPECT().NotifyCreated(gomock.Any(), gomock.Any())

	req := connect.NewRequest(&configv1.CreateConfigRequest{
		Path:      "/a.json",
		Namespace: "prod",
		Content:   "{}",
		Format:    configv1.Format_FORMAT_JSON,
	})

	resp, err := h.CreateConfig(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_DeleteConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_config.NewMockdeleteEnforcer(ctrl)
	configs := mock_config.NewMockconfigDeleter(ctrl)
	watch := mock_config.NewMockdeleteWatchNotifier(ctrl)

	delUC := configuc.NewDeleteUseCase(enforcer, configs, watch)
	h := &ConfigHandler{del: delUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
	configs.EXPECT().Delete(gomock.Any(), "/a.json", "prod").Return(int64(1), nil)
	watch.EXPECT().NotifyDeleted(gomock.Any(), "/a.json", "prod", int64(1))

	req := connect.NewRequest(&configv1.DeleteConfigRequest{
		Path:      "/a.json",
		Namespace: "prod",
	})

	_, err := h.DeleteConfig(ctx, req)
	require.NoError(t, err)
}
