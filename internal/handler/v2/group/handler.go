package group

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	v1 "github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=group_mock -source=handler.go

type groupUsecase interface {
	Create(ctx context.Context, name string) (*domain.Group, error)
	Get(ctx context.Context, id string) (*domain.Group, error)
	Update(ctx context.Context, id, name, description string,
		perms []domain.Permission, members []string, version int64) (*domain.Group, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, params groupuc.ListParams) (*groupuc.ListResult, error)
}

type Handler struct {
	uc groupUsecase
}

func NewHandler(groupUsecase groupUsecase) *Handler {
	return &Handler{
		uc: groupUsecase,
	}
}

func (h *Handler) CreateGroup(
	ctx context.Context,
	req *connect.Request[v1.CreateGroupRequest],
) (*connect.Response[v1.CreateGroupResponse], error) {
	group, err := h.uc.Create(ctx, req.Msg.GetName())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.CreateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) GetGroup(
	ctx context.Context,
	req *connect.Request[v1.GetGroupRequest],
) (*connect.Response[v1.GetGroupResponse], error) {
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
	perms := permission.AssignmentsToDomain(req.Msg.GetPermissions())
	group, err := h.uc.Update(
		ctx,
		req.Msg.GetId(),
		req.Msg.GetName(),
		req.Msg.GetDescription(),
		perms,
		req.Msg.GetMembers(),
		req.Msg.GetVersion(),
	)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.UpdateGroupResponse{Group: domainGroupToProto(group)}), nil
}

func (h *Handler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[v1.DeleteGroupRequest],
) (*connect.Response[v1.DeleteGroupResponse], error) {
	if err := h.uc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&v1.DeleteGroupResponse{}), nil
}

func (h *Handler) ListGroups(
	ctx context.Context,
	_ *connect.Request[v1.ListGroupsRequest],
) (*connect.Response[v1.ListGroupsResponse], error) {
	result, err := h.uc.List(ctx, groupuc.ListParams{})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*v1.Group, 0, len(result.Groups))
	for _, g := range result.Groups {
		protos = append(protos, domainGroupToProto(g))
	}

	return connect.NewResponse(&v1.ListGroupsResponse{Groups: protos}), nil
}

func domainGroupToProto(g *domain.Group) *v1.Group {
	if g == nil {
		return nil
	}

	return &v1.Group{
		Id:          g.ID,
		Name:        g.Name,
		Members:     g.Members,
		CreatedAt:   timestamppb.New(g.CreatedAt),
		UpdatedAt:   timestamppb.New(g.UpdatedAt),
		IsSystem:    g.System,
		Description: g.Description,
		Version:     g.Version,
		// Permissions are stored in Casbin policy, not on the entity — fetched
		// separately via PDP. Wiring in M5/M6.
	}
}
