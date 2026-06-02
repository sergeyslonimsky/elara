package user

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	handler "github.com/sergeyslonimsky/elara/internal/handler/v2"
	commonproto "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	userproto "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1"
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
		Get(ctx context.Context, actor domain.AuthInfo, userID uuid.UUID) (*useruc.GetResult, error)
		Create(
			ctx context.Context,
			actor domain.AuthInfo,
			data useruc.CreateData,
		) (*useruc.CreateResult, error)
		ResetPassword(
			ctx context.Context,
			actor domain.AuthInfo,
			userID uuid.UUID,
			newPassword string,
		) error
		Delete(ctx context.Context, actor domain.AuthInfo, userID uuid.UUID) error
		UpdateGroups(
			ctx context.Context,
			actor domain.AuthInfo,
			data useruc.UpdateGroupsData,
		) (*useruc.UpdateGroupsResult, error)
		Deactivate(
			ctx context.Context,
			actor domain.AuthInfo,
			userID uuid.UUID,
		) (*useruc.DeactivateResult, error)
		Reactivate(
			ctx context.Context,
			actor domain.AuthInfo,
			userID uuid.UUID,
		) (*useruc.ReactivateResult, error)
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
	req *connect.Request[userproto.ListUsersRequest],
) (*connect.Response[userproto.ListUsersResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	result, err := h.uc.List(ctx, actor, useruc.ListParams{
		Limit:  int(req.Msg.GetPagination().GetLimit()),
		Offset: int(req.Msg.GetPagination().GetOffset()),
		Query:  req.Msg.GetSearch(),
	})
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	protos := make([]*userproto.User, 0, len(result.Users))
	for _, u := range result.Users {
		protos = append(protos, domainUserToProto(u))
	}

	return connect.NewResponse(&userproto.ListUsersResponse{
		Users: protos,
		Pagination: &commonproto.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}

func (h *Handler) GetUser(
	ctx context.Context,
	req *connect.Request[userproto.GetUserRequest],
) (*connect.Response[userproto.GetUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := h.uc.Get(ctx, actor, userID)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.GetUserResponse{
		User:              domainUserToProto(result.User),
		VisibleGroupIds:   result.VisibleGroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func (h *Handler) CreateUser(
	ctx context.Context,
	req *connect.Request[userproto.CreateUserRequest],
) (*connect.Response[userproto.CreateUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
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
		DisplayName:     req.Msg.GetName(),
		InitialPassword: req.Msg.GetInitialPassword(),
		InitialGroupIDs: req.Msg.GetInitialGroupIds(),
	})
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.CreateUserResponse{
		User:              domainUserToProto(result.User),
		GroupIds:          result.GroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func (h *Handler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[userproto.ResetUserPasswordRequest],
) (*connect.Response[userproto.ResetUserPasswordResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	if err := h.requireBasicAuth("password reset"); err != nil {
		return nil, err
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	if err := h.uc.ResetPassword(ctx, actor, userID, req.Msg.GetNewPassword()); err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.ResetUserPasswordResponse{}), nil
}

func (h *Handler) DeleteUser(
	ctx context.Context,
	req *connect.Request[userproto.DeleteUserRequest],
) (*connect.Response[userproto.DeleteUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	if err := h.requireBasicAuth("user deletion"); err != nil {
		return nil, err
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	if err := h.uc.Delete(ctx, actor, userID); err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.DeleteUserResponse{}), nil
}

// UpdateUserGroups applies an explicit add/remove delta to the target
// user's group memberships.
func (h *Handler) UpdateUserGroups(
	ctx context.Context,
	req *connect.Request[userproto.UpdateUserGroupsRequest],
) (*connect.Response[userproto.UpdateUserGroupsResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := h.uc.UpdateGroups(ctx, actor, useruc.UpdateGroupsData{
		UserID:                    userID,
		AddGroupIDs:               req.Msg.GetAddGroupIds(),
		RemoveGroupIDs:            req.Msg.GetRemoveGroupIds(),
		ExpectedMembershipVersion: req.Msg.ExpectedVersion,
	})
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.UpdateUserGroupsResponse{
		User:              domainUserToProto(result.User),
		VisibleGroupIds:   result.VisibleGroupIDs,
		MembershipVersion: result.MembershipVersion,
	}), nil
}

func domainUserToProto(u *domain.User) *userproto.User {
	if u == nil {
		return nil
	}

	identities := make([]*userproto.Identity, 0, len(u.Identities))
	for _, id := range u.Identities {
		identities = append(identities, &userproto.Identity{
			Provider: string(id.Provider),
			Subject:  id.Subject,
		})
	}

	return &userproto.User{
		Id:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Picture:     u.Picture,
		Status:      domainStatusToProto(u.Status),
		Identities:  identities,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		LastLoginAt: timestamppb.New(u.LastLoginAt),
		IsSystem:    u.System,
	}
}

func domainStatusToProto(s domain.UserStatus) userproto.UserStatus {
	switch s {
	case domain.UserStatusActive:
		return userproto.UserStatus_USER_STATUS_ACTIVE
	case domain.UserStatusDeactivated:
		return userproto.UserStatus_USER_STATUS_DEACTIVATED
	default:
		return userproto.UserStatus_USER_STATUS_UNSPECIFIED
	}
}

func (h *Handler) DeactivateUser(
	ctx context.Context,
	req *connect.Request[userproto.DeactivateUserRequest],
) (*connect.Response[userproto.DeactivateUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := h.uc.Deactivate(ctx, actor, userID)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.DeactivateUserResponse{
		User: domainUserToProto(result.User),
	}), nil
}

func (h *Handler) ReactivateUser(
	ctx context.Context,
	req *connect.Request[userproto.ReactivateUserRequest],
) (*connect.Response[userproto.ReactivateUserResponse], error) {
	actor, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	userID, err := parseUserID(req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := h.uc.Reactivate(ctx, actor, userID)
	if err != nil {
		return nil, handler.ToConnectError(err)
	}

	return connect.NewResponse(&userproto.ReactivateUserResponse{
		User: domainUserToProto(result.User),
	}), nil
}

// parseUserID converts the wire-level user_id (UUID string) into a uuid.UUID,
// emitting an InvalidArgument connect error on parse failure. buf.validate
// already gates the wire shape, but defense-in-depth — Go-side parse keeps
// the handler robust if validation is ever bypassed.
func parseUserID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid user_id: %w", err))
	}

	return id, nil
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
