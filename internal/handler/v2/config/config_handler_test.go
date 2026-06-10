package config_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	configmock "github.com/sergeyslonimsky/elara/internal/handler/v2/config/mocks"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func setupHandler(
	t *testing.T,
) (*config.ConfigHandler, *configmock.Mockauthz, *configmock.MockconfigUsecase) {
	t.Helper()

	ctrl := gomock.NewController(t)
	az := configmock.NewMockauthz(ctrl)
	uc := configmock.NewMockconfigUsecase(ctrl)
	h := config.NewConfigHandler(az, uc)

	return h, az, uc
}

func TestConfigHandler_GetConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)

	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		Get(gomock.Any(), configuc.GetInput{Path: "/a.json", Namespace: "prod"}).
		Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	resp, err := h.GetConfig(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_GetConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.GetConfig(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_Forbidden(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrForbidden)

	_, err := h.GetConfig(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestConfigHandler_GetConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.GetConfig(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_CreateConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)

	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().
		Create(gomock.Any(), &domain.Config{
			Path:      "/a.json",
			Namespace: "prod",
			Content:   "{}",
			Format:    domain.FormatJSON,
		}).
		Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	resp, err := h.CreateConfig(t.Context(), connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_CreateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.CreateConfig(t.Context(), connect.NewRequest(&configv1.CreateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)

	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(&domain.Config{Path: "/a.json", Namespace: "prod"}, nil)

	resp, err := h.UpdateConfig(t.Context(), connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.Equal(t, "/a.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_UpdateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.UpdateConfig(t.Context(), connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_UpdateConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil, domain.ErrNotFound)

	_, err := h.UpdateConfig(t.Context(), connect.NewRequest(&configv1.UpdateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().
		Delete(gomock.Any(), configuc.DeleteInput{Path: "/a.json", Namespace: "prod"}).
		Return(nil)

	_, err := h.DeleteConfig(
		t.Context(),
		connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
}

func TestConfigHandler_DeleteConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.DeleteConfig(
		t.Context(),
		connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

	_, err := h.DeleteConfig(
		t.Context(),
		connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_DeleteConfig_Locked(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(domain.ErrLocked)

	_, err := h.DeleteConfig(
		t.Context(),
		connect.NewRequest(&configv1.DeleteConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestConfigHandler_ListConfigs(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		List(gomock.Any(), gomock.Any()).
		Return(&configuc.ListResult{
			Entries: []*configuc.DirectoryEntry{
				{Name: "a.json", FullPath: "/a.json", IsFile: true, Format: domain.FormatJSON},
			},
			Total: 1,
		}, nil)

	resp, err := h.ListConfigs(
		t.Context(),
		connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_ListConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.ListConfigs(
		t.Context(),
		connect.NewRequest(&configv1.ListConfigsRequest{Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigHistory(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		History(gomock.Any(), gomock.Any()).
		Return([]*domain.HistoryEntry{{Revision: 1}}, nil)

	resp, err := h.GetConfigHistory(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigHistoryRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetEntries(), 1)
}

func TestConfigHandler_GetConfigHistory_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.GetConfigHistory(t.Context(), connect.NewRequest(&configv1.GetConfigHistoryRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_SearchConfigs(t *testing.T) {
	t.Parallel()

	// SearchConfigs has NO authz.Require call — scoping done inside usecase via pdp.
	h, _, uc := setupHandler(t)
	uc.EXPECT().
		Search(gomock.Any(), gomock.Any()).
		Return(&configuc.SearchResult{
			Results: []*domain.ConfigSummary{{Path: "/app/1.json", Namespace: "prod"}},
			Total:   1,
		}, nil)

	resp, err := h.SearchConfigs(
		t.Context(),
		connect.NewRequest(&configv1.SearchConfigsRequest{Query: "app", Namespace: "prod"}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetResults(), 1)
}

func TestConfigHandler_SearchConfigs_UsecaseError(t *testing.T) {
	t.Parallel()

	h, _, uc := setupHandler(t)
	uc.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, domain.ErrUnauthorized)

	_, err := h.SearchConfigs(t.Context(), connect.NewRequest(&configv1.SearchConfigsRequest{
		Query: "app", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_CopyConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)

	gomock.InOrder(
		az.EXPECT().
			RequireNamespace(gomock.Any(), domain.ActionRead, "ns1").
			Return(nil),
		az.EXPECT().
			RequireNamespace(gomock.Any(), domain.ActionWrite, "ns2").
			Return(nil),
	)
	uc.EXPECT().
		Copy(gomock.Any(), configuc.CopyInput{
			SourcePath:      "/src.json",
			SourceNamespace: "ns1",
			DestPath:        "/dest.json",
			DestNamespace:   "ns2",
		}).
		Return(&domain.Config{Path: "/dest.json", Namespace: "ns2"}, nil)

	req := connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath:           "/src.json",
		SourceNamespace:      "ns1",
		DestinationPath:      "/dest.json",
		DestinationNamespace: "ns2",
	})

	resp, err := h.CopyConfig(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, "/dest.json", resp.Msg.GetConfig().GetPath())
}

func TestConfigHandler_CopyConfig_SourceForbidden(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "ns1").
		Return(domain.ErrForbidden)

	_, err := h.CopyConfig(t.Context(), connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath: "/src.json", SourceNamespace: "ns1", DestinationPath: "/dst.json", DestinationNamespace: "ns2",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestConfigHandler_CopyConfig_DestForbidden(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	gomock.InOrder(
		az.EXPECT().
			RequireNamespace(gomock.Any(), domain.ActionRead, "ns1").
			Return(nil),
		az.EXPECT().
			RequireNamespace(gomock.Any(), domain.ActionWrite, "ns2").
			Return(domain.ErrForbidden),
	)

	_, err := h.CopyConfig(t.Context(), connect.NewRequest(&configv1.CopyConfigRequest{
		SourcePath: "/src.json", SourceNamespace: "ns1", DestinationPath: "/dst.json", DestinationNamespace: "ns2",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestConfigHandler_ValidateConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		Return(&domain.ValidationResult{Valid: true, DetectedFormat: domain.FormatJSON}, nil)

	resp, err := h.ValidateConfig(t.Context(), connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}", Format: configv1.Format_FORMAT_JSON,
	}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.GetResult().GetValid())
}

func TestConfigHandler_ValidateConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.ValidateConfig(t.Context(), connect.NewRequest(&configv1.ValidateConfigRequest{
		Path: "/a.json", Namespace: "prod", Content: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigDiff(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		Diff(gomock.Any(), configuc.DiffInput{Path: "/a.json", Namespace: "prod", V1: 1, V2: 2}).
		Return(&domain.ConfigDiff{FromRevision: 1, ToRevision: 2, FromContent: "v1", ToContent: "v2"}, nil)

	resp, err := h.GetConfigDiff(t.Context(), connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Msg.GetFromRevision())
}

func TestConfigHandler_GetConfigDiff_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.GetConfigDiff(t.Context(), connect.NewRequest(&configv1.GetConfigDiffRequest{
		Path: "/a.json", Namespace: "prod", FromRevision: 1, ToRevision: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().
		Lock(gomock.Any(), configuc.LockInput{Namespace: "prod", Path: "/a.json"}).
		Return(nil)

	_, err := h.LockConfig(
		t.Context(),
		connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
}

func TestConfigHandler_LockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.LockConfig(t.Context(), connect.NewRequest(&configv1.LockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_LockConfig_NotFound(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().Lock(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

	_, err := h.LockConfig(
		t.Context(),
		connect.NewRequest(&configv1.LockConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConfigHandler_UnlockConfig(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(nil)
	uc.EXPECT().
		Unlock(gomock.Any(), configuc.UnlockInput{Namespace: "prod", Path: "/a.json"}).
		Return(nil)

	_, err := h.UnlockConfig(
		t.Context(),
		connect.NewRequest(&configv1.UnlockConfigRequest{Path: "/a.json", Namespace: "prod"}),
	)
	require.NoError(t, err)
}

func TestConfigHandler_UnlockConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.UnlockConfig(t.Context(), connect.NewRequest(&configv1.UnlockConfigRequest{
		Path: "/a.json", Namespace: "prod",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestConfigHandler_GetConfigAtRevision(t *testing.T) {
	t.Parallel()

	h, az, uc := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(nil)
	uc.EXPECT().
		GetAtRevision(gomock.Any(), configuc.GetAtRevisionInput{Path: "/a.json", Namespace: "prod", Revision: 1}).
		Return(&domain.HistoryEntry{Revision: 1, Content: "v1"}, nil)

	resp, err := h.GetConfigAtRevision(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigAtRevisionRequest{
			Path:      "/a.json",
			Namespace: "prod",
			Revision:  1,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Msg.GetEntry().GetRevision())
}

func TestConfigHandler_GetConfigAtRevision_Unauthorized(t *testing.T) {
	t.Parallel()

	h, az, _ := setupHandler(t)
	az.EXPECT().
		RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
		Return(domain.ErrUnauthorized)

	_, err := h.GetConfigAtRevision(
		t.Context(),
		connect.NewRequest(&configv1.GetConfigAtRevisionRequest{
			Path:      "/a.json",
			Namespace: "prod",
			Revision:  1,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
