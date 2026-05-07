package v2

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

var (
	errOIDCWithPassword        = errors.New("initial_password must not be set in OIDC mode")
	errBasicAuthWithNoPassword = errors.New("initial_password is required in basic-auth mode")
)

// UserHandler implements authv1connect.UserServiceHandler.
type UserHandler struct {
	list          *authuc.ListUsersUseCase
	get           *authuc.GetUserUseCase
	createUser    *authuc.CreateUserUseCase
	resetPassword *authuc.ResetPasswordUseCase
	deleteUser    *authuc.DeleteUserUseCase
	authType      config.AuthType
}

// NewUserHandler returns a new UserHandler.
func NewUserHandler(
	list *authuc.ListUsersUseCase,
	get *authuc.GetUserUseCase,
	createUser *authuc.CreateUserUseCase,
	resetPassword *authuc.ResetPasswordUseCase,
	deleteUser *authuc.DeleteUserUseCase,
	authType config.AuthType,
) *UserHandler {
	return &UserHandler{
		list:          list,
		get:           get,
		createUser:    createUser,
		resetPassword: resetPassword,
		deleteUser:    deleteUser,
		authType:      authType,
	}
}

func (h *UserHandler) ListUsers(
	ctx context.Context,
	_ *connect.Request[authv1.ListUsersRequest],
) (*connect.Response[authv1.ListUsersResponse], error) {
	users, err := h.list.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*authv1.User, 0, len(users))
	for _, u := range users {
		protos = append(protos, domainUserToProto(u))
	}

	return connect.NewResponse(&authv1.ListUsersResponse{Users: protos}), nil
}

func (h *UserHandler) GetUser(
	ctx context.Context,
	req *connect.Request[authv1.GetUserRequest],
) (*connect.Response[authv1.GetUserResponse], error) {
	user, err := h.get.Execute(ctx, req.Msg.GetEmail())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.GetUserResponse{User: domainUserToProto(user)}), nil
}

func (h *UserHandler) CreateUser(
	ctx context.Context,
	req *connect.Request[authv1.CreateUserRequest],
) (*connect.Response[authv1.CreateUserResponse], error) {
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
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.CreateUserResponse{User: domainUserToProto(user)}), nil
}

func (h *UserHandler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[authv1.ResetUserPasswordRequest],
) (*connect.Response[authv1.ResetUserPasswordResponse], error) {
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
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.ResetUserPasswordResponse{}), nil
}

func (h *UserHandler) DeleteUser(
	ctx context.Context,
	req *connect.Request[authv1.DeleteUserRequest],
) (*connect.Response[authv1.DeleteUserResponse], error) {
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
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.DeleteUserResponse{}), nil
}

func domainUserToProto(u *domain.User) *authv1.User {
	if u == nil {
		return nil
	}

	return &authv1.User{
		Email:       u.Email,
		Name:        u.Name,
		Picture:     u.Picture,
		Provider:    u.Provider,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		LastLoginAt: timestamppb.New(u.LastLoginAt),
	}
}
