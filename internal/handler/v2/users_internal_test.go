package v2

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestUserHandler_ListUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		users   []*domain.User
		repoErr error
		wantLen int
		wantErr bool
	}{
		{
			name:    "returns all users",
			users:   []*domain.User{{Email: "a@example.com"}, {Email: "b@example.com"}},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			users:   []*domain.User{},
			wantLen: 0,
		},
		{
			name:    "storage error returns internal",
			repoErr: errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			lister := auth_mock.NewMockuserLister(ctrl)
			lister.EXPECT().List(gomock.Any()).Return(tc.users, tc.repoErr)

			h := NewUserHandler(authuc.NewListUsersUseCase(lister), nil)

			resp, err := h.ListUsers(context.Background(), connect.NewRequest(&authv1.ListUsersRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetUsers(), tc.wantLen)
		})
	}
}

func TestUserHandler_GetUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		user     *domain.User
		repoErr  error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:  "returns user by email",
			email: "alice@example.com",
			user:  &domain.User{Email: "alice@example.com", Name: "Alice"},
		},
		{
			name:     "not found returns NotFound code",
			email:    "ghost@example.com",
			repoErr:  domain.NewNotFoundError("user", "ghost@example.com"),
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockuserGetter(ctrl)
			getter.EXPECT().Get(gomock.Any(), tc.email).Return(tc.user, tc.repoErr)

			h := NewUserHandler(nil, authuc.NewGetUserUseCase(getter))

			resp, err := h.GetUser(context.Background(), connect.NewRequest(&authv1.GetUserRequest{Email: tc.email}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.email, resp.Msg.GetUser().GetEmail())
		})
	}
}
