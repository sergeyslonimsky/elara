package user

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	userv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

var (
	errOIDCWithPassword        = errors.New("initial_password must not be set in OIDC mode")
	errBasicAuthWithNoPassword = errors.New("initial_password is required in basic-auth mode")
)

// Handler implements userv1connect.UserServiceHandler.
type Handler struct {
	list          *authuc.ListUsersUseCase
	get           *authuc.GetUserUseCase
	createUser    *authuc.CreateUserUseCase
	resetPassword *authuc.ResetPasswordUseCase
	deleteUser    *authuc.DeleteUserUseCase
	authType      config.AuthType
}

// New returns a new Handler.
func New(
	list *authuc.ListUsersUseCase,
	get *authuc.GetUserUseCase,
	createUser *authuc.CreateUserUseCase,
	resetPassword *authuc.ResetPasswordUseCase,
	deleteUser *authuc.DeleteUserUseCase,
	authType config.AuthType,
) *Handler {
	return &Handler{
		list:          list,
		get:           get,
		createUser:    createUser,
		resetPassword: resetPassword,
		deleteUser:    deleteUser,
		authType:      authType,
	}
}

func (h *Handler) ListUsers(
	ctx context.Context,
	_ *connect.Request[userv1.ListUsersRequest],
) (*connect.Response[userv1.ListUsersResponse], error) {
	users, err := h.list.Execute(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*userv1.User, 0, len(users))
	for _, u := range users {
		protos = append(protos, domainUserToProto(u))
	}

	return connect.NewResponse(&userv1.ListUsersResponse{Users: protos}), nil
}

func (h *Handler) GetUser(
	ctx context.Context,
	req *connect.Request[userv1.GetUserRequest],
) (*connect.Response[userv1.GetUserResponse], error) {
	user, err := h.get.Execute(ctx, req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.GetUserResponse{User: domainUserToProto(user)}), nil
}

func (h *Handler) CreateUser(
	ctx context.Context,
	req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {
	if h.authType == config.AuthTypeNone {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("user creation is not available: auth type is none: %w", domain.ErrFeatureNotAvailable),
		)
	}

	if h.authType == config.AuthTypeOIDC && req.Msg.GetInitialPassword() != "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errOIDCWithPassword,
		)
	}

	if h.authType == config.AuthTypeBasicAuth && req.Msg.GetInitialPassword() == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errBasicAuthWithNoPassword,
		)
	}

	user, err := h.createUser.Execute(ctx, req.Msg.GetEmail(), req.Msg.GetName(), req.Msg.GetInitialPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.CreateUserResponse{User: domainUserToProto(user)}), nil
}

func (h *Handler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[userv1.ResetUserPasswordRequest],
) (*connect.Response[userv1.ResetUserPasswordResponse], error) {
	if h.authType != config.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"password reset is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	err := h.resetPassword.Execute(ctx, req.Msg.GetEmail(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.ResetUserPasswordResponse{}), nil
}

func (h *Handler) DeleteUser(
	ctx context.Context,
	req *connect.Request[userv1.DeleteUserRequest],
) (*connect.Response[userv1.DeleteUserResponse], error) {
	if h.authType != config.AuthTypeBasicAuth {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf(
				"user deletion is not available: auth type is %s: %w",
				h.authType,
				domain.ErrFeatureNotAvailable,
			),
		)
	}

	if err := h.deleteUser.Execute(ctx, req.Msg.GetEmail()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
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
	}
}
