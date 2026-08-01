package filter_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/filter"
	filtermock "github.com/sergeyslonimsky/elara/internal/handler/v2/filter/mocks"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	filterv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/filter/v1"
	filteruc "github.com/sergeyslonimsky/elara/internal/usecase/filter"
)

const actorEmail = "u@example.com"

func withActor(ctx context.Context) context.Context {
	return auth2.WithClaims(ctx, &auth2.Claims{Email: actorEmail})
}

func TestHandler_GetNamespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *filterv1.GetNamespacesRequest
		mockFunc func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context)
		errIs    error
		wantErr  string
		want     []*filterv1.Item
	}{
		{
			name: "maps request actions/search to query and items to proto",
			req: &filterv1.GetNamespacesRequest{
				Filters: &filterv1.Filters{Query: "pr"},
				Actions: []commonv1.PermissionAction{
					commonv1.PermissionAction_PERMISSION_ACTION_READ,
				},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Namespaces(ctx, domain.AuthInfo{Email: actorEmail}, filteruc.Query{
						Actions: []domain.Action{domain.ActionRead},
						Search:  "pr",
					}).
					Return([]filteruc.Item{
						{
							Key:     "prod",
							Value:   "prod",
							Actions: []domain.Action{domain.ActionRead, domain.ActionWrite},
						},
					}, nil)

				return filter.New(uc), ctx
			},
			want: []*filterv1.Item{
				{
					Key:   "prod",
					Value: "prod",
					Actions: []commonv1.PermissionAction{
						commonv1.PermissionAction_PERMISSION_ACTION_READ,
						commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
					},
				},
			},
		},
		{
			name: "unauthenticated context returns ErrUnauthorized without calling usecase",
			req:  &filterv1.GetNamespacesRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				return filter.New(filtermock.NewMockfilterUsecase(ctrl)), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "usecase error is propagated",
			req:  &filterv1.GetNamespacesRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Namespaces(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))

				return filter.New(uc), ctx
			},
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.GetNamespaces(ctx, connect.NewRequest(tt.req))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg.GetItems())
		})
	}
}

func TestHandler_GetGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *filterv1.GetGroupsRequest
		mockFunc func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context)
		errIs    error
		wantErr  string
		want     []*filterv1.Item
	}{
		{
			name: "maps request to query and items to proto (key=id, value=name)",
			req: &filterv1.GetGroupsRequest{
				Actions: []commonv1.PermissionAction{
					commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
				},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Groups(ctx, domain.AuthInfo{Email: actorEmail}, filteruc.Query{
						Actions: []domain.Action{domain.ActionWrite},
					}).
					Return([]filteruc.Item{
						{Key: "id-a", Value: "alpha", Actions: []domain.Action{domain.ActionWrite}},
					}, nil)

				return filter.New(uc), ctx
			},
			want: []*filterv1.Item{
				{
					Key:   "id-a",
					Value: "alpha",
					Actions: []commonv1.PermissionAction{
						commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
					},
				},
			},
		},
		{
			name: "unauthenticated context returns ErrUnauthorized without calling usecase",
			req:  &filterv1.GetGroupsRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				return filter.New(filtermock.NewMockfilterUsecase(ctrl)), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "usecase error is propagated",
			req:  &filterv1.GetGroupsRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Groups(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))

				return filter.New(uc), ctx
			},
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.GetGroups(ctx, connect.NewRequest(tt.req))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg.GetItems())
		})
	}
}

func TestHandler_GetUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *filterv1.GetUsersRequest
		mockFunc func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context)
		errIs    error
		wantErr  string
		want     []*filterv1.Item
	}{
		{
			name: "maps all action and items to proto (key=email, value=name)",
			req: &filterv1.GetUsersRequest{
				Actions: []commonv1.PermissionAction{
					commonv1.PermissionAction_PERMISSION_ACTION_ALL,
				},
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Users(ctx, domain.AuthInfo{Email: actorEmail}, filteruc.Query{
						Actions: []domain.Action{domain.ActionAll},
					}).
					Return([]filteruc.Item{
						{
							Key:     "alice@example.com",
							Value:   "Alice",
							Actions: []domain.Action{domain.ActionAll},
						},
					}, nil)

				return filter.New(uc), ctx
			},
			want: []*filterv1.Item{
				{
					Key:   "alice@example.com",
					Value: "Alice",
					Actions: []commonv1.PermissionAction{
						commonv1.PermissionAction_PERMISSION_ACTION_ALL,
					},
				},
			},
		},
		{
			name: "unauthenticated context returns ErrUnauthorized without calling usecase",
			req:  &filterv1.GetUsersRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				return filter.New(filtermock.NewMockfilterUsecase(ctrl)), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "usecase error is propagated",
			req:  &filterv1.GetUsersRequest{},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*filter.Handler, context.Context) {
				ctx = withActor(ctx)
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().
					Users(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))

				return filter.New(uc), ctx
			},
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h, ctx := tt.mockFunc(t.Context(), ctrl)

			resp, err := h.GetUsers(ctx, connect.NewRequest(tt.req))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg.GetItems())
		})
	}
}

func TestHandler_GetPermissionCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(ctrl *gomock.Controller) *filter.Handler
		want     []*filterv1.ObjectCatalogEntry
	}{
		{
			name: "maps catalog entries including every scope",
			mockFunc: func(ctrl *gomock.Controller) *filter.Handler {
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().Catalog().Return([]filteruc.CatalogEntry{
					{
						Object:  domain.ObjectNamespace,
						Scope:   filteruc.ScopeNamespace,
						Actions: []domain.Action{domain.ActionRead},
					},
					{
						Object:  domain.ObjectGroup,
						Scope:   filteruc.ScopeGroup,
						Actions: []domain.Action{domain.ActionWrite},
					},
					{
						Object:  domain.ObjectUser,
						Scope:   filteruc.ScopeGlobal,
						Actions: []domain.Action{domain.ActionAll},
					},
					{Object: domain.ObjectToken, Scope: filteruc.ScopeUnspecified, Actions: nil},
				})

				return filter.New(uc)
			},
			want: []*filterv1.ObjectCatalogEntry{
				{
					Object:  permission.ObjectToProto(domain.ObjectNamespace),
					Scope:   filterv1.ObjectScope_OBJECT_SCOPE_NAMESPACE,
					Actions: []commonv1.PermissionAction{commonv1.PermissionAction_PERMISSION_ACTION_READ},
				},
				{
					Object:  permission.ObjectToProto(domain.ObjectGroup),
					Scope:   filterv1.ObjectScope_OBJECT_SCOPE_GROUP,
					Actions: []commonv1.PermissionAction{commonv1.PermissionAction_PERMISSION_ACTION_WRITE},
				},
				{
					Object:  permission.ObjectToProto(domain.ObjectUser),
					Scope:   filterv1.ObjectScope_OBJECT_SCOPE_GLOBAL,
					Actions: []commonv1.PermissionAction{commonv1.PermissionAction_PERMISSION_ACTION_ALL},
				},
				{
					Object:  permission.ObjectToProto(domain.ObjectToken),
					Scope:   filterv1.ObjectScope_OBJECT_SCOPE_UNSPECIFIED,
					Actions: []commonv1.PermissionAction{},
				},
			},
		},
		{
			name: "empty catalog returns empty entries",
			mockFunc: func(ctrl *gomock.Controller) *filter.Handler {
				uc := filtermock.NewMockfilterUsecase(ctrl)
				uc.EXPECT().Catalog().Return(nil)

				return filter.New(uc)
			},
			want: []*filterv1.ObjectCatalogEntry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetPermissionCatalog(
				t.Context(),
				connect.NewRequest(&filterv1.GetPermissionCatalogRequest{}),
			)

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.Msg.GetEntries())
		})
	}
}
