package auth_test

import (
	"errors"
	"slices"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetterUpdater wraps separate getter and updater mocks into a single type
// that satisfies the anonymous interface{ groupGetter; groupUpdater } used by
// UpdateGroupUseCase, AddMemberUseCase, and RemoveMemberUseCase.
type groupGetterUpdater struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupUpdater
}

func TestCreateGroupUseCase_Execute(t *testing.T) { // NOSONAR
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

			ctrl := gomock.NewController(t)
			creator := auth_mock.NewMockgroupCreator(ctrl)
			creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tc.repoErr)

			uc := authuc.NewCreateGroupUseCase(creator)
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

func TestGetGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   *domain.Group
		repoErr error
		wantErr bool
	}{
		{
			name:  "returns group",
			group: &domain.Group{ID: "g1", Name: "test"},
		},
		{
			name:    "not found returns error",
			repoErr: domain.NewNotFoundError("group", "g1"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			getter.EXPECT().Get(gomock.Any(), "g1").Return(tc.group, tc.repoErr)

			uc := authuc.NewGetGroupUseCase(getter)
			got, err := uc.Execute(t.Context(), "g1")

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ID != tc.group.ID {
				t.Errorf("got ID %q, want %q", got.ID, tc.group.ID)
			}
		})
	}
}

func TestUpdateGroupUseCase_Execute(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name      string
		group     *domain.Group
		getErr    error
		updateErr error
		newName   string
		wantErr   bool
		wantName  string
	}{
		{
			name:     "updates group name",
			group:    &domain.Group{ID: "g1", Name: "old-name"},
			newName:  "new-name",
			wantName: "new-name",
		},
		{
			name:    "group not found",
			getErr:  domain.NewNotFoundError("group", "g1"),
			newName: "new-name",
			wantErr: true,
		},
		{
			name:      "update fails",
			group:     &domain.Group{ID: "g1", Name: "old-name"},
			updateErr: errors.New("storage error"),
			newName:   "new-name",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetterUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), "g1").Return(tc.group, tc.getErr)
			if tc.getErr == nil {
				updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			}

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

func TestAddMemberUseCase_Execute(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name       string
		group      *domain.Group
		email      string
		getErr     error
		updateErr  error
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
			getErr:  domain.NewNotFoundError("group", "g1"),
			email:   "user@example.com",
			wantErr: true,
		},
		{
			name:    "duplicate member returns error",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{"user@example.com"}},
			email:   "user@example.com",
			wantErr: true,
		},
		{
			name:      "update fails",
			group:     &domain.Group{ID: "g1", Name: "test"},
			email:     "user@example.com",
			updateErr: errors.New("storage error"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetterUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), "g1").Return(tc.group, tc.getErr)
			if tc.getErr == nil && tc.group != nil && !slices.Contains(tc.group.Members, tc.email) {
				updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			}

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

func TestRemoveMemberUseCase_Execute(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name      string
		group     *domain.Group
		email     string
		getErr    error
		updateErr error
		wantErr   bool
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
		{
			name:    "group not found",
			getErr:  domain.NewNotFoundError("group", "g1"),
			email:   "user@example.com",
			wantErr: true,
		},
		{
			name:      "update fails",
			group:     &domain.Group{ID: "g1", Name: "test", Members: []string{"user@example.com"}},
			email:     "user@example.com",
			updateErr: errors.New("storage error"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetterUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), "g1").Return(tc.group, tc.getErr)
			if tc.getErr == nil && tc.group != nil && slices.Contains(tc.group.Members, tc.email) {
				updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			}

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
