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
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	useruc "github.com/sergeyslonimsky/elara/internal/usecase/user"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=user_mock -source=handler.go

var (
	errOIDCWithPassword        = errors.New("initial_password must not be set in OIDC mode")
	errBasicAuthWithNoPassword = errors.New("initial_password is required in basic-auth mode")
)

type (
	authz interface {
		Require(ctx context.Context, object, action, domainStr string) error
	}

	usecase interface {
		List(ctx context.Context, params useruc.ListParams) (*useruc.ListResult, error)
		Get(ctx context.Context, email string) (*useruc.GetResult, error)
		Create(ctx context.Context, email, name, initialPassword string) (*domain.User, error)
		ResetPassword(ctx context.Context, targetEmail, newPassword string) error
		Delete(ctx context.Context, targetEmail string) error
		UpdateGroups(
			ctx context.Context,
			actor domain.AuthInfo,
			data useruc.UpdateGroupsData,
		) (*useruc.UpdateGroupsResult, error)
	}
)

// Handler implements userv1connect.UserServiceHandler.
type Handler struct {
	authz    authz
	uc       usecase
	authType config.AuthType
}

// New returns a new Handler.
func New(
	authz authz,
	uc usecase,
	authType config.AuthType,
) *Handler {
	return &Handler{
		authz:    authz,
		uc:       uc,
		authType: authType,
	}
}

func (h *Handler) ListUsers(
	ctx context.Context,
	_ *connect.Request[userv1.ListUsersRequest],
) (*connect.Response[userv1.ListUsersResponse], error) {
	result, err := h.uc.List(ctx, useruc.ListParams{})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*userv1.User, 0, len(result.Users))
	for _, u := range result.Users {
		protos = append(protos, domainUserToProto(u))
	}

	return connect.NewResponse(&userv1.ListUsersResponse{Users: protos}), nil
}

func (h *Handler) GetUser(
	ctx context.Context,
	req *connect.Request[userv1.GetUserRequest],
) (*connect.Response[userv1.GetUserResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectUser, domain.ActionRead, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.Get(ctx, req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.GetUserResponse{
		User:     domainUserToProto(result.User),
		GroupIds: result.GroupIDs,
	}), nil
}

func (h *Handler) CreateUser(
	ctx context.Context,
	req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectUser, domain.ActionCreate, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

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

	user, err := h.uc.Create(ctx, req.Msg.GetEmail(), req.Msg.GetName(), req.Msg.GetInitialPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.CreateUserResponse{User: domainUserToProto(user)}), nil
}

func (h *Handler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[userv1.ResetUserPasswordRequest],
) (*connect.Response[userv1.ResetUserPasswordResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectUser, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

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

	err := h.uc.ResetPassword(ctx, req.Msg.GetEmail(), req.Msg.GetNewPassword())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.ResetUserPasswordResponse{}), nil
}

func (h *Handler) DeleteUser(
	ctx context.Context,
	req *connect.Request[userv1.DeleteUserRequest],
) (*connect.Response[userv1.DeleteUserResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectUser, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

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

	if err := h.uc.Delete(ctx, req.Msg.GetEmail()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
}

// UpdateUserGroups replaces the target user's group memberships with the
// requested set of group IDs.
//
// The handler enforces the coarse resource-class gate (ObjectUser:Write
// always, ObjectGroup:Write when the request carries any group changes) so
// that an unauthorized caller fails fast without driving a full WriteTx.
// The usecase then layers the per-delta checks — ObjectGroup:Write on each
// affected group plus anti-escalation on additions — inside the same
// transaction as the Casbin g-rule sync.
func (h *Handler) UpdateUserGroups(
	ctx context.Context,
	req *connect.Request[userv1.UpdateUserGroupsRequest],
) (*connect.Response[userv1.UpdateUserGroupsResponse], error) {
	actor, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.authz.Require(ctx, domain.ObjectUser, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	if len(req.Msg.GetGroupIds()) > 0 {
		if err := h.authz.Require(ctx, domain.ObjectGroup, domain.ActionWrite, domain.DomainAll); err != nil {
			return nil, v2.ToConnectError(err)
		}
	}

	result, err := h.uc.UpdateGroups(ctx, actor, useruc.UpdateGroupsData{
		Email:    req.Msg.GetEmail(),
		GroupIDs: req.Msg.GetGroupIds(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&userv1.UpdateUserGroupsResponse{
		User:     domainUserToProto(result.User),
		GroupIds: result.GroupIDs,
	}), nil
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
