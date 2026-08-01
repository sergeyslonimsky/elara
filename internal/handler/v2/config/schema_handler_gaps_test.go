package config_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	configmock "github.com/sergeyslonimsky/elara/internal/handler/v2/config/mocks"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
)

func TestSchemaHandler_DetachSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *config.SchemaHandler
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().Detach(gomock.Any(), "prod", "/*.json").Return(nil)

				return config.NewSchemaHandler(az, uc)
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").
					Return(domain.ErrUnauthorized)

				return config.NewSchemaHandler(az, configmock.NewMockschemaUsecase(ctrl))
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionWrite, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().Detach(gomock.Any(), "prod", "/*.json").Return(domain.ErrNotFound)

				return config.NewSchemaHandler(az, uc)
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.DetachSchema(t.Context(), connect.NewRequest(&configv1.DetachSchemaRequest{
				Namespace:   "prod",
				PathPattern: "/*.json",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, resp.Msg)
		})
	}
}

func TestSchemaHandler_GetSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *config.SchemaHandler
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), "prod", "/*.json").
					Return(&domain.SchemaAttachment{Namespace: "prod", PathPattern: "/*.json"}, nil)

				return config.NewSchemaHandler(az, uc)
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
					Return(domain.ErrUnauthorized)

				return config.NewSchemaHandler(az, configmock.NewMockschemaUsecase(ctrl))
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "not found",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), "prod", "/*.json").Return(nil, domain.ErrNotFound)

				return config.NewSchemaHandler(az, uc)
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetSchema(t.Context(), connect.NewRequest(&configv1.GetSchemaRequest{
				Namespace:   "prod",
				PathPattern: "/*.json",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "prod", resp.Msg.GetSchema().GetNamespace())
		})
	}
}

func TestSchemaHandler_GetEffectiveSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *config.SchemaHandler
		wantCode connect.Code
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().GetEffective(gomock.Any(), "prod", "/app/a.json").
					Return(&domain.SchemaAttachment{Namespace: "prod", PathPattern: "/app/*.json"}, nil)

				return config.NewSchemaHandler(az, uc)
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").
					Return(domain.ErrUnauthorized)

				return config.NewSchemaHandler(az, configmock.NewMockschemaUsecase(ctrl))
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "not found",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				az := configmock.NewMockschemaAuthz(ctrl)
				az.EXPECT().RequireNamespace(gomock.Any(), domain.ActionRead, "prod").Return(nil)

				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().GetEffective(gomock.Any(), "prod", "/app/a.json").
					Return(nil, domain.ErrNotFound)

				return config.NewSchemaHandler(az, uc)
			},
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetEffectiveSchema(t.Context(), connect.NewRequest(&configv1.GetEffectiveSchemaRequest{
				Namespace: "prod",
				Path:      "/app/a.json",
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}
			require.NoError(t, err)
			assert.Equal(t, "prod", resp.Msg.GetSchema().GetNamespace())
		})
	}
}

func TestSchemaHandler_ListSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *config.SchemaHandler
		wantErr  string
		wantLen  int
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().List(gomock.Any(), "prod").Return([]*domain.SchemaAttachment{
					{Namespace: "prod", PathPattern: "/a.json"},
					{Namespace: "prod", PathPattern: "/b.json"},
				}, nil)

				return config.NewSchemaHandler(configmock.NewMockschemaAuthz(ctrl), uc)
			},
			wantLen: 2,
		},
		{
			name: "usecase error propagates",
			mockFunc: func(ctrl *gomock.Controller) *config.SchemaHandler {
				uc := configmock.NewMockschemaUsecase(ctrl)
				uc.EXPECT().List(gomock.Any(), "prod").Return(nil, domain.ErrNotFound)

				return config.NewSchemaHandler(configmock.NewMockschemaAuthz(ctrl), uc)
			},
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ListSchemas(t.Context(), connect.NewRequest(&configv1.ListSchemasRequest{
				Namespace: "prod",
			}))

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetSchemas(), tt.wantLen)
		})
	}
}
