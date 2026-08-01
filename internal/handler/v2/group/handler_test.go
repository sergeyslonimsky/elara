package group_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/group"
	group_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/group/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// withActor returns a context carrying claims so authctx.AuthInfoFromContext
// resolves a valid domain.AuthInfo.
func withActor(ctx context.Context, email string) context.Context {
	return authctx.WithClaims(ctx, &authctx.Claims{Email: email})
}

func TestHandler_CreateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(actor, domain.ObjectGroup, domain.ActionCreate, domain.DomainAll).Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Create(ctx, actor, groupuc.CreateData{
					Name:        "team-a",
					Description: "desc",
				}).Return(&groupuc.CreateResult{
					Group:          &domain.Group{Name: "team-a"},
					VisibleMembers: []string{"m1"},
				}, nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "no auth context returns unauthenticated",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				return group.NewHandler(group_mock.NewMockauthz(ctrl), group_mock.NewMockgroupUsecase(ctrl)), ctx
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "authz denies",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionCreate, domain.DomainAll).
					Return(domain.ErrForbidden)

				return group.NewHandler(az, group_mock.NewMockgroupUsecase(ctrl)), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionCreate, domain.DomainAll).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrAlreadyExists)

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.CreateGroup(ctx, connect.NewRequest(&v1.CreateGroupRequest{
				Name:        "team-a",
				Description: "desc",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "team-a", resp.Msg.GetGroup().GetName())
			assert.Equal(t, []string{"m1"}, resp.Msg.GetVisibleMembers())
		})
	}
}

func TestHandler_GetGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(actor, domain.ObjectGroup, domain.ActionRead, domain.GroupResource("team-a")).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Get(ctx, actor, "team-a").Return(&groupuc.GetResult{
					Group:          &domain.Group{Name: "team-a"},
					VisibleMembers: []string{"m1"},
				}, nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "authz denies",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().
					RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionRead, domain.GroupResource("team-a")).
					Return(domain.ErrForbidden)

				return group.NewHandler(az, group_mock.NewMockgroupUsecase(ctrl)), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionRead, gomock.Any()).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Get(ctx, gomock.Any(), "team-a").Return(nil, domain.ErrNotFound)

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.GetGroup(ctx, connect.NewRequest(&v1.GetGroupRequest{Name: "team-a"}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "team-a", resp.Msg.GetGroup().GetName())
		})
	}
}

func TestHandler_UpdateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(actor, domain.ObjectGroup, domain.ActionWrite, domain.GroupResource("team-a")).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Update(ctx, actor, groupuc.UpdateData{
					Name:        "team-a",
					DisplayName: "Team A",
					Description: "desc",
				}).Return(&domain.Group{Name: "team-a", DisplayName: "Team A"}, nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionWrite, gomock.Any()).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrVersionConflict)

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodeAborted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.UpdateGroup(ctx, connect.NewRequest(&v1.UpdateGroupRequest{
				Name:        "team-a",
				DisplayName: "Team A",
				Description: "desc",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "Team A", resp.Msg.GetGroup().GetDisplayName())
		})
	}
}

func TestHandler_UpdateGroupMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().
					RequireUser(actor, domain.ObjectGroup, domain.ActionWrite, domain.GroupResource("team-a")).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().UpdateMembers(ctx, actor, groupuc.UpdateMembersData{
					GroupName:    "team-a",
					AddEmails:    []string{"a@example.com"},
					RemoveEmails: []string{"b@example.com"},
				}).Return(&groupuc.UpdateMembersResult{
					Group:          &domain.Group{Name: "team-a"},
					VisibleMembers: []string{"a@example.com"},
				}, nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionWrite, gomock.Any()).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().UpdateMembers(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("membership grant denied"))

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.UpdateGroupMembers(ctx, connect.NewRequest(&v1.UpdateGroupMembersRequest{
				GroupName:    "team-a",
				AddEmails:    []string{"a@example.com"},
				RemoveEmails: []string{"b@example.com"},
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"a@example.com"}, resp.Msg.GetVisibleMembers())
		})
	}
}

func TestHandler_UpdateGroupPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().
					RequireUser(actor, domain.ObjectGroup, domain.ActionWrite, domain.GroupResource("team-a")).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().UpdatePermissions(ctx, actor, groupuc.UpdatePermissionsData{
					GroupName: "team-a",
				}).Return(&groupuc.UpdatePermissionsResult{
					Group: &domain.Group{Name: "team-a"},
				}, nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionWrite, gomock.Any()).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().UpdatePermissions(ctx, gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrPermissionEscalation)

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.UpdateGroupPermissions(ctx, connect.NewRequest(&v1.UpdateGroupPermissionsRequest{
				GroupName: "team-a",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "team-a", resp.Msg.GetGroup().GetName())
		})
	}
}

func TestHandler_DeleteGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(actor, domain.ObjectGroup, domain.ActionWrite, domain.GroupResource("team-a")).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Delete(ctx, actor, "team-a").Return(nil)

				return group.NewHandler(az, uc), ctx
			},
		},
		{
			name: "system group is immutable",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				az := group_mock.NewMockauthz(ctrl)
				az.EXPECT().RequireUser(gomock.Any(), domain.ObjectGroup, domain.ActionWrite, gomock.Any()).
					Return(nil)

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().Delete(ctx, gomock.Any(), "team-a").Return(domain.ErrSystemImmutable)

				return group.NewHandler(az, uc), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.DeleteGroup(ctx, connect.NewRequest(&v1.DeleteGroupRequest{Name: "team-a"}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, resp.Msg)
		})
	}
}

func TestHandler_ListGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Handler, context.Context)
		wantErr  string
		wantLen  int
		wantTot  int32
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				actor := domain.AuthInfo{Email: "actor@example.com"}

				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().List(ctx, actor, groupuc.ListParams{
					Limit:  10,
					Offset: 0,
					Query:  "team",
				}).Return(&groupuc.ListResult{
					Groups: []*domain.Group{{Name: "team-a"}, {Name: "team-b"}},
					Total:  2,
					Limit:  10,
				}, nil)

				return group.NewHandler(group_mock.NewMockauthz(ctrl), uc), ctx
			},
			wantLen: 2,
			wantTot: 2,
		},
		{
			name: "no auth context returns unauthenticated",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				return group.NewHandler(group_mock.NewMockauthz(ctrl), group_mock.NewMockgroupUsecase(ctrl)), ctx
			},
			wantErr: "unauthorized",
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*group.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := group_mock.NewMockgroupUsecase(ctrl)
				uc.EXPECT().List(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db down"))

				return group.NewHandler(group_mock.NewMockauthz(ctrl), uc), ctx
			},
			wantErr: "db down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.ListGroups(ctx, connect.NewRequest(&v1.ListGroupsRequest{
				Pagination: &commonv1.PaginationRequest{Limit: 10},
				Search:     "team",
			}))

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetGroups(), tt.wantLen)
			assert.Equal(t, tt.wantTot, resp.Msg.GetPagination().GetTotal())
		})
	}
}
