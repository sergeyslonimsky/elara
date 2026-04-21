package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	namespacev2 "github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

type NamespaceHandler struct {
	create *nsuc.CreateUseCase
	get    *nsuc.GetUseCase
	update *nsuc.UpdateUseCase
	list   *nsuc.ListUseCase
	del    *nsuc.DeleteUseCase
	lock   *nsuc.LockUseCase
	unlock *nsuc.UnlockUseCase
}

func NewNamespaceHandler(
	create *nsuc.CreateUseCase,
	get *nsuc.GetUseCase,
	update *nsuc.UpdateUseCase,
	list *nsuc.ListUseCase,
	del *nsuc.DeleteUseCase,
	lock *nsuc.LockUseCase,
	unlock *nsuc.UnlockUseCase,
) *NamespaceHandler {
	return &NamespaceHandler{
		create: create,
		get:    get,
		update: update,
		list:   list,
		del:    del,
		lock:   lock,
		unlock: unlock,
	}
}

func (h *NamespaceHandler) CreateNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.CreateNamespaceRequest],
) (*connect.Response[namespacev2.CreateNamespaceResponse], error) {
	ns := &domain.Namespace{
		Name:        req.Msg.GetName(),
		Description: req.Msg.GetDescription(),
	}

	result, err := h.create.Execute(ctx, ns)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.CreateNamespaceResponse{
		Namespace: domainNamespaceToProto(result),
	}), nil
}

func (h *NamespaceHandler) GetNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.GetNamespaceRequest],
) (*connect.Response[namespacev2.GetNamespaceResponse], error) {
	result, err := h.get.Execute(ctx, req.Msg.GetName())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.GetNamespaceResponse{
		Namespace: domainNamespaceToProto(result),
	}), nil
}

func (h *NamespaceHandler) UpdateNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.UpdateNamespaceRequest],
) (*connect.Response[namespacev2.UpdateNamespaceResponse], error) {
	result, err := h.update.Execute(ctx, req.Msg.GetName(), req.Msg.GetDescription())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.UpdateNamespaceResponse{
		Namespace: domainNamespaceToProto(result),
	}), nil
}

func (h *NamespaceHandler) ListNamespaces(
	ctx context.Context,
	req *connect.Request[namespacev2.ListNamespacesRequest],
) (*connect.Response[namespacev2.ListNamespacesResponse], error) {
	params := nsuc.NSListParams{
		Sort:  protoSortToDomain(req.Msg.GetSort()),
		Query: req.Msg.GetQuery(),
	}

	if p := req.Msg.GetPagination(); p != nil {
		limit, err := normalizeLimit(p.GetLimit())
		if err != nil {
			return nil, err
		}

		offset, err := normalizeOffset(p.GetOffset())
		if err != nil {
			return nil, err
		}

		params.Limit = limit
		params.Offset = offset
	}

	result, err := h.list.Execute(ctx, params)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*namespacev2.Namespace, 0, len(result.Namespaces))
	for _, ns := range result.Namespaces {
		protos = append(protos, domainNamespaceToProto(ns))
	}

	return connect.NewResponse(&namespacev2.ListNamespacesResponse{
		Namespaces: protos,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}

func (h *NamespaceHandler) DeleteNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.DeleteNamespaceRequest],
) (*connect.Response[namespacev2.DeleteNamespaceResponse], error) {
	if err := h.del.Execute(ctx, req.Msg.GetName()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.DeleteNamespaceResponse{}), nil
}

func (h *NamespaceHandler) LockNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.LockNamespaceRequest],
) (*connect.Response[namespacev2.LockNamespaceResponse], error) {
	if err := h.lock.Execute(ctx, req.Msg.GetName()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.LockNamespaceResponse{}), nil
}

func (h *NamespaceHandler) UnlockNamespace(
	ctx context.Context,
	req *connect.Request[namespacev2.UnlockNamespaceRequest],
) (*connect.Response[namespacev2.UnlockNamespaceResponse], error) {
	if err := h.unlock.Execute(ctx, req.Msg.GetName()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&namespacev2.UnlockNamespaceResponse{}), nil
}

func domainNamespaceToProto(ns *domain.Namespace) *namespacev2.Namespace {
	if ns == nil {
		return nil
	}

	return &namespacev2.Namespace{
		Name:        ns.Name,
		Description: ns.Description,
		ConfigCount: int32(ns.ConfigCount),
		Locked:      ns.Locked,
		CreatedAt:   timestamppb.New(ns.CreatedAt),
		UpdatedAt:   timestamppb.New(ns.UpdatedAt),
	}
}
