package user

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	userv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1"
	useruc "github.com/sergeyslonimsky/elara/internal/usecase/user"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=user_mock -source=handler.go

var (
	errOIDCWithPassword        = errors.New("initial_password must not be set in OIDC mode")
	errBasicAuthWithNoPassword = errors.New("initial_password is required in basic-auth mode")
)

type (
	usecase interface {
		List(
			ctx context.Context,
			actor domain.AuthInfo,
			params useruc.ListParams,
		) (*useruc.ListResult, error)
		Get(ctx context.Context, actor domain.AuthInfo, email string) (*useruc.GetResult, error)
		Create(
			ctx context.Context,
			actor domain.AuthInfo,
			data useruc.CreateData,
		) (*useruc.CreateResult, error)
		ResetPassword(
			ctx context.Context,
			actor domain.AuthInfo,
			targetEmail, newPassword string,
		) error
		Delete(ctx context.Context, actor domain.AuthInfo, targetEmail string) error
		UpdateGroups(
			ctx context.Context,
			actor domain.AuthInfo,
			data useruc.UpdateGroupsData,
		) (*useruc.UpdateGroupsResult, error)
	}
)

// Handler implements userv1connect.UserServiceHandler.
type Handler struct {
	uc       usecase
	authType domain.AuthType
}

func New(uc usecase, authType domain.AuthType) *Handler {
	return &Handler{uc: uc, authType: authType}
}

func (h *Handler) ListUsers(
	ctx context.Context,
	req *connect.Request[userv1.ListUsersRequest],
) (*connect.Response[userv1.ListUsersResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.List(ctx, actor, useruc.ListParams{
		Limit:  int(req.Msg.GetPagination().GetLimit()),
		Offset: int(req.Msg.GetPagination().GetOffset()),
		Query:  req.Msg.GetSearch(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*userv1.User, 0, len(result.Users))
	for _, u := range result.Users {
		protos = append(protos, domainUserToProto(u))
	}

	return connect.NewResponse(&userv1.ListUsersResponse{
		Users: protos,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}

func (h *Handler) GetUser(
	ctx context.Context,
	req *connect.Request[userv1.GetUserRequest],
) (*connect.Response[userv1.GetUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.Get(ctx, actor, req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.GetUserResponse{
		User:              domainUserToProto(result.User),
		VisibleGroupIds:   result.VisibleGroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func (h *Handler) CreateUser(
	ctx context.Context,
	req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if h.authType == domain.AuthTypeNone {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"user creation is not available: auth type is none: %w",
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	if h.authType == domain.AuthTypeOIDC && req.Msg.GetInitialPassword() != "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errOIDCWithPassword)
	}
	if h.authType == domain.AuthTypeBasicAuth && req.Msg.GetInitialPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errBasicAuthWithNoPassword)
	}

	result, err := h.uc.Create(ctx, actor, useruc.CreateData{
		Email:           req.Msg.GetEmail(),
		Name:            req.Msg.GetName(),
		InitialPassword: req.Msg.GetInitialPassword(),
		InitialGroupIDs: req.Msg.GetInitialGroupIds(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.CreateUserResponse{
		User:              domainUserToProto(result.User),
		GroupIds:          result.GroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func (h *Handler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[userv1.ResetUserPasswordRequest],
) (*connect.Response[userv1.ResetUserPasswordResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.requireBasicAuth("password reset"); err != nil {
		return nil, err
	}

	if err := h.uc.ResetPassword(ctx, actor, req.Msg.GetEmail(), req.Msg.GetNewPassword()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.ResetUserPasswordResponse{}), nil
}

func (h *Handler) DeleteUser(
	ctx context.Context,
	req *connect.Request[userv1.DeleteUserRequest],
) (*connect.Response[userv1.DeleteUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.requireBasicAuth("user deletion"); err != nil {
		return nil, err
	}

	if err := h.uc.Delete(ctx, actor, req.Msg.GetEmail()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
}

// UpdateUserGroups applies an explicit add/remove delta to the target
// user's group memberships.
func (h *Handler) UpdateUserGroups(
	ctx context.Context,
	req *connect.Request[userv1.UpdateUserGroupsRequest],
) (*connect.Response[userv1.UpdateUserGroupsResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.UpdateGroups(ctx, actor, useruc.UpdateGroupsData{
		Email:                     req.Msg.GetEmail(),
		AddGroupIDs:               req.Msg.GetAddGroupIds(),
		RemoveGroupIDs:            req.Msg.GetRemoveGroupIds(),
		ExpectedMembershipVersion: req.Msg.ExpectedVersion,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.UpdateUserGroupsResponse{
		User:              domainUserToProto(result.User),
		VisibleGroupIds:   result.VisibleGroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func (h *Handler) requireBasicAuth(operation string) error {
	if h.authType == domain.AuthTypeBasicAuth {
		return nil
	}

	return connect.NewError(
		connect.CodeInvalidArgument,
		fmt.Errorf(
			"%s is not available: auth type is %s: %w",
			operation,
			h.authType,
			domain.ErrFeatureNotAvailable,
		),
	)
}

func domainUserToProto(u *domain.User) *userv1.User {
	if u == nil {
		return nil
	}

	return &userv1.User{
		Email:       u.Email,
		Name:        u.Name,
		Picture:     u.Picture,
		Provider:    u.Provider,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		LastLoginAt: timestamppb.New(u.LastLoginAt),
		IsSystem:    u.System,
	}
}
