package access

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// GroupHandler implements accessv1connect.GroupServiceHandler.
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
	req *connect.Request[accessv1.CreateGroupRequest],
) (*connect.Response[accessv1.CreateGroupResponse], error) {
	group, err := h.create.Execute(ctx, req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.CreateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) GetGroup(
	ctx context.Context,
	req *connect.Request[accessv1.GetGroupRequest],
) (*connect.Response[accessv1.GetGroupResponse], error) {
	group, err := h.get.Execute(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.GetGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[accessv1.UpdateGroupRequest],
) (*connect.Response[accessv1.UpdateGroupResponse], error) {
	group, err := h.update.Execute(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[accessv1.DeleteGroupRequest],
) (*connect.Response[accessv1.DeleteGroupResponse], error) {
	if err := h.del.Execute(ctx, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.DeleteGroupResponse{}), nil
}

func (h *GroupHandler) ListGroups(
	ctx context.Context,
	_ *connect.Request[accessv1.ListGroupsRequest],
) (*connect.Response[accessv1.ListGroupsResponse], error) {
	groups, err := h.list.Execute(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*accessv1.Group, 0, len(groups))
	for _, g := range groups {
		protos = append(protos, domainGroupToProto(g))
	}

	return connect.NewResponse(&accessv1.ListGroupsResponse{Groups: protos}), nil
}

func (h *GroupHandler) AddMember(
	ctx context.Context,
	req *connect.Request[accessv1.AddMemberRequest],
) (*connect.Response[accessv1.AddMemberResponse], error) {
	group, err := h.addMember.Execute(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.AddMemberResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) RemoveMember(
	ctx context.Context,
	req *connect.Request[accessv1.RemoveMemberRequest],
) (*connect.Response[accessv1.RemoveMemberResponse], error) {
	group, err := h.removeMember.Execute(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.RemoveMemberResponse{Group: domainGroupToProto(group)}), nil
}

func domainGroupToProto(g *domain.Group) *accessv1.Group {
	if g == nil {
		return nil
	}

	return &accessv1.Group{
		Id:        g.ID,
		Name:      g.Name,
		Members:   g.Members,
		CreatedAt: timestamppb.New(g.CreatedAt),
		UpdatedAt: timestamppb.New(g.UpdatedAt),
	}
}
