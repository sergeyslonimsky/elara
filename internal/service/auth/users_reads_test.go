package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/service/auth/mocks"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

func TestUserService_GetByID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &domain.User{ID: userID, Email: "alice@example.com"}

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		want     *domain.User
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)

				return auth.NewUserService(repo)
			},
			want: user,
		},
		{
			name: "not found is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(nil, storage.ErrResourceNotFound)

				return auth.NewUserService(repo)
			},
			wantErr: "get user by id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.GetByID(t.Context(), userID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, domain.ErrNotFound)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserService_GetByIdentity(t *testing.T) {
	t.Parallel()

	user := &domain.User{ID: uuid.New(), Email: "alice@example.com"}

	tests := []struct {
		name     string
		provider string
		subject  string
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		want     *domain.User
	}{
		{
			name:     "success",
			provider: "basic",
			subject:  "alice",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByIdentity(gomock.Any(), "basic", "alice").Return(user, nil)

				return auth.NewUserService(repo)
			},
			want: user,
		},
		{
			name:     "not found is wrapped",
			provider: "basic",
			subject:  "missing",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByIdentity(gomock.Any(), "basic", "missing").Return(nil, storage.ErrResourceNotFound)

				return auth.NewUserService(repo)
			},
			wantErr: "get user by identity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.GetByIdentity(t.Context(), tt.provider, tt.subject)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, domain.ErrNotFound)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	t.Parallel()

	user := &domain.User{ID: uuid.New(), Email: "alice@example.com"}

	tests := []struct {
		name     string
		email    string
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		want     *domain.User
	}{
		{
			name:  "success normalizes email",
			email: "Alice@Example.Com",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByEmail(gomock.Any(), "alice@example.com").Return(user, nil)

				return auth.NewUserService(repo)
			},
			want: user,
		},
		{
			name:  "invalid email fails normalization",
			email: "not-an-email",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				return auth.NewUserService(auth_mock.NewMockuserRepository(ctrl))
			},
			wantErr: "normalize email",
		},
		{
			name:  "not found is wrapped",
			email: "missing@example.com",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByEmail(gomock.Any(), "missing@example.com").Return(nil, storage.ErrResourceNotFound)

				return auth.NewUserService(repo)
			},
			wantErr: "get user by email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.GetByEmail(t.Context(), tt.email)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserService_GetSystemUser(t *testing.T) {
	t.Parallel()

	user := &domain.User{ID: uuid.New(), System: true}

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		want     *domain.User
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetSystemUser(gomock.Any()).Return(user, nil)

				return auth.NewUserService(repo)
			},
			want: user,
		},
		{
			name: "not found is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetSystemUser(gomock.Any()).Return(nil, storage.ErrResourceNotFound)

				return auth.NewUserService(repo)
			},
			wantErr: "get system user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.GetSystemUser(t.Context())

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, domain.ErrNotFound)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserService_Delete(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().Delete(gomock.Any(), userID).Return(nil)

				return auth.NewUserService(repo)
			},
		},
		{
			name: "not found is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().Delete(gomock.Any(), userID).Return(storage.ErrResourceNotFound)

				return auth.NewUserService(repo)
			},
			wantErr: "delete user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.Delete(t.Context(), userID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, domain.ErrNotFound)

				return
			}
			require.NoError(t, err)
		})
	}
}

// TestUserService_Create_StorageErrors exercises the remaining mapStorageErr
// branches (already-exists and passthrough) that TestUserService_Create does
// not cover.
func TestUserService_Create_StorageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.User
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
	}{
		{
			name:  "already exists is wrapped",
			input: &domain.User{Email: "taken@example.com"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(storage.ErrResourceAlreadyExists)

				return auth.NewUserService(repo)
			},
			wantErr: "create user",
			errIs:   domain.ErrAlreadyExists,
		},
		{
			name:  "other storage errors pass through unchanged",
			input: &domain.User{Email: "test@example.com"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrIdentityTaken)

				return auth.NewUserService(repo)
			},
			errIs: domain.ErrIdentityTaken,
		},
		{
			name:  "invalid email fails normalization",
			input: &domain.User{Email: "not-an-email"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				return auth.NewUserService(auth_mock.NewMockuserRepository(ctrl))
			},
			wantErr: "normalize email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.Create(t.Context(), tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				if tt.wantErr != "" {
					require.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
