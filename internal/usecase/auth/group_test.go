package auth_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

type fakeGroupRepo struct {
	group  *domain.Group
	groups []*domain.Group
	err    error
}

func (f *fakeGroupRepo) Create(_ context.Context, group *domain.Group) error {
	if f.err != nil {
		return f.err
	}
	f.group = group

	return nil
}

func (f *fakeGroupRepo) Get(_ context.Context, _ string) (*domain.Group, error) {
	return f.group, f.err
}

func (f *fakeGroupRepo) Update(_ context.Context, group *domain.Group) error {
	if f.err != nil {
		return f.err
	}
	f.group = group

	return nil
}

func (f *fakeGroupRepo) Delete(_ context.Context, _ string) error {
	return f.err
}

func (f *fakeGroupRepo) List(_ context.Context) ([]*domain.Group, error) {
	return f.groups, f.err
}

func TestCreateGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		repoErr error
		wantErr bool
	}{
		{
			name:  "creates group with ID and timestamps",
			input: "my-group",
		},
		{
			name:    "propagates repo error",
			input:   "fail-group",
			repoErr: errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeGroupRepo{err: tc.repoErr}
			uc := authuc.NewCreateGroupUseCase(repo)
			got, err := uc.Execute(t.Context(), tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ID == "" {
				t.Error("expected non-empty ID")
			}

			if got.CreatedAt.IsZero() {
				t.Error("expected non-zero CreatedAt")
			}

			if got.UpdatedAt.IsZero() {
				t.Error("expected non-zero UpdatedAt")
			}

			if got.Name != tc.input {
				t.Errorf("got name %q, want %q", got.Name, tc.input)
			}
		})
	}
}

func TestAddMemberUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		group      *domain.Group
		email      string
		repoErr    error
		wantErr    bool
		wantMember bool
	}{
		{
			name:       "adds member to group",
			group:      &domain.Group{ID: "g1", Name: "test"},
			email:      "user@example.com",
			wantMember: true,
		},
		{
			name:    "group not found",
			repoErr: domain.NewNotFoundError("group", "g1"),
			email:   "user@example.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeGroupRepo{group: tc.group, err: tc.repoErr}
			uc := authuc.NewAddMemberUseCase(repo)
			got, err := uc.Execute(t.Context(), "g1", tc.email)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			found := slices.Contains(got.Members, tc.email)

			if tc.wantMember && !found {
				t.Errorf("expected member %q in group", tc.email)
			}
		})
	}
}

func TestRemoveMemberUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *domain.Group
		email   string
		wantErr bool
	}{
		{
			name:  "removes existing member",
			group: &domain.Group{ID: "g1", Name: "test", Members: []string{"user@example.com"}},
			email: "user@example.com",
		},
		{
			name:    "member not found returns error",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{}},
			email:   "ghost@example.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeGroupRepo{group: tc.group}
			uc := authuc.NewRemoveMemberUseCase(repo)
			got, err := uc.Execute(t.Context(), "g1", tc.email)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, m := range got.Members {
				if m == tc.email {
					t.Errorf("member %q should have been removed", tc.email)
				}
			}
		})
	}
}

func TestUpdateGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		group    *domain.Group
		newName  string
		repoErr  error
		wantErr  bool
		wantName string
	}{
		{
			name:     "updates group name",
			group:    &domain.Group{ID: "g1", Name: "old-name"},
			newName:  "new-name",
			wantName: "new-name",
		},
		{
			name:    "group not found",
			repoErr: domain.NewNotFoundError("group", "g1"),
			newName: "new-name",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeGroupRepo{group: tc.group, err: tc.repoErr}
			uc := authuc.NewUpdateGroupUseCase(repo)
			got, err := uc.Execute(t.Context(), "g1", tc.newName)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Name != tc.wantName {
				t.Errorf("got name %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}
