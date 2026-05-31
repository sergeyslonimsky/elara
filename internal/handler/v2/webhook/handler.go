package webhook

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	webhookv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=webhook_mock -source=handler.go

type (
	authz interface {
		Require(
			ctx context.Context,
			object domain.Object,
			action domain.Action,
			domainStr string,
		) error
		RequireNamespace(ctx context.Context, action domain.Action, name string) error
		RequireGroup(ctx context.Context, action domain.Action, id string) error
	}

	usecase interface {
		Create(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error)
		Get(ctx context.Context, id string) (*domain.Webhook, error)
		Update(
			ctx context.Context,
			id string,
			params webhookuc.UpdateParams,
		) (*domain.Webhook, error)
		Delete(ctx context.Context, id string) error
		List(ctx context.Context) ([]*domain.Webhook, error)
		GetHistory(ctx context.Context, webhookID string) ([]domain.DeliveryAttempt, error)
	}
)

type Handler struct {
	authz authz
	uc    usecase
}

func New(authz authz, uc usecase) *Handler {
	return &Handler{authz: authz, uc: uc}
}

func (h *Handler) CreateWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.CreateWebhookRequest],
) (*connect.Response[webhookv1.CreateWebhookResponse], error) {
	ns := req.Msg.GetNamespaceFilter()
	if ns == "" {
		ns = domain.DomainAll
	}

	if err := h.authz.Require(ctx, domain.ObjectWebhook, domain.ActionCreate, ns); err != nil {
		return nil, v2.ToConnectError(err)
	}

	// A webhook observes config changes in its target namespace, so its creator
	// must be able to read that namespace (write⊇read covers writers too).
	if err := h.authz.RequireNamespace(ctx, domain.ActionRead, ns); err != nil {
		return nil, v2.ToConnectError(err)
	}

	w := &domain.Webhook{
		URL:             req.Msg.GetUrl(),
		NamespaceFilter: req.Msg.GetNamespaceFilter(),
		PathPrefix:      req.Msg.GetPathPrefix(),
		Events:          protoEventsToDomain(req.Msg.GetEvents()),
		Secret:          req.Msg.GetSecret(),
		Enabled:         req.Msg.GetEnabled(),
	}

	result, err := h.uc.Create(ctx, w)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&webhookv1.CreateWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *Handler) GetWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.GetWebhookRequest],
) (*connect.Response[webhookv1.GetWebhookResponse], error) {
	result, err := h.uc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&webhookv1.GetWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *Handler) UpdateWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.UpdateWebhookRequest],
) (*connect.Response[webhookv1.UpdateWebhookResponse], error) {
	params := webhookuc.UpdateParams{
		URL:             req.Msg.GetUrl(),
		NamespaceFilter: req.Msg.GetNamespaceFilter(),
		PathPrefix:      req.Msg.GetPathPrefix(),
		Events:          protoEventsToDomain(req.Msg.GetEvents()),
		Secret:          req.Msg.GetSecret(),
		Enabled:         req.Msg.GetEnabled(),
	}

	result, err := h.uc.Update(ctx, req.Msg.GetId(), params)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&webhookv1.UpdateWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *Handler) DeleteWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.DeleteWebhookRequest],
) (*connect.Response[webhookv1.DeleteWebhookResponse], error) {
	if err := h.uc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&webhookv1.DeleteWebhookResponse{}), nil
}

func (h *Handler) ListWebhooks(
	ctx context.Context,
	_ *connect.Request[webhookv1.ListWebhooksRequest],
) (*connect.Response[webhookv1.ListWebhooksResponse], error) {
	results, err := h.uc.List(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*webhookv1.Webhook, 0, len(results))
	for _, w := range results {
		protos = append(protos, domainWebhookToProto(w))
	}

	return connect.NewResponse(&webhookv1.ListWebhooksResponse{
		Webhooks: protos,
	}), nil
}

func (h *Handler) GetDeliveryHistory(
	ctx context.Context,
	req *connect.Request[webhookv1.GetDeliveryHistoryRequest],
) (*connect.Response[webhookv1.GetDeliveryHistoryResponse], error) {
	attempts, err := h.uc.GetHistory(ctx, req.Msg.GetWebhookId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*webhookv1.DeliveryAttempt, 0, len(attempts))
	for _, a := range attempts {
		protos = append(protos, &webhookv1.DeliveryAttempt{
			AttemptNumber: int32(a.AttemptNumber),
			StatusCode:    int32(a.StatusCode),
			LatencyMs:     a.LatencyMS,
			Error:         a.Error,
			Success:       a.Success,
			Timestamp:     timestamppb.New(a.Timestamp),
		})
	}

	return connect.NewResponse(&webhookv1.GetDeliveryHistoryResponse{
		Attempts: protos,
	}), nil
}

func domainWebhookToProto(w *domain.Webhook) *webhookv1.Webhook {
	if w == nil {
		return nil
	}

	events := make([]webhookv1.WebhookEvent, 0, len(w.Events))
	for _, e := range w.Events {
		events = append(events, domainEventToProto(e))
	}

	return &webhookv1.Webhook{
		Id:              w.ID,
		Url:             w.URL,
		NamespaceFilter: w.NamespaceFilter,
		PathPrefix:      w.PathPrefix,
		Events:          events,
		Enabled:         w.Enabled,
		CreatedAt:       timestamppb.New(w.CreatedAt),
		UpdatedAt:       timestamppb.New(w.UpdatedAt),
	}
}

func protoEventsToDomain(events []webhookv1.WebhookEvent) []domain.WebhookEventType {
	out := make([]domain.WebhookEventType, 0, len(events))
	for _, e := range events {
		out = append(out, protoEventToDomain(e))
	}

	return out
}

func protoEventToDomain(e webhookv1.WebhookEvent) domain.WebhookEventType {
	switch e {
	case webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED:
		return domain.WebhookEventCreated
	case webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED:
		return domain.WebhookEventUpdated
	case webhookv1.WebhookEvent_WEBHOOK_EVENT_DELETED:
		return domain.WebhookEventDeleted
	default:
		return ""
	}
}

func domainEventToProto(e domain.WebhookEventType) webhookv1.WebhookEvent {
	switch e {
	case domain.WebhookEventCreated:
		return webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED
	case domain.WebhookEventUpdated:
		return webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED
	case domain.WebhookEventDeleted:
		return webhookv1.WebhookEvent_WEBHOOK_EVENT_DELETED
	default:
		return webhookv1.WebhookEvent_WEBHOOK_EVENT_UNSPECIFIED
	}
}
