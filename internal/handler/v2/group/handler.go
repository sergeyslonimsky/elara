package group

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
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
		Create(ctx context.Context, user domain.AuthInfo, name string) (*domain.Group, error)
		Get(ctx context.Context, id string) (*domain.Group, error)
		Update(ctx context.Context, user domain.AuthInfo, data groupuc.UpdateData) (*domain.Group, error)
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

	group, err := h.uc.Create(ctx, user, req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.CreateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) GetGroup(
	ctx context.Context,
	req *connect.Request[v1.GetGroupRequest],
) (*connect.Response[v1.GetGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(user, domain.ObjectGroup, domain.ActionRead, "group:"+req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	group, err := h.uc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.GetGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[v1.UpdateGroupRequest],
) (*connect.Response[v1.UpdateGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(user, domain.ObjectGroup, domain.ActionWrite, "group:"+req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	data := updateGroupReqToData(req.Msg)

	group, err := h.uc.Update(ctx, user, data)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[v1.DeleteGroupRequest],
) (*connect.Response[v1.DeleteGroupResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err = h.authz.RequireUser(user, domain.ObjectGroup, domain.ActionWrite, "group:"+req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.uc.Delete(ctx, user, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.DeleteGroupResponse{}), nil
}

func (h *Handler) ListGroups(
	ctx context.Context,
	_ *connect.Request[v1.ListGroupsRequest],
) (*connect.Response[v1.ListGroupsResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	result, err := h.uc.List(ctx, user, groupuc.ListParams{})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*v1.Group, 0, len(result.Groups))
	for _, g := range result.Groups {
		protos = append(protos, domainGroupToProto(g))
	}

	return connect.NewResponse(&v1.ListGroupsResponse{Groups: protos}), nil
}
