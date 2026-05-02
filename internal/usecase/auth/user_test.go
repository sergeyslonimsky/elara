package auth_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestListUsersUseCase_Execute(t *testing.T) {
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
			name:    "returns empty slice",
			users:   []*domain.User{},
			wantLen: 0,
		},
		{
			name:    "propagates repo error",
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

			uc := authuc.NewListUsersUseCase(lister)
			got, err := uc.Execute(t.Context())

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tc.wantLen {
				t.Errorf("got %d users, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetUserUseCase_Execute(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name    string
		user    *domain.User
		repoErr error
		email   string
		wantErr bool
	}{
		{
			name:  "returns user",
			user:  &domain.User{Email: "user@example.com"},
			email: "user@example.com",
		},
		{
			name:    "not found propagated",
			repoErr: domain.NewNotFoundError("user", "ghost@example.com"),
			email:   "ghost@example.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockuserGetter(ctrl)
			getter.EXPECT().Get(gomock.Any(), tc.email).Return(tc.user, tc.repoErr)

			uc := authuc.NewGetUserUseCase(getter)
			got, err := uc.Execute(t.Context(), tc.email)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tc.repoErr != nil && !errors.Is(err, domain.ErrNotFound) {
					t.Errorf("expected ErrNotFound, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Email != tc.user.Email {
				t.Errorf("got email %q, want %q", got.Email, tc.user.Email)
			}
		})
	}
}
