package user_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/user"
	user_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/user/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	userproto "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1"
	useruc "github.com/sergeyslonimsky/elara/internal/usecase/user"
)

// withActor returns a context carrying claims so authctx.AuthInfoFromContext
// resolves a valid domain.AuthInfo.
func withActor(ctx context.Context, email string) context.Context {
	return authctx.WithClaims(ctx, &authctx.Claims{Email: email})
}

func TestHandler_ListUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantErr  string
		wantLen  int
		wantTot  int32
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().List(ctx, domain.AuthInfo{Email: "actor@example.com"}, useruc.ListParams{
					Limit:  10,
					Offset: 0,
					Query:  "bob",
				}).Return(&useruc.ListResult{
					Users: []*domain.User{{ID: uuid.New(), Email: "bob@example.com"}},
					Total: 1,
					Limit: 10,
				}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			name: "no auth context returns unauthenticated",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				uc := user_mock.NewMockusecase(ctrl)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantErr: "unauthorized",
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().List(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db down"))

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantErr: "db down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.ListUsers(ctx, connect.NewRequest(&userproto.ListUsersRequest{
				Pagination: &commonv1.PaginationRequest{Limit: 10},
				Search:     "bob",
			}))

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetUsers(), tt.wantLen)
			assert.Equal(t, tt.wantTot, resp.Msg.GetPagination().GetTotal())
		})
	}
}

func TestHandler_GetUser(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
		wantErr  string
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Get(ctx, domain.AuthInfo{Email: "actor@example.com"}, id).
					Return(&useruc.GetResult{
						User:              &domain.User{ID: id, Email: "target@example.com"},
						VisibleGroupIDs:   []string{"g1"},
						MembershipVersion: 3,
					}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "invalid user id",
			reqID: "not-a-uuid",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase not found",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Get(ctx, gomock.Any(), id).Return(nil, domain.ErrNotFound)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.GetUser(ctx, connect.NewRequest(&userproto.GetUserRequest{UserId: tt.reqID}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, id.String(), resp.Msg.GetUser().GetId())
			assert.Equal(t, []string{"g1"}, resp.Msg.GetVisibleGroupIds())
			assert.Equal(t, int64(3), resp.Msg.GetMembershipVersion())
		})
	}
}

func TestHandler_CreateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *userproto.CreateUserRequest
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
		wantErr  string
	}{
		{
			name: "success basic auth",
			req: &userproto.CreateUserRequest{
				Email:           "new@example.com",
				Name:            "New User",
				InitialPassword: "secret",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Create(ctx, domain.AuthInfo{Email: "actor@example.com"}, useruc.CreateData{
					Email:           "new@example.com",
					DisplayName:     "New User",
					InitialPassword: "secret",
				}).Return(&useruc.CreateResult{
					User:              &domain.User{Email: "new@example.com"},
					GroupIDs:          []string{"g1"},
					MembershipVersion: 1,
				}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name: "auth type none is rejected",
			req:  &userproto.CreateUserRequest{Email: "x@example.com"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeNone), ctx
			},
			wantCode: connect.CodeInvalidArgument,
			wantErr:  "user creation is not available",
		},
		{
			name: "oidc with initial password is rejected",
			req:  &userproto.CreateUserRequest{Email: "x@example.com", InitialPassword: "secret"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeOIDC), ctx
			},
			wantCode: connect.CodeInvalidArgument,
			wantErr:  "initial_password must not be set",
		},
		{
			name: "basic auth without password is rejected",
			req:  &userproto.CreateUserRequest{Email: "x@example.com"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
			wantErr:  "initial_password is required",
		},
		{
			name: "usecase error propagates",
			req:  &userproto.CreateUserRequest{Email: "x@example.com", InitialPassword: "secret"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrAlreadyExists)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.CreateUser(ctx, connect.NewRequest(tt.req))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))
				if tt.wantErr != "" {
					require.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "new@example.com", resp.Msg.GetUser().GetEmail())
			assert.Equal(t, []string{"g1"}, resp.Msg.GetGroupIds())
		})
	}
}

func TestHandler_ResetUserPassword(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().ResetPassword(ctx, domain.AuthInfo{Email: "actor@example.com"}, id, "newpass").
					Return(nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "oidc mode is rejected before parsing id",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeOIDC), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "invalid user id",
			reqID: "bad-id",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase error propagates",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().ResetPassword(ctx, gomock.Any(), id, "newpass").
					Return(domain.ErrForbidden)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.ResetUserPassword(ctx, connect.NewRequest(&userproto.ResetUserPasswordRequest{
				UserId:      tt.reqID,
				NewPassword: "newpass",
			}))

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

func TestHandler_DeleteUser(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Delete(ctx, domain.AuthInfo{Email: "actor@example.com"}, id).Return(nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "oidc mode is rejected",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeOIDC), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "invalid user id",
			reqID: "bad-id",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase error propagates",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Delete(ctx, gomock.Any(), id).Return(domain.ErrSystemImmutable)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.DeleteUser(ctx, connect.NewRequest(&userproto.DeleteUserRequest{UserId: tt.reqID}))

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

func TestHandler_UpdateUserGroups(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().UpdateGroups(ctx, domain.AuthInfo{Email: "actor@example.com"}, useruc.UpdateGroupsData{
					UserID:         id,
					AddGroupIDs:    []string{"g1"},
					RemoveGroupIDs: []string{"g2"},
				}).Return(&useruc.UpdateGroupsResult{
					User:              &domain.User{ID: id},
					VisibleGroupIDs:   []string{"g1"},
					MembershipVersion: 2,
				}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "invalid user id",
			reqID: "bad-id",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase error propagates",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().UpdateGroups(ctx, gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrVersionConflict)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeAborted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.UpdateUserGroups(ctx, connect.NewRequest(&userproto.UpdateUserGroupsRequest{
				UserId:         tt.reqID,
				AddGroupIds:    []string{"g1"},
				RemoveGroupIds: []string{"g2"},
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"g1"}, resp.Msg.GetVisibleGroupIds())
			assert.Equal(t, int64(2), resp.Msg.GetMembershipVersion())
		})
	}
}

func TestHandler_DeactivateUser(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Deactivate(ctx, domain.AuthInfo{Email: "actor@example.com"}, id).
					Return(&useruc.DeactivateResult{User: &domain.User{ID: id, Status: domain.UserStatusDeactivated}}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "invalid user id",
			reqID: "bad-id",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase error propagates",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Deactivate(ctx, gomock.Any(), id).Return(nil, domain.ErrNotFound)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.DeactivateUser(ctx, connect.NewRequest(&userproto.DeactivateUserRequest{UserId: tt.reqID}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, userproto.UserStatus_USER_STATUS_DEACTIVATED, resp.Msg.GetUser().GetStatus())
		})
	}
}

func TestHandler_ReactivateUser(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name     string
		reqID    string
		mockFunc func(context.Context, *gomock.Controller) (*user.Handler, context.Context)
		wantCode connect.Code
	}{
		{
			name:  "success",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Reactivate(ctx, domain.AuthInfo{Email: "actor@example.com"}, id).
					Return(&useruc.ReactivateResult{User: &domain.User{ID: id, Status: domain.UserStatusActive}}, nil)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
		},
		{
			name:  "invalid user id",
			reqID: "bad-id",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")

				return user.New(user_mock.NewMockusecase(ctrl), domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:  "usecase error propagates",
			reqID: id.String(),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Handler, context.Context) {
				ctx = withActor(ctx, "actor@example.com")
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Reactivate(ctx, gomock.Any(), id).Return(nil, domain.ErrNotFound)

				return user.New(uc, domain.AuthTypeBasicAuth), ctx
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.ReactivateUser(ctx, connect.NewRequest(&userproto.ReactivateUserRequest{UserId: tt.reqID}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, userproto.UserStatus_USER_STATUS_ACTIVE, resp.Msg.GetUser().GetStatus())
		})
	}
}
