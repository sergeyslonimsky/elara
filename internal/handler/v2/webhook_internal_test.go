package v2

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

// fakeWebhookRepo satisfies all storage interfaces used by webhook use cases.
type fakeWebhookRepo struct {
	webhooks  map[string]*domain.Webhook
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeWebhookRepo() *fakeWebhookRepo {
	return &fakeWebhookRepo{webhooks: make(map[string]*domain.Webhook)}
}

func (r *fakeWebhookRepo) Create(_ context.Context, w *domain.Webhook) error {
	if r.createErr != nil {
		return r.createErr
	}

	w.ID = "gen-id"
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	r.webhooks[w.ID] = w

	return nil
}

func (r *fakeWebhookRepo) Get(_ context.Context, id string) (*domain.Webhook, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}

	w, ok := r.webhooks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return w, nil
}

func (r *fakeWebhookRepo) Update(_ context.Context, w *domain.Webhook) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.webhooks[w.ID] = w

	return nil
}

func (r *fakeWebhookRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}

	if _, ok := r.webhooks[id]; !ok {
		return domain.ErrNotFound
	}

	delete(r.webhooks, id)

	return nil
}

func (r *fakeWebhookRepo) List(_ context.Context) ([]*domain.Webhook, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]*domain.Webhook, 0, len(r.webhooks))
	for _, w := range r.webhooks {
		out = append(out, w)
	}

	return out, nil
}

func (r *fakeWebhookRepo) seed(w *domain.Webhook) {
	if w.ID == "" {
		w.ID = "seed-id"
	}

	r.webhooks[w.ID] = w
}

// fakeWebhookDispatcher satisfies dispatcher interfaces used by delete and history use cases.
type fakeWebhookDispatcher struct {
	history    map[string][]domain.DeliveryAttempt
	clearedIDs []string
}

func newFakeWebhookDispatcher() *fakeWebhookDispatcher {
	return &fakeWebhookDispatcher{history: make(map[string][]domain.DeliveryAttempt)}
}

func (d *fakeWebhookDispatcher) GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt {
	return d.history[webhookID]
}

func (d *fakeWebhookDispatcher) ClearHistory(webhookID string) {
	d.clearedIDs = append(d.clearedIDs, webhookID)
	delete(d.history, webhookID)
}

func newTestWebhookHandler(repo *fakeWebhookRepo, dispatcher *fakeWebhookDispatcher) *WebhookHandler {
	return NewWebhookHandler(
		webhookuc.NewCreateUseCase(repo),
		webhookuc.NewGetUseCase(repo),
		webhookuc.NewUpdateUseCase(repo),
		webhookuc.NewDeleteUseCase(repo, dispatcher),
		webhookuc.NewListUseCase(repo),
		webhookuc.NewHistoryUseCase(dispatcher),
	)
}

// -----------------------------------------------------------------------------
// CreateWebhook
// -----------------------------------------------------------------------------

func TestWebhookHandler_CreateWebhook_Success(t *testing.T) {
	t.Parallel()

	repo := newFakeWebhookRepo()
	h := newTestWebhookHandler(repo, newFakeWebhookDispatcher())

	resp, err := h.CreateWebhook(context.Background(), connect.NewRequest(&webhookv1.CreateWebhookRequest{
		Url:             "https://example.com/hook",
		Events:          []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
		NamespaceFilter: "production",
		PathPrefix:      "/app",
		Enabled:         true,
	}))

	require.NoError(t, err)
	assert.Equal(t, "gen-id", resp.Msg.GetWebhook().GetId())
	assert.Equal(t, "https://example.com/hook", resp.Msg.GetWebhook().GetUrl())
	assert.Equal(t, "production", resp.Msg.GetWebhook().GetNamespaceFilter())
	assert.Equal(t, "/app", resp.Msg.GetWebhook().GetPathPrefix())
	assert.True(t, resp.Msg.GetWebhook().GetEnabled())
	assert.Equal(
		t,
		[]webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
		resp.Msg.GetWebhook().GetEvents(),
	)
}

func TestWebhookHandler_CreateWebhook_MissingURL_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	_, err := h.CreateWebhook(context.Background(), connect.NewRequest(&webhookv1.CreateWebhookRequest{
		Events: []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestWebhookHandler_CreateWebhook_NoEvents_ReturnsInvalidArgument(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	_, err := h.CreateWebhook(context.Background(), connect.NewRequest(&webhookv1.CreateWebhookRequest{
		Url: "https://example.com/hook",
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// GetWebhook
// -----------------------------------------------------------------------------

func TestWebhookHandler_GetWebhook_Success(t *testing.T) {
	t.Parallel()

	repo := newFakeWebhookRepo()
	repo.seed(&domain.Webhook{
		ID:      "wh-1",
		URL:     "https://example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventUpdated},
		Enabled: true,
	})

	h := newTestWebhookHandler(repo, newFakeWebhookDispatcher())

	resp, err := h.GetWebhook(context.Background(), connect.NewRequest(&webhookv1.GetWebhookRequest{Id: "wh-1"}))

	require.NoError(t, err)
	assert.Equal(t, "wh-1", resp.Msg.GetWebhook().GetId())
	assert.Equal(t, "https://example.com/hook", resp.Msg.GetWebhook().GetUrl())
}

func TestWebhookHandler_GetWebhook_NotFound(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	_, err := h.GetWebhook(context.Background(), connect.NewRequest(&webhookv1.GetWebhookRequest{Id: "missing"}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// UpdateWebhook
// -----------------------------------------------------------------------------

func TestWebhookHandler_UpdateWebhook_Success(t *testing.T) {
	t.Parallel()

	repo := newFakeWebhookRepo()
	repo.seed(&domain.Webhook{
		ID:      "wh-1",
		URL:     "https://old.example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
		Enabled: true,
	})

	h := newTestWebhookHandler(repo, newFakeWebhookDispatcher())

	resp, err := h.UpdateWebhook(context.Background(), connect.NewRequest(&webhookv1.UpdateWebhookRequest{
		Id:      "wh-1",
		Url:     "https://new.example.com/hook",
		Events:  []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED},
		Enabled: false,
	}))

	require.NoError(t, err)
	assert.Equal(t, "https://new.example.com/hook", resp.Msg.GetWebhook().GetUrl())
	assert.False(t, resp.Msg.GetWebhook().GetEnabled())
	assert.Equal(
		t,
		[]webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED},
		resp.Msg.GetWebhook().GetEvents(),
	)
}

func TestWebhookHandler_UpdateWebhook_NotFound(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	_, err := h.UpdateWebhook(context.Background(), connect.NewRequest(&webhookv1.UpdateWebhookRequest{
		Id:     "missing",
		Url:    "https://example.com/hook",
		Events: []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
	}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// DeleteWebhook
// -----------------------------------------------------------------------------

func TestWebhookHandler_DeleteWebhook_Success(t *testing.T) {
	t.Parallel()

	repo := newFakeWebhookRepo()
	dispatcher := newFakeWebhookDispatcher()
	repo.seed(&domain.Webhook{
		ID:      "wh-1",
		URL:     "https://example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
		Enabled: true,
	})

	h := newTestWebhookHandler(repo, dispatcher)

	_, err := h.DeleteWebhook(context.Background(), connect.NewRequest(&webhookv1.DeleteWebhookRequest{Id: "wh-1"}))

	require.NoError(t, err)
	assert.Contains(t, dispatcher.clearedIDs, "wh-1")
}

func TestWebhookHandler_DeleteWebhook_NotFound(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	_, err := h.DeleteWebhook(context.Background(), connect.NewRequest(&webhookv1.DeleteWebhookRequest{Id: "missing"}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// ListWebhooks
// -----------------------------------------------------------------------------

func TestWebhookHandler_ListWebhooks_Empty(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	resp, err := h.ListWebhooks(context.Background(), connect.NewRequest(&webhookv1.ListWebhooksRequest{}))

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.GetWebhooks())
}

func TestWebhookHandler_ListWebhooks_ReturnsAll(t *testing.T) {
	t.Parallel()

	repo := newFakeWebhookRepo()
	repo.seed(&domain.Webhook{
		ID:      "wh-1",
		URL:     "https://a.example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventCreated},
		Enabled: true,
	})
	repo.seed(&domain.Webhook{
		ID:      "wh-2",
		URL:     "https://b.example.com/hook",
		Events:  []domain.WebhookEventType{domain.WebhookEventDeleted},
		Enabled: false,
	})

	h := newTestWebhookHandler(repo, newFakeWebhookDispatcher())

	resp, err := h.ListWebhooks(context.Background(), connect.NewRequest(&webhookv1.ListWebhooksRequest{}))

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetWebhooks(), 2)
}

// -----------------------------------------------------------------------------
// GetDeliveryHistory
// -----------------------------------------------------------------------------

func TestWebhookHandler_GetDeliveryHistory_ReturnsAttempts(t *testing.T) {
	t.Parallel()

	dispatcher := newFakeWebhookDispatcher()
	dispatcher.history["wh-1"] = []domain.DeliveryAttempt{
		{AttemptNumber: 1, StatusCode: 200, LatencyMS: 42, Success: true, Timestamp: time.Now()},
		{
			AttemptNumber: 2,
			StatusCode:    500,
			LatencyMS:     10,
			Success:       false,
			Error:         "server error",
			Timestamp:     time.Now(),
		},
	}

	h := newTestWebhookHandler(newFakeWebhookRepo(), dispatcher)

	resp, err := h.GetDeliveryHistory(
		context.Background(),
		connect.NewRequest(&webhookv1.GetDeliveryHistoryRequest{WebhookId: "wh-1"}),
	)

	require.NoError(t, err)
	require.Len(t, resp.Msg.GetAttempts(), 2)
	assert.Equal(t, int32(1), resp.Msg.GetAttempts()[0].GetAttemptNumber())
	assert.Equal(t, int32(200), resp.Msg.GetAttempts()[0].GetStatusCode())
	assert.True(t, resp.Msg.GetAttempts()[0].GetSuccess())
	assert.Equal(t, int32(500), resp.Msg.GetAttempts()[1].GetStatusCode())
	assert.Equal(t, "server error", resp.Msg.GetAttempts()[1].GetError())
}

func TestWebhookHandler_GetDeliveryHistory_UnknownWebhook_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	h := newTestWebhookHandler(newFakeWebhookRepo(), newFakeWebhookDispatcher())

	resp, err := h.GetDeliveryHistory(
		context.Background(),
		connect.NewRequest(&webhookv1.GetDeliveryHistoryRequest{WebhookId: "unknown"}),
	)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.GetAttempts())
}

// -----------------------------------------------------------------------------
// Event conversion helpers
// -----------------------------------------------------------------------------

func TestProtoEventToDomain_AllValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input webhookv1.WebhookEvent
		want  domain.WebhookEventType
	}{
		{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED, domain.WebhookEventCreated},
		{webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED, domain.WebhookEventUpdated},
		{webhookv1.WebhookEvent_WEBHOOK_EVENT_DELETED, domain.WebhookEventDeleted},
		{webhookv1.WebhookEvent_WEBHOOK_EVENT_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, protoEventToDomain(tt.input))
		})
	}
}

func TestDomainEventToProto_AllValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input domain.WebhookEventType
		want  webhookv1.WebhookEvent
	}{
		{domain.WebhookEventCreated, webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
		{domain.WebhookEventUpdated, webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED},
		{domain.WebhookEventDeleted, webhookv1.WebhookEvent_WEBHOOK_EVENT_DELETED},
		{"unknown", webhookv1.WebhookEvent_WEBHOOK_EVENT_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, domainEventToProto(tt.input))
		})
	}
}

func TestDomainWebhookToProto_NilReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, domainWebhookToProto(nil))
}
