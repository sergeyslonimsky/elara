package group

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=group_mock -source=handler.go

type (
	authz interface {
		RequireUser(user domain.AuthInfo, object, action, domainStr string) error
	}

	groupUsecase interface {
		Create(ctx context.Context, user domain.AuthInfo, data groupuc.CreateData) (*groupuc.CreateResult, error)
		Get(ctx context.Context, user domain.AuthInfo, id string) (*groupuc.GetResult, error)
		Update(ctx context.Context, user domain.AuthInfo, data groupuc.UpdateData) (*domain.Group, error)
		UpdateMembers(
			ctx context.Context,
			user domain.AuthInfo,
			data groupuc.UpdateMembersData,
		) (*groupuc.UpdateMembersResult, error)
		UpdatePermissions(
			ctx context.Context,
			user domain.AuthInfo,
			data groupuc.UpdatePermissionsData,
		) (*groupuc.UpdatePermissionsResult, error)
		Delete(ctx context.Context, user domain.AuthInfo, id string) error
		List(ctx context.Context, user domain.AuthInfo, params groupuc.ListParams) (*groupuc.ListResult, error)
	}
)

type Handler struct {
	authz authz
	uc    groupUsecase
}

func NewHandler(authz authz, uc groupUsecase) *Handler {
	return &Handler{
		authz: authz,
		uc:    uc,
	}
}

func (h *Handler) CreateGroup(
	ctx context.Context,
	req *connect.Request[v1.CreateGroupRequest],
) (*connect.Response[v1.CreateGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(user, domain.ObjectGroup, domain.ActionCreate, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.Create(ctx, user, groupuc.CreateData{
		Name:                   req.Msg.GetName(),
		Description:            req.Msg.GetDescription(),
		InitialMembers:         req.Msg.GetInitialMembers(),
		InitialPermissions:     permission.AssignmentsToDomain(req.Msg.GetInitialPermissions()),
		InitialManagerGroupIDs: req.Msg.GetInitialManagerGroupIds(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.CreateGroupResponse{
		Group:          domainGroupToProto(result.Group),
		VisibleMembers: result.VisibleMembers,
		Permissions:    permission.AssignmentsToProto(result.Permissions),
	}), nil
}

func (h *Handler) GetGroup(
	ctx context.Context,
	req *connect.Request[v1.GetGroupRequest],
) (*connect.Response[v1.GetGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(
		user,
		domain.ObjectGroup,
		domain.ActionRead,
		domain.GroupResource(req.Msg.GetId()),
	); err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.Get(ctx, user, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.GetGroupResponse{
		Group:          domainGroupToProto(result.Group),
		VisibleMembers: result.VisibleMembers,
		Permissions:    permission.AssignmentsToProto(result.Permissions),
	}), nil
}

func (h *Handler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[v1.UpdateGroupRequest],
) (*connect.Response[v1.UpdateGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(
		user,
		domain.ObjectGroup,
		domain.ActionWrite,
		domain.GroupResource(req.Msg.GetId()),
	); err != nil {
		return nil, v2.ToConnectError(err)
	}

	group, err := h.uc.Update(ctx, user, groupuc.UpdateData{
		ID:                      req.Msg.GetId(),
		Name:                    req.Msg.GetName(),
		Description:             req.Msg.GetDescription(),
		ExpectedMetadataVersion: req.Msg.ExpectedMetadataVersion,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) UpdateGroupMembers(
	ctx context.Context,
	req *connect.Request[v1.UpdateGroupMembersRequest],
) (*connect.Response[v1.UpdateGroupMembersResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(
		user,
		domain.ObjectGroup,
		domain.ActionWrite,
		domain.GroupResource(req.Msg.GetGroupId()),
	); err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.UpdateMembers(ctx, user, groupuc.UpdateMembersData{
		GroupID:                req.Msg.GetGroupId(),
		AddEmails:              req.Msg.GetAddEmails(),
		RemoveEmails:           req.Msg.GetRemoveEmails(),
		ExpectedMembersVersion: req.Msg.ExpectedMembersVersion,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.UpdateGroupMembersResponse{
		Group:          domainGroupToProto(result.Group),
		VisibleMembers: result.VisibleMembers,
	}), nil
}

func (h *Handler) UpdateGroupPermissions(
	ctx context.Context,
	req *connect.Request[v1.UpdateGroupPermissionsRequest],
) (*connect.Response[v1.UpdateGroupPermissionsResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(
		user,
		domain.ObjectGroup,
		domain.ActionWrite,
		domain.GroupResource(req.Msg.GetGroupId()),
	); err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.UpdatePermissions(ctx, user, groupuc.UpdatePermissionsData{
		GroupID:                    req.Msg.GetGroupId(),
		Add:                        permission.AssignmentsToDomain(req.Msg.GetAdd()),
		Remove:                     permission.AssignmentsToDomain(req.Msg.GetRemove()),
		ExpectedPermissionsVersion: req.Msg.ExpectedPermissionsVersion,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.UpdateGroupPermissionsResponse{
		Group:       domainGroupToProto(result.Group),
		Permissions: permission.AssignmentsToProto(result.Permissions),
	}), nil
}

func (h *Handler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[v1.DeleteGroupRequest],
) (*connect.Response[v1.DeleteGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(
		user,
		domain.ObjectGroup,
		domain.ActionWrite,
		domain.GroupResource(req.Msg.GetId()),
	); err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.uc.Delete(ctx, user, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.DeleteGroupResponse{}), nil
}

func (h *Handler) ListGroups(
	ctx context.Context,
	req *connect.Request[v1.ListGroupsRequest],
) (*connect.Response[v1.ListGroupsResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.List(ctx, user, groupuc.ListParams{
		Limit:  int(req.Msg.GetPagination().GetLimit()),
		Offset: int(req.Msg.GetPagination().GetOffset()),
		Query:  req.Msg.GetSearch(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*v1.Group, 0, len(result.Groups))
	for _, g := range result.Groups {
		protos = append(protos, domainGroupToProto(g))
	}

	return connect.NewResponse(&v1.ListGroupsResponse{
		Groups: protos,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}
