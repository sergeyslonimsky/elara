package namespace

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	namespace_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/namespace/mocks"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	namespacev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

func TestHandler_GetNamespace(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	tests := []struct {
		name     string
		req      *namespacev1.GetNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.GetNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.GetNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionRead, "prod").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Get(gomock.Any(), "prod").
					Return(&domain.Namespace{
						Name:        "prod",
						ConfigCount: 5,
						CreatedAt:   now,
						UpdatedAt:   now,
					}, nil)

				return New(az, uc)
			},
			want: &namespacev1.GetNamespaceResponse{
				Namespace: &namespacev1.Namespace{
					Name:        "prod",
					ConfigCount: 5,
					CreatedAt:   timestamppb.New(now),
					UpdatedAt:   timestamppb.New(now),
				},
			},
		},
		{
			name: "unauthorized",
			req:  &namespacev1.GetNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionRead, "prod").
					Return(domain.ErrUnauthorized)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "unauthorized",
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "forbidden",
			req:  &namespacev1.GetNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionRead, "prod").
					Return(domain.ErrForbidden)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "forbidden",
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "not found",
			req:  &namespacev1.GetNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionRead, "prod").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Get(gomock.Any(), "prod").
					Return(nil, domain.ErrNotFound)

				return New(az, uc)
			},
			wantErr:  "not found",
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_CreateNamespace(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	tests := []struct {
		name     string
		req      *namespacev1.CreateNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.CreateNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.CreateNamespaceRequest{Name: "new-ns", Description: "desc"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionCreate, domain.DomainAll).
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Create(gomock.Any(), &domain.Namespace{Name: "new-ns", Description: "desc"}).
					Return(&domain.Namespace{
						Name:        "new-ns",
						Description: "desc",
						CreatedAt:   now,
						UpdatedAt:   now,
					}, nil)

				return New(az, uc)
			},
			want: &namespacev1.CreateNamespaceResponse{
				Namespace: &namespacev1.Namespace{
					Name:        "new-ns",
					Description: "desc",
					CreatedAt:   timestamppb.New(now),
					UpdatedAt:   timestamppb.New(now),
				},
			},
		},
		{
			name: "unauthorized",
			req:  &namespacev1.CreateNamespaceRequest{Name: "new-ns"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionCreate, domain.DomainAll).
					Return(domain.ErrUnauthorized)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "unauthorized",
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "already exists",
			req:  &namespacev1.CreateNamespaceRequest{Name: "existing"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionCreate, domain.DomainAll).
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(nil, domain.ErrAlreadyExists)

				return New(az, uc)
			},
			wantErr:  "already exists",
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.CreateNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_UpdateNamespace(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	tests := []struct {
		name     string
		req      *namespacev1.UpdateNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.UpdateNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.UpdateNamespaceRequest{Name: "prod", Description: "new-desc"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Update(gomock.Any(), "prod", "new-desc").
					Return(&domain.Namespace{
						Name:        "prod",
						Description: "new-desc",
						ConfigCount: 10,
						CreatedAt:   now,
						UpdatedAt:   now,
					}, nil)

				return New(az, uc)
			},
			want: &namespacev1.UpdateNamespaceResponse{
				Namespace: &namespacev1.Namespace{
					Name:        "prod",
					Description: "new-desc",
					ConfigCount: 10,
					CreatedAt:   timestamppb.New(now),
					UpdatedAt:   timestamppb.New(now),
				},
			},
		},
		{
			name: "unauthorized",
			req:  &namespacev1.UpdateNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(domain.ErrUnauthorized)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "unauthorized",
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name: "forbidden",
			req:  &namespacev1.UpdateNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(domain.ErrForbidden)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "forbidden",
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.UpdateNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_ListNamespaces(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	tests := []struct {
		name     string
		req      *namespacev1.ListNamespacesRequest
		mockFunc func(context.Context, *gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.ListNamespacesResponse
	}{
		{
			name: "success",
			req: &namespacev1.ListNamespacesRequest{
				Query: "prod",
				Pagination: &commonv1.PaginationRequest{
					Limit:  10,
					Offset: 0,
				},
			},
			mockFunc: func(_ context.Context, ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					List(gomock.Any(), nsuc.ListParams{
						Query:  "prod",
						Limit:  10,
						Offset: 0,
					}).
					Return(&nsuc.ListResult{
						Namespaces: []*domain.Namespace{
							{
								Name:        "prod",
								ConfigCount: 5,
								CreatedAt:   now,
								UpdatedAt:   now,
							},
						},
						Total:  1,
						Limit:  10,
						Offset: 0,
					}, nil)

				return New(az, uc)
			},
			want: &namespacev1.ListNamespacesResponse{
				Namespaces: []*namespacev1.Namespace{
					{
						Name:        "prod",
						ConfigCount: 5,
						CreatedAt:   timestamppb.New(now),
						UpdatedAt:   timestamppb.New(now),
					},
				},
				Pagination: &commonv1.PaginationResponse{
					Total:  1,
					Limit:  10,
					Offset: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.ListNamespaces(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_DeleteNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *namespacev1.DeleteNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.DeleteNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.DeleteNamespaceRequest{Name: "empty-ns"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionDelete, "empty-ns").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Delete(gomock.Any(), "empty-ns").
					Return(nil)

				return New(az, uc)
			},
			want: &namespacev1.DeleteNamespaceResponse{},
		},
		{
			name: "not empty",
			req:  &namespacev1.DeleteNamespaceRequest{Name: "full-ns"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionDelete, "full-ns").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Delete(gomock.Any(), "full-ns").
					Return(domain.NewValidationError("namespace", "contains 5 config(s)"))

				return New(az, uc)
			},
			wantErr:  "contains 5 config(s)",
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "forbidden",
			req:  &namespacev1.DeleteNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionDelete, "prod").
					Return(domain.ErrForbidden)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "forbidden",
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.DeleteNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_LockNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *namespacev1.LockNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.LockNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.LockNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Lock(gomock.Any(), "prod").
					Return(nil)

				return New(az, uc)
			},
			want: &namespacev1.LockNamespaceResponse{},
		},
		{
			name: "forbidden",
			req:  &namespacev1.LockNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(domain.ErrForbidden)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "forbidden",
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.LockNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}

func TestHandler_UnlockNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *namespacev1.UnlockNamespaceRequest
		mockFunc func(*gomock.Controller) *Handler
		wantErr  string
		wantCode connect.Code
		want     *namespacev1.UnlockNamespaceResponse
	}{
		{
			name: "success",
			req:  &namespacev1.UnlockNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(nil)
				uc := namespace_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Unlock(gomock.Any(), "prod").
					Return(nil)

				return New(az, uc)
			},
			want: &namespacev1.UnlockNamespaceResponse{},
		},
		{
			name: "forbidden",
			req:  &namespacev1.UnlockNamespaceRequest{Name: "prod"},
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				az := namespace_mock.NewMockauthz(ctrl)
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectNamespace, domain.ActionWrite, "prod").
					Return(domain.ErrForbidden)
				uc := namespace_mock.NewMockusecase(ctrl)

				return New(az, uc)
			},
			wantErr:  "forbidden",
			wantCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.UnlockNamespace(t.Context(), connect.NewRequest(tt.req))

			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg)
		})
	}
}
