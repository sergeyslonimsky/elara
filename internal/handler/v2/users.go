package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// UserHandler implements authv1connect.UserServiceHandler.
type UserHandler struct {
	list *authuc.ListUsersUseCase
	get  *authuc.GetUserUseCase
}

// NewUserHandler returns a new UserHandler.
func NewUserHandler(list *authuc.ListUsersUseCase, get *authuc.GetUserUseCase) *UserHandler {
	return &UserHandler{list: list, get: get}
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
