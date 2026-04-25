package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
)

type SchemaHandler struct {
	attach       *schemauc.AttachUseCase
	detach       *schemauc.DetachUseCase
	get          *schemauc.GetUseCase
	getEffective *schemauc.GetEffectiveUseCase
	list         *schemauc.ListUseCase
}

func NewSchemaHandler(
	attach *schemauc.AttachUseCase,
	detach *schemauc.DetachUseCase,
	get *schemauc.GetUseCase,
	getEffective *schemauc.GetEffectiveUseCase,
	list *schemauc.ListUseCase,
) *SchemaHandler {
	return &SchemaHandler{attach: attach, detach: detach, get: get, getEffective: getEffective, list: list}
}

func (h *SchemaHandler) AttachSchema(
	ctx context.Context,
	req *connect.Request[configv1.AttachSchemaRequest],
) (*connect.Response[configv1.AttachSchemaResponse], error) {
	s, err := h.attach.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPathPattern(), req.Msg.GetJsonSchema())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv1.AttachSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) DetachSchema(
	ctx context.Context,
	req *connect.Request[configv1.DetachSchemaRequest],
) (*connect.Response[configv1.DetachSchemaResponse], error) {
	if err := h.detach.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPathPattern()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv1.DetachSchemaResponse{}), nil
}

func (h *SchemaHandler) GetSchema(
	ctx context.Context,
	req *connect.Request[configv1.GetSchemaRequest],
) (*connect.Response[configv1.GetSchemaResponse], error) {
	s, err := h.get.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPathPattern())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv1.GetSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) GetEffectiveSchema(
	ctx context.Context,
	req *connect.Request[configv1.GetEffectiveSchemaRequest],
) (*connect.Response[configv1.GetEffectiveSchemaResponse], error) {
	s, err := h.getEffective.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPath())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv1.GetEffectiveSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) ListSchemas(
	ctx context.Context,
	req *connect.Request[configv1.ListSchemasRequest],
) (*connect.Response[configv1.ListSchemasResponse], error) {
	schemas, err := h.list.Execute(ctx, req.Msg.GetNamespace())
	if err != nil {
		return nil, toConnectError(err)
	}
	protos := make([]*configv1.SchemaAttachment, 0, len(schemas))
	for _, s := range schemas {
		protos = append(protos, domainSchemaToProto(s))
	}

	return connect.NewResponse(&configv1.ListSchemasResponse{Schemas: protos}), nil
}

func domainSchemaToProto(s *domain.SchemaAttachment) *configv1.SchemaAttachment {
	if s == nil {
		return nil
	}

	return &configv1.SchemaAttachment{
		Id:          s.ID,
		Namespace:   s.Namespace,
		PathPattern: s.PathPattern,
		JsonSchema:  s.JSONSchema,
		CreatedAt:   timestamppb.New(s.CreatedAt),
	}
}
