package transfer_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/transfer"
	transfermock "github.com/sergeyslonimsky/elara/internal/handler/v2/transfer/mocks"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

func TestHandler_ExportNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *transferv1.ExportNamespaceRequest
		mockFunc func(*gomock.Controller) *transfer.Handler
		errCode  connect.Code
		want     *transferv1.ExportNamespaceResponse
	}{
		{
			name: "success",
			req: &transferv1.ExportNamespaceRequest{
				Namespace: "prod",
				Encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionRead, "prod").
					Return(nil)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					ExportNamespace(gomock.Any(), "prod", false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON).
					Return([]byte("{}"), "application/json", "prod-export.json", nil)

				return transfer.New(az, uc)
			},
			want: &transferv1.ExportNamespaceResponse{
				ContentType: "application/json",
				Filename:    "prod-export.json",
			},
		},
		{
			name: "unauthorized",
			req:  &transferv1.ExportNamespaceRequest{Namespace: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionRead, "prod").
					Return(domain.ErrUnauthorized)
				uc := transfermock.NewMockusecase(ctrl)

				return transfer.New(az, uc)
			},
			errCode: connect.CodeUnauthenticated,
		},
		{
			name: "forbidden",
			req:  &transferv1.ExportNamespaceRequest{Namespace: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionRead, "prod").
					Return(domain.ErrForbidden)
				uc := transfermock.NewMockusecase(ctrl)

				return transfer.New(az, uc)
			},
			errCode: connect.CodePermissionDenied,
		},
		{
			name: "not found",
			req:  &transferv1.ExportNamespaceRequest{Namespace: "missing"},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionRead, "missing").
					Return(nil)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					ExportNamespace(gomock.Any(), "missing", gomock.Any(), gomock.Any()).
					Return(nil, "", "", domain.ErrNotFound)

				return transfer.New(az, uc)
			},
			errCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ExportNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.errCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.errCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.GetContentType(), resp.Msg.GetContentType())
			assert.Equal(t, tt.want.GetFilename(), resp.Msg.GetFilename())
			assert.NotEmpty(t, resp.Msg.GetData())
		})
	}
}

func TestHandler_ExportAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *transferv1.ExportAllRequest
		mockFunc func(*gomock.Controller) *transfer.Handler
		errCode  connect.Code
		want     *transferv1.ExportAllResponse
	}{
		{
			name: "success",
			req: &transferv1.ExportAllRequest{
				Encoding: transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					ExportAll(gomock.Any(), false, transferv1.BundleEncoding_BUNDLE_ENCODING_JSON, transferv1.ZipLayout(0)).
					Return([]byte("{}"), "application/json", "elara-export-all.json", nil)

				return transfer.New(az, uc)
			},
			want: &transferv1.ExportAllResponse{
				ContentType: "application/json",
				Filename:    "elara-export-all.json",
			},
		},
		{
			name: "usecase error",
			req:  &transferv1.ExportAllRequest{},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					ExportAll(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, "", "", domain.ErrUnauthorized)

				return transfer.New(az, uc)
			},
			errCode: connect.CodeUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ExportAll(t.Context(), connect.NewRequest(tt.req))

			if tt.errCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.errCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.GetContentType(), resp.Msg.GetContentType())
			assert.Equal(t, tt.want.GetFilename(), resp.Msg.GetFilename())
			assert.NotEmpty(t, resp.Msg.GetData())
		})
	}
}

func TestHandler_ImportNamespace(t *testing.T) {
	t.Parallel()

	validBundle := []byte(`{"namespace": "prod", "configs": []}`)

	tests := []struct {
		name     string
		req      *transferv1.ImportNamespaceRequest
		mockFunc func(*gomock.Controller) *transfer.Handler
		errCode  connect.Code
		want     *transferv1.ImportNamespaceResponse
	}{
		{
			name: "success",
			req: &transferv1.ImportNamespaceRequest{
				Data:      validBundle,
				Namespace: "prod",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionWrite, "prod").
					Return(nil)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					Import(gomock.Any(), validBundle, transferv1.ConflictResolution(0), false, "prod").
					Return(&domain.ImportReport{}, nil)

				return transfer.New(az, uc)
			},
			want: &transferv1.ImportNamespaceResponse{Created: 0},
		},
		{
			name: "invalid bundle",
			req: &transferv1.ImportNamespaceRequest{
				Data:      []byte(`invalid`),
				Namespace: "prod",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionWrite, "prod").
					Return(nil)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					Import(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, domain.NewValidationError("data", "invalid bundle"))

				return transfer.New(az, uc)
			},
			errCode: connect.CodeInvalidArgument,
		},
		{
			name: "forbidden",
			req: &transferv1.ImportNamespaceRequest{
				Data: validBundle,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionWrite, "").
					Return(domain.ErrForbidden)
				uc := transfermock.NewMockusecase(ctrl)

				return transfer.New(az, uc)
			},
			errCode: connect.CodePermissionDenied,
		},
		{
			name: "unauthorized",
			req: &transferv1.ImportNamespaceRequest{
				Data: validBundle,
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionWrite, "").
					Return(domain.ErrUnauthorized)
				uc := transfermock.NewMockusecase(ctrl)

				return transfer.New(az, uc)
			},
			errCode: connect.CodeUnauthenticated,
		},
		{
			name: "storage error",
			req: &transferv1.ImportNamespaceRequest{
				Data:      validBundle,
				Namespace: "prod",
			},
			mockFunc: func(ctrl *gomock.Controller) *transfer.Handler {
				az := transfermock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectTransfer, domain.ActionWrite, "prod").
					Return(nil)
				uc := transfermock.NewMockusecase(ctrl)
				uc.EXPECT().
					Import(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)

				return transfer.New(az, uc)
			},
			errCode: connect.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ImportNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.errCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.errCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.GetCreated(), resp.Msg.GetCreated())
		})
	}
}
