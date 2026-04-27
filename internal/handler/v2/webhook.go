package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type WebhookHandler struct {
	create  *webhookuc.CreateUseCase
	get     *webhookuc.GetUseCase
	update  *webhookuc.UpdateUseCase
	del     *webhookuc.DeleteUseCase
	list    *webhookuc.ListUseCase
	history *webhookuc.HistoryUseCase
}

func NewWebhookHandler(
	create *webhookuc.CreateUseCase,
	get *webhookuc.GetUseCase,
	update *webhookuc.UpdateUseCase,
	del *webhookuc.DeleteUseCase,
	list *webhookuc.ListUseCase,
	history *webhookuc.HistoryUseCase,
) *WebhookHandler {
	return &WebhookHandler{
		create:  create,
		get:     get,
		update:  update,
		del:     del,
		list:    list,
		history: history,
	}
}

func (h *WebhookHandler) CreateWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.CreateWebhookRequest],
) (*connect.Response[webhookv1.CreateWebhookResponse], error) {
	events := make([]domain.WebhookEventType, 0, len(req.Msg.GetEvents()))
	for _, e := range req.Msg.GetEvents() {
		events = append(events, protoEventToDomain(e))
	}

	w := &domain.Webhook{
		URL:             req.Msg.GetUrl(),
		NamespaceFilter: req.Msg.GetNamespaceFilter(),
		PathPrefix:      req.Msg.GetPathPrefix(),
		Events:          events,
		Secret:          req.Msg.GetSecret(),
		Enabled:         req.Msg.GetEnabled(),
	}

	result, err := h.create.Execute(ctx, w)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&webhookv1.CreateWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *WebhookHandler) GetWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.GetWebhookRequest],
) (*connect.Response[webhookv1.GetWebhookResponse], error) {
	result, err := h.get.Execute(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&webhookv1.GetWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *WebhookHandler) UpdateWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.UpdateWebhookRequest],
) (*connect.Response[webhookv1.UpdateWebhookResponse], error) {
	events := make([]domain.WebhookEventType, 0, len(req.Msg.GetEvents()))
	for _, e := range req.Msg.GetEvents() {
		events = append(events, protoEventToDomain(e))
	}

	params := webhookuc.UpdateParams{
		URL:             req.Msg.GetUrl(),
		NamespaceFilter: req.Msg.GetNamespaceFilter(),
		PathPrefix:      req.Msg.GetPathPrefix(),
		Events:          events,
		Secret:          req.Msg.GetSecret(),
		Enabled:         req.Msg.GetEnabled(),
	}

	result, err := h.update.Execute(ctx, req.Msg.GetId(), params)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&webhookv1.UpdateWebhookResponse{
		Webhook: domainWebhookToProto(result),
	}), nil
}

func (h *WebhookHandler) DeleteWebhook(
	ctx context.Context,
	req *connect.Request[webhookv1.DeleteWebhookRequest],
) (*connect.Response[webhookv1.DeleteWebhookResponse], error) {
	if err := h.del.Execute(ctx, req.Msg.GetId()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&webhookv1.DeleteWebhookResponse{}), nil
}

func (h *WebhookHandler) ListWebhooks(
	ctx context.Context,
	_ *connect.Request[webhookv1.ListWebhooksRequest],
) (*connect.Response[webhookv1.ListWebhooksResponse], error) {
	results, err := h.list.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*webhookv1.Webhook, 0, len(results))
	for _, w := range results {
		protos = append(protos, domainWebhookToProto(w))
	}

	return connect.NewResponse(&webhookv1.ListWebhooksResponse{
		Webhooks: protos,
	}), nil
}

func (h *WebhookHandler) GetDeliveryHistory(
	_ context.Context,
	req *connect.Request[webhookv1.GetDeliveryHistoryRequest],
) (*connect.Response[webhookv1.GetDeliveryHistoryResponse], error) {
	attempts := h.history.Execute(req.Msg.GetWebhookId())

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
