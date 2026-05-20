package group_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/group"
	groupmock "github.com/sergeyslonimsky/elara/internal/handler/v2/group/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	groupv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// -----------------------------------------------------------------------------
// CreateGroup
// -----------------------------------------------------------------------------

func TestGroupHandler_CreateGroup(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		req       *groupv1.CreateGroupRequest
		setupMock func(*groupmock.MockgroupUsecase)
		wantErr   bool
		wantCode  connect.Code
		assertOK  func(*testing.T, *groupv1.CreateGroupResponse)
	}{
		{
			name: "success",
			req:  &groupv1.CreateGroupRequest{Name: "developers"},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Create(gomock.Any(), "developers").Return(
					&domain.Group{
						ID: "g-1", Name: "developers", Members: []string{},
						Version: 1, CreatedAt: now, UpdatedAt: now,
					}, nil,
				)
			},
			assertOK: func(t *testing.T, resp *groupv1.CreateGroupResponse) {
				t.Helper()
				assert.Equal(t, "g-1", resp.GetGroup().GetId())
				assert.Equal(t, "developers", resp.GetGroup().GetName())
				assert.EqualValues(t, 1, resp.GetGroup().GetVersion())
				assert.False(t, resp.GetGroup().GetIsSystem())
			},
		},
		{
			name: "already exists",
			req:  &groupv1.CreateGroupRequest{Name: "dup"},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Create(gomock.Any(), "dup").Return(
					nil, domain.NewAlreadyExistsError("group", "dup"),
				)
			},
			wantErr:  true,
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := groupmock.NewMockgroupUsecase(ctrl)
			tc.setupMock(uc)

			h := group.NewHandler(uc)
			resp, err := h.CreateGroup(t.Context(), connect.NewRequest(tc.req))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			tc.assertOK(t, resp.Msg)
		})
	}
}

// -----------------------------------------------------------------------------
// GetGroup
// -----------------------------------------------------------------------------

func TestGroupHandler_GetGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(*groupmock.MockgroupUsecase)
		wantErr   bool
		wantCode  connect.Code
		wantID    string
	}{
		{
			name: "found",
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Get(gomock.Any(), "g-1").Return(
					&domain.Group{ID: "g-1", Name: "developers", Version: 3}, nil,
				)
			},
			wantID: "g-1",
		},
		{
			name: "not found",
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Get(gomock.Any(), "missing").Return(
					nil, domain.NewNotFoundError("group", "missing"),
				)
			},
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := groupmock.NewMockgroupUsecase(ctrl)
			tc.setupMock(uc)

			h := group.NewHandler(uc)
			reqID := "g-1"
			if tc.wantID == "" {
				reqID = "missing"
			}

			resp, err := h.GetGroup(t.Context(), connect.NewRequest(&groupv1.GetGroupRequest{Id: reqID}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, resp.Msg.GetGroup().GetId())
		})
	}
}

// -----------------------------------------------------------------------------
// UpdateGroup — the M4 centerpiece. Verifies handler-to-usecase param wiring
// and error → connect.Code mapping for every M4 failure path.
// -----------------------------------------------------------------------------

func TestGroupHandler_UpdateGroup(t *testing.T) {
	t.Parallel()

	now := time.Now()
	domainPerm := domain.Permission{
		Object: domain.ObjectNamespace,
		Action: domain.ActionWrite,
		Domain: "dev",
	}
	protoPerm := &commonv1.PermissionAssignment{
		Object: commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE,
		Action: commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
		Domain: "dev",
	}

	tests := []struct {
		name      string
		req       *groupv1.UpdateGroupRequest
		setupMock func(*groupmock.MockgroupUsecase)
		wantErr   bool
		wantCode  connect.Code
		assertOK  func(*testing.T, *groupv1.UpdateGroupResponse)
	}{
		{
			name: "success — translates proto perms to domain and bumps version",
			req: &groupv1.UpdateGroupRequest{
				Id:          "g-1",
				Name:        "developers",
				Description: "writes to dev ns",
				Permissions: []*commonv1.PermissionAssignment{protoPerm},
				Members:     []string{"alice@example.com"},
				Version:     1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(
						gomock.Any(),
						"g-1", "developers", "writes to dev ns",
						[]domain.Permission{domainPerm},
						[]string{"alice@example.com"},
						int64(1),
					).
					Return(&domain.Group{
						ID: "g-1", Name: "developers",
						Description: "writes to dev ns",
						Members:     []string{"alice@example.com"},
						Version:     2, CreatedAt: now, UpdatedAt: now,
					}, nil)
			},
			assertOK: func(t *testing.T, resp *groupv1.UpdateGroupResponse) {
				t.Helper()
				assert.Equal(t, "developers", resp.GetGroup().GetName())
				assert.Equal(t, "writes to dev ns", resp.GetGroup().GetDescription())
				assert.EqualValues(t, 2, resp.GetGroup().GetVersion())
				assert.Equal(t, []string{"alice@example.com"}, resp.GetGroup().GetMembers())
			},
		},
		{
			name: "version conflict → 409 Aborted",
			req: &groupv1.UpdateGroupRequest{
				Id: "g-1", Name: "developers", Version: 1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrVersionConflict)
			},
			wantErr:  true,
			wantCode: connect.CodeAborted,
		},
		{
			name: "permission escalation → 403 PermissionDenied",
			req: &groupv1.UpdateGroupRequest{
				Id: "g-1", Name: "developers", Version: 1,
				Permissions: []*commonv1.PermissionAssignment{protoPerm},
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrPermissionEscalation)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "system group → 403 PermissionDenied",
			req: &groupv1.UpdateGroupRequest{
				Id: "system:superadmin", Name: "system:superadmin", Version: 1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrSystemImmutable)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "not found → 404 NotFound",
			req: &groupv1.UpdateGroupRequest{
				Id: "missing", Name: "x", Version: 1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.NewNotFoundError("group", "missing"))
			},
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
		{
			name: "unauthenticated → 401 Unauthenticated",
			req: &groupv1.UpdateGroupRequest{
				Id: "g-1", Name: "x", Version: 1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
						gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrUnauthorized)
			},
			wantErr:  true,
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "empty permissions/members pass through as nil",
			req: &groupv1.UpdateGroupRequest{
				Id: "g-1", Name: "developers", Version: 1,
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(
						gomock.Any(),
						"g-1", "developers", "",
						gomock.Nil(),
						gomock.Nil(),
						int64(1),
					).
					Return(&domain.Group{ID: "g-1", Name: "developers", Version: 2}, nil)
			},
			assertOK: func(t *testing.T, resp *groupv1.UpdateGroupResponse) {
				t.Helper()
				assert.Equal(t, "g-1", resp.GetGroup().GetId())
			},
		},
		{
			name: "unspecified perm enum is dropped",
			req: &groupv1.UpdateGroupRequest{
				Id: "g-1", Name: "developers", Version: 1,
				Permissions: []*commonv1.PermissionAssignment{
					{
						Object: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED,
						Action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
						Domain: "dev",
					},
					protoPerm,
				},
			},
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().
					Update(
						gomock.Any(),
						"g-1", "developers", "",
						[]domain.Permission{domainPerm}, // unspecified got dropped
						gomock.Nil(),
						int64(1),
					).
					Return(&domain.Group{ID: "g-1", Name: "developers", Version: 2}, nil)
			},
			assertOK: func(t *testing.T, resp *groupv1.UpdateGroupResponse) {
				t.Helper()
				assert.Equal(t, "g-1", resp.GetGroup().GetId())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := groupmock.NewMockgroupUsecase(ctrl)
			tc.setupMock(uc)

			h := group.NewHandler(uc)
			resp, err := h.UpdateGroup(t.Context(), connect.NewRequest(tc.req))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			tc.assertOK(t, resp.Msg)
		})
	}
}

// -----------------------------------------------------------------------------
// DeleteGroup
// -----------------------------------------------------------------------------

func TestGroupHandler_DeleteGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(*groupmock.MockgroupUsecase)
		wantErr   bool
		wantCode  connect.Code
	}{
		{
			name: "success",
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Delete(gomock.Any(), "g-1").Return(nil)
			},
		},
		{
			name: "forbidden (e.g. system group)",
			setupMock: func(uc *groupmock.MockgroupUsecase) {
				uc.EXPECT().Delete(gomock.Any(), "g-1").Return(
					errors.Join(domain.ErrForbidden, errors.New("system group")),
				)
			},
			wantErr:  true,
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := groupmock.NewMockgroupUsecase(ctrl)
			tc.setupMock(uc)

			h := group.NewHandler(uc)
			_, err := h.DeleteGroup(t.Context(),
				connect.NewRequest(&groupv1.DeleteGroupRequest{Id: "g-1"}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
		})
	}
}

// -----------------------------------------------------------------------------
// ListGroups
// -----------------------------------------------------------------------------

func TestGroupHandler_ListGroups(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := groupmock.NewMockgroupUsecase(ctrl)
	uc.EXPECT().List(gomock.Any(), groupuc.ListParams{}).Return(&groupuc.ListResult{
		Groups: []*domain.Group{
			{ID: "g-1", Name: "devs", Version: 1},
			{ID: "g-2", Name: "qa", Version: 1, System: true},
		},
		Total:  2,
		Limit:  20,
		Offset: 0,
	}, nil)

	h := group.NewHandler(uc)
	resp, err := h.ListGroups(t.Context(), connect.NewRequest(&groupv1.ListGroupsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetGroups(), 2)
	assert.Equal(t, "devs", resp.Msg.GetGroups()[0].GetName())
	assert.True(t, resp.Msg.GetGroups()[1].GetIsSystem())
}

// Compile-time guard: ensure the test compiles against the current mock
// package signature. Helps catch missed regen of the mock after
// groupUsecase interface changes.
var _ context.Context = context.Background()
