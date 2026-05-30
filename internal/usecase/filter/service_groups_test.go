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

func TestService_Groups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    filter.Query
		mockFunc func(*gomock.Controller) *filter.Service
		wantErr  string
		want     []filter.Item
	}{
		{
			name: "no group grants returns empty list without hitting repo",
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
				}, nil)

				return filter.New(perms, nil, nil, nil)
			},
			want: []filter.Item{},
		},
		{
			name:  "explicit group ids resolved against full group list, value is name",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}, Search: "x"},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectGroup,
						Action: domain.ActionRead,
						Domain: domain.GroupResource("id-a"),
					},
					{
						Object: domain.ObjectGroup,
						Action: domain.ActionWrite,
						Domain: domain.GroupResource("id-b"),
					},
				}, nil)

				groups := filtermock.NewMockgroupLister(ctrl)
				groups.EXPECT().
					List(gomock.Any(), domain.GroupFilter{Wildcard: true, Search: "x"}, domain.GroupListParams{}).
					Return([]*domain.Group{
						{ID: "id-a", Name: "alpha"},
						{ID: "id-b", Name: "beta"},
						{ID: "id-c", Name: "gamma"}, // no grant -> excluded
					}, 3, nil)

				return filter.New(perms, nil, groups, nil)
			},
			want: []filter.Item{
				{Key: "id-a", Value: "alpha", Actions: []domain.Action{domain.ActionRead}},
				{Key: "id-b", Value: "beta", Actions: []domain.Action{domain.ActionWrite}},
			},
		},
		{
			name:  "wildcard group grant covers every group",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectGroup,
						Action: domain.ActionRead,
						Domain: domain.DomainAll,
					},
				}, nil)

				groups := filtermock.NewMockgroupLister(ctrl)
				groups.EXPECT().
					List(gomock.Any(), domain.GroupFilter{Wildcard: true}, domain.GroupListParams{}).
					Return([]*domain.Group{{ID: "id-a", Name: "alpha"}, {ID: "id-b", Name: "beta"}}, 2, nil)

				return filter.New(perms, nil, groups, nil)
			},
			want: []filter.Item{
				{Key: "id-a", Value: "alpha", Actions: []domain.Action{domain.ActionRead}},
				{Key: "id-b", Value: "beta", Actions: []domain.Action{domain.ActionRead}},
			},
		},
		{
			name: "repo error is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{
						Object: domain.ObjectGroup,
						Action: domain.ActionRead,
						Domain: domain.GroupResource("id-a"),
					},
				}, nil)

				groups := filtermock.NewMockgroupLister(ctrl)
				groups.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))

				return filter.New(perms, nil, groups, nil)
			},
			wantErr: "list groups: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			svc := tt.mockFunc(ctrl)

			got, err := svc.Groups(t.Context(), domain.AuthInfo{Email: actorEmail}, tt.query)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
