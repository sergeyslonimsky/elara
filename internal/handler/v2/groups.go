package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// GroupHandler implements authv1connect.GroupServiceHandler.
type GroupHandler struct {
	create       *authuc.CreateGroupUseCase
	get          *authuc.GetGroupUseCase
	update       *authuc.UpdateGroupUseCase
	del          *authuc.DeleteGroupUseCase
	list         *authuc.ListGroupsUseCase
	addMember    *authuc.AddMemberUseCase
	removeMember *authuc.RemoveMemberUseCase
}

// NewGroupHandler returns a new GroupHandler.
func NewGroupHandler(
	create *authuc.CreateGroupUseCase,
	get *authuc.GetGroupUseCase,
	update *authuc.UpdateGroupUseCase,
	del *authuc.DeleteGroupUseCase,
	list *authuc.ListGroupsUseCase,
	addMember *authuc.AddMemberUseCase,
	removeMember *authuc.RemoveMemberUseCase,
) *GroupHandler {
	return &GroupHandler{
		create:       create,
		get:          get,
		update:       update,
		del:          del,
		list:         list,
		addMember:    addMember,
		removeMember: removeMember,
	}
}

func (h *GroupHandler) CreateGroup(
	ctx context.Context,
	req *connect.Request[authv1.CreateGroupRequest],
) (*connect.Response[authv1.CreateGroupResponse], error) {
	group, err := h.create.Execute(ctx, req.Msg.GetName())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.CreateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) GetGroup(
	ctx context.Context,
	req *connect.Request[authv1.GetGroupRequest],
) (*connect.Response[authv1.GetGroupResponse], error) {
	group, err := h.get.Execute(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.GetGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[authv1.UpdateGroupRequest],
) (*connect.Response[authv1.UpdateGroupResponse], error) {
	group, err := h.update.Execute(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[authv1.DeleteGroupRequest],
) (*connect.Response[authv1.DeleteGroupResponse], error) {
	if err := h.del.Execute(ctx, req.Msg.GetId()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.DeleteGroupResponse{}), nil
}

func (h *GroupHandler) ListGroups(
	ctx context.Context,
	_ *connect.Request[authv1.ListGroupsRequest],
) (*connect.Response[authv1.ListGroupsResponse], error) {
	groups, err := h.list.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*authv1.Group, 0, len(groups))
	for _, g := range groups {
		protos = append(protos, domainGroupToProto(g))
	}

	return connect.NewResponse(&authv1.ListGroupsResponse{Groups: protos}), nil
}

func (h *GroupHandler) AddMember(
	ctx context.Context,
	req *connect.Request[authv1.AddMemberRequest],
) (*connect.Response[authv1.AddMemberResponse], error) {
	group, err := h.addMember.Execute(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.AddMemberResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) RemoveMember(
	ctx context.Context,
	req *connect.Request[authv1.RemoveMemberRequest],
) (*connect.Response[authv1.RemoveMemberResponse], error) {
	group, err := h.removeMember.Execute(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.RemoveMemberResponse{Group: domainGroupToProto(group)}), nil
}

func domainGroupToProto(g *domain.Group) *authv1.Group {
	if g == nil {
		return nil
	}

	return &authv1.Group{
		Id:        g.ID,
		Name:      g.Name,
		Members:   g.Members,
		CreatedAt: timestamppb.New(g.CreatedAt),
		UpdatedAt: timestamppb.New(g.UpdatedAt),
	}
}
