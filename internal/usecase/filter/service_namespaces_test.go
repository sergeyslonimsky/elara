package filter_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/filter"
	filtermock "github.com/sergeyslonimsky/elara/internal/usecase/filter/mocks"
)

const actorEmail = "u@example.com"

func TestService_Namespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    filter.Query
		mockFunc func(*gomock.Controller) *filter.Service
		wantErr  string
		want     []filter.Item
	}{
		{
			name: "no namespace grants returns empty list without hitting repo",
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectGroup, Action: domain.ActionRead, Domain: "group:1"},
				}, nil)

				return filter.New(perms, nil, nil, nil)
			},
			want: []filter.Item{},
		},
		{
			name:  "explicit scope maps domains to NamespaceFilter.Names",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionRead,
						Domain: domain.NamespaceResource("dev"),
					},
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionWrite,
						Domain: domain.NamespaceResource("prod"),
					},
				}, nil)

				ns := filtermock.NewMocknamespaceLister(ctrl)
				ns.EXPECT().
					List(gomock.Any(), domain.NamespaceFilter{
						Names:  map[string]struct{}{"dev": {}, "prod": {}},
						Search: "",
					}, domain.NamespaceListParams{}).
					Return([]*domain.Namespace{{Name: "dev"}, {Name: "prod"}}, 2, nil)

				return filter.New(perms, ns, nil, nil)
			},
			want: []filter.Item{
				{Key: "dev", Value: "dev", Actions: []domain.Action{domain.ActionRead}},
				{Key: "prod", Value: "prod", Actions: []domain.Action{domain.ActionWrite}},
			},
		},
		{
			name:  "wildcard scope lists every namespace and merges per-ns extras",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}, Search: "p"},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionRead,
						Domain: domain.DomainAll,
					},
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionWrite,
						Domain: domain.NamespaceResource("prod"),
					},
				}, nil)

				ns := filtermock.NewMocknamespaceLister(ctrl)
				ns.EXPECT().
					List(gomock.Any(), domain.NamespaceFilter{Wildcard: true, Search: "p"}, domain.NamespaceListParams{}).
					Return([]*domain.Namespace{{Name: "prod"}, {Name: "play"}}, 2, nil)

				return filter.New(perms, ns, nil, nil)
			},
			want: []filter.Item{
				{
					Key:     "prod",
					Value:   "prod",
					Actions: []domain.Action{domain.ActionRead, domain.ActionWrite},
				},
				{Key: "play", Value: "play", Actions: []domain.Action{domain.ActionRead}},
			},
		},
		{
			name:  "write filter excludes read-only namespaces",
			query: filter.Query{Actions: []domain.Action{domain.ActionWrite}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionRead,
						Domain: domain.NamespaceResource("dev"),
					},
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionWrite,
						Domain: domain.NamespaceResource("prod"),
					},
				}, nil)

				ns := filtermock.NewMocknamespaceLister(ctrl)
				ns.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*domain.Namespace{{Name: "dev"}, {Name: "prod"}}, 2, nil)

				return filter.New(perms, ns, nil, nil)
			},
			want: []filter.Item{
				{Key: "prod", Value: "prod", Actions: []domain.Action{domain.ActionWrite}},
			},
		},
		{
			name:  "object-all grant collapses to a single all action",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
				}, nil)

				ns := filtermock.NewMocknamespaceLister(ctrl)
				ns.EXPECT().
					List(gomock.Any(), domain.NamespaceFilter{Wildcard: true}, domain.NamespaceListParams{}).
					Return([]*domain.Namespace{{Name: "dev"}}, 1, nil)

				return filter.New(perms, ns, nil, nil)
			},
			want: []filter.Item{
				{Key: "dev", Value: "dev", Actions: []domain.Action{domain.ActionAll}},
			},
		},
		{
			name: "repo error is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectNamespace,
						Action: domain.ActionRead,
						Domain: domain.NamespaceResource("dev"),
					},
				}, nil)

				ns := filtermock.NewMocknamespaceLister(ctrl)
				ns.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))

				return filter.New(perms, ns, nil, nil)
			},
			wantErr: "list namespaces: db error",
		},
		{
			name: "permissions error is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return(nil, errors.New("casbin down"))

				return filter.New(perms, nil, nil, nil)
			},
			wantErr: "list permissions: casbin down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			svc := tt.mockFunc(ctrl)

			got, err := svc.Namespaces(t.Context(), domain.AuthInfo{Email: actorEmail}, tt.query)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
