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

func TestService_Users(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    filter.Query
		mockFunc func(*gomock.Controller) *filter.Service
		wantErr  string
		want     []filter.Item
	}{
		{
			name:  "no global user grant returns empty list without hitting repo",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectGroup, Action: domain.ActionRead, Domain: domain.GroupResource("id-a")},
				}, nil)

				return filter.New(perms, nil, nil, nil)
			},
			want: []filter.Item{},
		},
		{
			name:  "global user read returns every user with the global action set",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}, Search: "a"},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: domain.DomainAll},
					{Object: domain.ObjectUser, Action: domain.ActionWrite, Domain: domain.DomainAll},
				}, nil)

				users := filtermock.NewMockuserLister(ctrl)
				users.EXPECT().
					List(gomock.Any(), domain.UserFilter{AnyUser: true, Search: "a"}, domain.UserListParams{}).
					Return([]*domain.User{
						{Email: "alice@example.com", Name: "Alice"},
						{Email: "bob@example.com"}, // no name -> value falls back to email
					}, 2, nil)

				return filter.New(perms, nil, nil, users)
			},
			want: []filter.Item{
				{
					Key:     "alice@example.com",
					Value:   "Alice",
					Actions: []domain.Action{domain.ActionRead, domain.ActionWrite},
				},
				{
					Key:     "bob@example.com",
					Value:   "bob@example.com",
					Actions: []domain.Action{domain.ActionRead, domain.ActionWrite},
				},
			},
		},
		{
			name:  "write filter excludes caller holding only user read",
			query: filter.Query{Actions: []domain.Action{domain.ActionWrite}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: domain.DomainAll},
				}, nil)

				return filter.New(perms, nil, nil, nil)
			},
			want: []filter.Item{},
		},
		{
			name:  "object-all grant yields all users with all action",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
				}, nil)

				users := filtermock.NewMockuserLister(ctrl)
				users.EXPECT().
					List(gomock.Any(), domain.UserFilter{AnyUser: true}, domain.UserListParams{}).
					Return([]*domain.User{{Email: "alice@example.com", Name: "Alice"}}, 1, nil)

				return filter.New(perms, nil, nil, users)
			},
			want: []filter.Item{
				{Key: "alice@example.com", Value: "Alice", Actions: []domain.Action{domain.ActionAll}},
			},
		},
		{
			name:  "repo error is wrapped",
			query: filter.Query{Actions: []domain.Action{domain.ActionRead}},
			mockFunc: func(ctrl *gomock.Controller) *filter.Service {
				perms := filtermock.NewMockpermissions(ctrl)
				perms.EXPECT().ListPermissions(actorEmail).Return([]domain.Permission{
					{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: domain.DomainAll},
				}, nil)

				users := filtermock.NewMockuserLister(ctrl)
				users.EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, 0, errors.New("db error"))

				return filter.New(perms, nil, nil, users)
			},
			wantErr: "list users: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			svc := tt.mockFunc(ctrl)

			got, err := svc.Users(t.Context(), domain.AuthInfo{Email: actorEmail}, tt.query)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
