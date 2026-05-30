package filter

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	filterv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/filter/v1"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	filteruc "github.com/sergeyslonimsky/elara/internal/usecase/filter"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=filter_mock -source=handler.go

type filterUsecase interface {
	Namespaces(
		ctx context.Context,
		actor domain.AuthInfo,
		query filteruc.Query,
	) ([]filteruc.Item, error)
	Groups(
		ctx context.Context,
		actor domain.AuthInfo,
		query filteruc.Query,
	) ([]filteruc.Item, error)
	Users(ctx context.Context, actor domain.AuthInfo, query filteruc.Query) ([]filteruc.Item, error)
}

type Handler struct {
	uc filterUsecase
}

func New(uc filterUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) GetNamespaces(
	ctx context.Context,
	req *connect.Request[filterv1.GetNamespacesRequest],
) (*connect.Response[filterv1.GetNamespacesResponse], error) {
	actor, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	items, err := h.uc.Namespaces(ctx, actor, toQuery(req.Msg.GetFilters(), req.Msg.GetActions()))
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&filterv1.GetNamespacesResponse{Items: itemsToProto(items)}), nil
}

func (h *Handler) GetGroups(
	ctx context.Context,
	req *connect.Request[filterv1.GetGroupsRequest],
) (*connect.Response[filterv1.GetGroupsResponse], error) {
	actor, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	items, err := h.uc.Groups(ctx, actor, toQuery(req.Msg.GetFilters(), req.Msg.GetActions()))
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&filterv1.GetGroupsResponse{Items: itemsToProto(items)}), nil
}

func (h *Handler) GetUsers(
	ctx context.Context,
	req *connect.Request[filterv1.GetUsersRequest],
) (*connect.Response[filterv1.GetUsersResponse], error) {
	actor, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	items, err := h.uc.Users(ctx, actor, toQuery(req.Msg.GetFilters(), req.Msg.GetActions()))
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&filterv1.GetUsersResponse{Items: itemsToProto(items)}), nil
}

func toQuery(filters *filterv1.Filters, actions []commonv1.PermissionAction) filteruc.Query {
	return filteruc.Query{
		Actions: actionsToDomain(actions),
		Search:  filters.GetQuery(),
	}
}

// actionsToDomain maps request action enums to domain actions, silently
// dropping UNSPECIFIED/unknown entries.
func actionsToDomain(in []commonv1.PermissionAction) []domain.Action {
	if len(in) == 0 {
		return nil
	}

	out := make([]domain.Action, 0, len(in))
	for _, a := range in {
		if act := permission.ActionToDomain(a); act != "" {
			out = append(out, act)
		}
	}

	return out
}

func itemsToProto(items []filteruc.Item) []*filterv1.Item {
	out := make([]*filterv1.Item, 0, len(items))
	for _, it := range items {
		out = append(out, &filterv1.Item{
			Key:     it.Key,
			Value:   it.Value,
			Actions: actionsToProto(it.Actions),
		})
	}

	return out
}

func actionsToProto(in []domain.Action) []commonv1.PermissionAction {
	out := make([]commonv1.PermissionAction, 0, len(in))
	for _, a := range in {
		out = append(out, permission.ActionToProto(a))
	}

	return out
}
