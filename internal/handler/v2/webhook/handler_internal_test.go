package webhook

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookmock "github.com/sergeyslonimsky/elara/internal/handler/v2/webhook/mocks"
	webhookv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

func TestHandler_CreateWebhook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *webhookv1.CreateWebhookRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  connect.Code
	}{
		{
			name: "success per-namespace gate",
			req: &webhookv1.CreateWebhookRequest{
				Url: "https://example.com/hook",
				Events: []webhookv1.WebhookEvent{
					webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED,
				},
				NamespaceFilter: "production",
				PathPrefix:      "/app",
				Enabled:         true,
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)

				az.EXPECT().
					Require(gomock.Any(), domain.ObjectWebhook, domain.ActionCreate, "production").
					Return(nil)
				az.EXPECT().
					RequireNamespace(gomock.Any(), domain.ActionRead, "production").
					Return(nil)
				uc.EXPECT().Create(gomock.Any(), gomock.Any()).
					Return(&domain.Webhook{
						ID:              "gen-id",
						URL:             "https://example.com/hook",
						NamespaceFilter: "production",
						PathPrefix:      "/app",
						Events:          []domain.WebhookEventType{domain.WebhookEventCreated},
						Enabled:         true,
					}, nil)

				return New(az, uc)
			},
		},
		{
			name: "global webhook gates on wildcard domain",
			req: &webhookv1.CreateWebhookRequest{
				Url:    "https://example.com/hook",
				Events: []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)

				az.EXPECT().
					Require(gomock.Any(), domain.ObjectWebhook, domain.ActionCreate, domain.DomainAll).
					Return(nil)
				az.EXPECT().
					RequireNamespace(gomock.Any(), domain.ActionRead, domain.DomainAll).
					Return(nil)
				uc.EXPECT().Create(gomock.Any(), gomock.Any()).
					Return(&domain.Webhook{ID: "gen-id", URL: "https://example.com/hook"}, nil)

				return New(az, uc)
			},
		},
		{
			name: "forbidden short-circuits before usecase",
			req: &webhookv1.CreateWebhookRequest{
				Url: "https://example.com/hook",
				Events: []webhookv1.WebhookEvent{
					webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED,
				},
				NamespaceFilter: "prod",
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)

				az.EXPECT().
					Require(gomock.Any(), domain.ObjectWebhook, domain.ActionCreate, "prod").
					Return(domain.ErrForbidden)

				return New(az, uc)
			},
			wantErr: connect.CodePermissionDenied,
		},
		{
			name: "invalid argument missing url",
			req: &webhookv1.CreateWebhookRequest{
				Events: []webhookv1.WebhookEvent{
					webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED,
				},
				NamespaceFilter: "prod",
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)

				az.EXPECT().
					Require(gomock.Any(), domain.ObjectWebhook, domain.ActionCreate, "prod").
					Return(nil)
				az.EXPECT().
					RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
					Return(nil)
				uc.EXPECT().Create(gomock.Any(), gomock.Any()).
					Return(nil, domain.NewValidationError("url", "url is required"))

				return New(az, uc)
			},
			wantErr: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.CreateWebhook(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "gen-id", resp.Msg.GetWebhook().GetId())
			assert.Equal(t, tt.req.GetUrl(), resp.Msg.GetWebhook().GetUrl())
		})
	}
}

func TestHandler_GetWebhook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		mockFunc func(*gomock.Controller) *Handler
		wantErr  connect.Code
	}{
		{
			name: "success delegates to usecase",
			id:   "wh-1",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), "wh-1").Return(&domain.Webhook{
					ID:              "wh-1",
					URL:             "https://example.com/hook",
					NamespaceFilter: "prod",
				}, nil)

				return New(az, uc)
			},
		},
		{
			name: "not found",
			id:   "missing",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), "missing").Return(nil, domain.ErrNotFound)

				return New(az, uc)
			},
			wantErr: connect.CodeNotFound,
		},
		{
			name: "forbidden propagates from usecase",
			id:   "wh-1",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), "wh-1").Return(nil, domain.ErrForbidden)

				return New(az, uc)
			},
			wantErr: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetWebhook(
				t.Context(),
				connect.NewRequest(&webhookv1.GetWebhookRequest{Id: tt.id}),
			)

			if tt.wantErr != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.id, resp.Msg.GetWebhook().GetId())
		})
	}
}

func TestHandler_UpdateWebhook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *webhookv1.UpdateWebhookRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  connect.Code
	}{
		{
			name: "success delegates to usecase",
			req: &webhookv1.UpdateWebhookRequest{
				Id:      "wh-1",
				Url:     "https://new.example.com/hook",
				Events:  []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_UPDATED},
				Enabled: false,
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().
					Update(gomock.Any(), "wh-1", webhookuc.UpdateParams{
						URL:     "https://new.example.com/hook",
						Events:  []domain.WebhookEventType{domain.WebhookEventUpdated},
						Enabled: false,
					}).
					Return(&domain.Webhook{
						ID:      "wh-1",
						URL:     "https://new.example.com/hook",
						Events:  []domain.WebhookEventType{domain.WebhookEventUpdated},
						Enabled: false,
					}, nil)

				return New(az, uc)
			},
		},
		{
			name: "not found",
			req: &webhookv1.UpdateWebhookRequest{
				Id:     "missing",
				Url:    "https://example.com/hook",
				Events: []webhookv1.WebhookEvent{webhookv1.WebhookEvent_WEBHOOK_EVENT_CREATED},
			},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().
					Update(gomock.Any(), "missing", gomock.Any()).
					Return(nil, domain.ErrNotFound)

				return New(az, uc)
			},
			wantErr: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.UpdateWebhook(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.req.GetUrl(), resp.Msg.GetWebhook().GetUrl())
			assert.False(t, resp.Msg.GetWebhook().GetEnabled())
		})
	}
}

func TestHandler_DeleteWebhook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		mockFunc func(*gomock.Controller) *Handler
		wantErr  connect.Code
	}{
		{
			name: "success",
			id:   "wh-1",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().Delete(gomock.Any(), "wh-1").Return(nil)

				return New(az, uc)
			},
		},
		{
			name: "not found",
			id:   "missing",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().Delete(gomock.Any(), "missing").Return(domain.ErrNotFound)

				return New(az, uc)
			},
			wantErr: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			_, err := h.DeleteWebhook(
				t.Context(),
				connect.NewRequest(&webhookv1.DeleteWebhookRequest{Id: tt.id}),
			)

			if tt.wantErr != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestHandler_ListWebhooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Handler
		wantLen  int
	}{
		{
			name: "success empty",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().List(gomock.Any()).Return([]*domain.Webhook{}, nil)

				return New(az, uc)
			},
			wantLen: 0,
		},
		{
			name: "success populated",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().List(gomock.Any()).Return([]*domain.Webhook{
					{ID: "wh-1", URL: "https://a.com", NamespaceFilter: "prod"},
					{ID: "wh-2", URL: "https://b.com", NamespaceFilter: "dev"},
				}, nil)

				return New(az, uc)
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ListWebhooks(
				t.Context(),
				connect.NewRequest(&webhookv1.ListWebhooksRequest{}),
			)

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetWebhooks(), tt.wantLen)
		})
	}
}

func TestHandler_GetDeliveryHistory(t *testing.T) {
	t.Parallel()

	attempts := []domain.DeliveryAttempt{
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

	tests := []struct {
		name     string
		id       string
		mockFunc func(*gomock.Controller) *Handler
		wantErr  connect.Code
	}{
		{
			name: "success",
			id:   "wh-1",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().GetHistory(gomock.Any(), "wh-1").Return(attempts, nil)

				return New(az, uc)
			},
		},
		{
			name: "not found",
			id:   "unknown",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := webhookmock.NewMockauthz(ctrl)
				uc := webhookmock.NewMockusecase(ctrl)
				uc.EXPECT().GetHistory(gomock.Any(), "unknown").Return(nil, domain.ErrNotFound)

				return New(az, uc)
			},
			wantErr: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetDeliveryHistory(
				t.Context(),
				connect.NewRequest(&webhookv1.GetDeliveryHistoryRequest{WebhookId: tt.id}),
			)

			if tt.wantErr != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetAttempts(), len(attempts))
		})
	}
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
