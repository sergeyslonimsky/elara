package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

type fakeUserLister struct {
	users []*domain.User
	err   error
}

func (f *fakeUserLister) List(_ context.Context) ([]*domain.User, error) {
	return f.users, f.err
}

type fakeUserGetter struct {
	user *domain.User
	err  error
}

func (f *fakeUserGetter) Get(_ context.Context, _ string) (*domain.User, error) {
	return f.user, f.err
}

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

			uc := authuc.NewListUsersUseCase(&fakeUserLister{users: tc.users, err: tc.repoErr})
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

func TestGetUserUseCase_Execute(t *testing.T) {
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

			uc := authuc.NewGetUserUseCase(&fakeUserGetter{user: tc.user, err: tc.repoErr})
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
