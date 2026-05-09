package config

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
)

//go:generate mockgen -destination=mocks/schema_handler_mock.go -package=config_mock -source=schema_handler.go

type schemaUsecase interface {
	Attach(ctx context.Context, in schemauc.AttachInput) (*domain.SchemaAttachment, error)
	Detach(ctx context.Context, namespace, pathPattern string) error
	Get(ctx context.Context, namespace, pathPattern string) (*domain.SchemaAttachment, error)
	GetEffective(ctx context.Context, namespace, path string) (*domain.SchemaAttachment, error)
	List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
}

type SchemaHandler struct {
	uc schemaUsecase
}

func NewSchemaHandler(uc schemaUsecase) *SchemaHandler {
	return &SchemaHandler{uc: uc}
}

func (h *SchemaHandler) AttachSchema(
	ctx context.Context,
	req *connect.Request[configv1.AttachSchemaRequest],
) (*connect.Response[configv1.AttachSchemaResponse], error) {
	s, err := h.uc.Attach(ctx, schemauc.AttachInput{
		Namespace:   req.Msg.GetNamespace(),
		PathPattern: req.Msg.GetPathPattern(),
		JSONSchema:  req.Msg.GetJsonSchema(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&configv1.AttachSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) DetachSchema(
	ctx context.Context,
	req *connect.Request[configv1.DetachSchemaRequest],
) (*connect.Response[configv1.DetachSchemaResponse], error) {
	if err := h.uc.Detach(ctx, req.Msg.GetNamespace(), req.Msg.GetPathPattern()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&configv1.DetachSchemaResponse{}), nil
}

func (h *SchemaHandler) GetSchema(
	ctx context.Context,
	req *connect.Request[configv1.GetSchemaRequest],
) (*connect.Response[configv1.GetSchemaResponse], error) {
	s, err := h.uc.Get(ctx, req.Msg.GetNamespace(), req.Msg.GetPathPattern())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&configv1.GetSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) GetEffectiveSchema(
	ctx context.Context,
	req *connect.Request[configv1.GetEffectiveSchemaRequest],
) (*connect.Response[configv1.GetEffectiveSchemaResponse], error) {
	s, err := h.uc.GetEffective(ctx, req.Msg.GetNamespace(), req.Msg.GetPath())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&configv1.GetEffectiveSchemaResponse{
		Schema: domainSchemaToProto(s),
	}), nil
}

func (h *SchemaHandler) ListSchemas(
	ctx context.Context,
	req *connect.Request[configv1.ListSchemasRequest],
) (*connect.Response[configv1.ListSchemasResponse], error) {
	schemas, err := h.uc.List(ctx, req.Msg.GetNamespace())
	if err != nil {
		return nil, v2.ToConnectError(err)
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
