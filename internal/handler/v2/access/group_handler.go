package access

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
)

//go:generate mockgen -destination=mocks/group_handler_mock.go -package=access_mock -source=group_handler.go

type groupUsecase interface {
	Create(ctx context.Context, name string) (*domain.Group, error)
	Get(ctx context.Context, id string) (*domain.Group, error)
	Update(ctx context.Context, id, name string) (*domain.Group, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*domain.Group, error)
	AddMember(ctx context.Context, groupID, email string) (*domain.Group, error)
	RemoveMember(ctx context.Context, groupID, email string) (*domain.Group, error)
}

// GroupHandler implements accessv1connect.GroupServiceHandler.
type GroupHandler struct {
	uc groupUsecase
}

// NewGroupHandler returns a new GroupHandler.
func NewGroupHandler(uc groupUsecase) *GroupHandler {
	return &GroupHandler{uc: uc}
}

func (h *GroupHandler) CreateGroup(
	ctx context.Context,
	req *connect.Request[accessv1.CreateGroupRequest],
) (*connect.Response[accessv1.CreateGroupResponse], error) {
	group, err := h.uc.Create(ctx, req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.CreateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) GetGroup(
	ctx context.Context,
	req *connect.Request[accessv1.GetGroupRequest],
) (*connect.Response[accessv1.GetGroupResponse], error) {
	group, err := h.uc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.GetGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[accessv1.UpdateGroupRequest],
) (*connect.Response[accessv1.UpdateGroupResponse], error) {
	group, err := h.uc.Update(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[accessv1.DeleteGroupRequest],
) (*connect.Response[accessv1.DeleteGroupResponse], error) {
	if err := h.uc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.DeleteGroupResponse{}), nil
}

func (h *GroupHandler) ListGroups(
	ctx context.Context,
	_ *connect.Request[accessv1.ListGroupsRequest],
) (*connect.Response[accessv1.ListGroupsResponse], error) {
	groups, err := h.uc.List(ctx)
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
	group, err := h.uc.AddMember(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.AddMemberResponse{Group: domainGroupToProto(group)}), nil
}

func (h *GroupHandler) RemoveMember(
	ctx context.Context,
	req *connect.Request[accessv1.RemoveMemberRequest],
) (*connect.Response[accessv1.RemoveMemberResponse], error) {
	group, err := h.uc.RemoveMember(ctx, req.Msg.GetGroupId(), req.Msg.GetEmail())
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
