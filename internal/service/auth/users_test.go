package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/service/auth/mocks"
)

func TestUserService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.User
		mockFunc func(*gomock.Controller) *auth.UserService
		want     func(*testing.T, *domain.User)
	}{
		{
			name:  "defaults status to active",
			input: &domain.User{Email: "test@example.com"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.Equal(t, domain.UserStatusActive, u.Status)
						assert.NotEqual(t, uuid.Nil, u.ID)

						return nil
					})

				return auth.NewUserService(repo)
			},
		},
		{
			name:  "mints ID when nil",
			input: &domain.User{Email: "test@example.com"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.NotEqual(t, uuid.Nil, u.ID)

						return nil
					})

				return auth.NewUserService(repo)
			},
		},
		{
			name:  "normalizes email",
			input: &domain.User{Email: "ALICE@Example.Com"},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.Equal(t, "alice@example.com", u.Email)

						return nil
					})

				return auth.NewUserService(repo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.Create(t.Context(), tt.input)
			require.NoError(t, err)
		})
	}
}

func TestUserService_LinkIdentity(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	existingIdentity := domain.Identity{Provider: "basic", Subject: "alice"}
	newIdentity := domain.Identity{Provider: "oidc", Subject: "alice-sub"}

	tests := []struct {
		name     string
		userID   uuid.UUID
		identity domain.Identity
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
		want     *domain.User
	}{
		{
			name:     "appends new identity",
			userID:   userID,
			identity: newIdentity,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{
					ID:         userID,
					Identities: []domain.Identity{existingIdentity},
				}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)

				expectedUser := &domain.User{
					ID:         userID,
					Identities: []domain.Identity{existingIdentity, newIdentity},
				}
				repo.EXPECT().Update(gomock.Any(), expectedUser).Return(nil)

				return auth.NewUserService(repo)
			},
			want: &domain.User{
				ID:         userID,
				Identities: []domain.Identity{existingIdentity, newIdentity},
			},
		},
		{
			name:     "idempotent when identity exists",
			userID:   userID,
			identity: existingIdentity,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{
					ID:         userID,
					Identities: []domain.Identity{existingIdentity},
				}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)

				return auth.NewUserService(repo)
			},
			want: &domain.User{
				ID:         userID,
				Identities: []domain.Identity{existingIdentity},
			},
		},
		{
			name:     "rejects system user",
			userID:   userID,
			identity: newIdentity,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{
					ID:     userID,
					System: true,
				}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)

				return auth.NewUserService(repo)
			},
			errIs: domain.ErrSystemImmutable,
		},
		{
			name:     "propagates GetByID error",
			userID:   userID,
			identity: newIdentity,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(nil, assert.AnError)

				return auth.NewUserService(repo)
			},
			wantErr: assert.AnError.Error(),
		},
		{
			name:     "propagates Update error",
			userID:   userID,
			identity: newIdentity,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{
					ID: userID,
				}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return auth.NewUserService(repo)
			},
			wantErr: "link identity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.LinkIdentity(t.Context(), tt.userID, tt.identity)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserService_RecordLogin(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name     string
		userID   uuid.UUID
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
		want     func(*testing.T, *domain.User)
	}{
		{
			name:   "stamps last login at",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{ID: userID}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)
				repo.EXPECT().
					Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.WithinDuration(t, time.Now(), u.LastLoginAt, time.Second)

						return nil
					})

				return auth.NewUserService(repo)
			},
			want: func(t *testing.T, u *domain.User) {
				t.Helper()
				assert.WithinDuration(t, time.Now(), u.LastLoginAt, time.Second)
			},
		},
		{
			name:   "propagates GetByID error",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(nil, assert.AnError)

				return auth.NewUserService(repo)
			},
			wantErr: assert.AnError.Error(),
		},
		{
			name:   "propagates Update error",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				user := &domain.User{ID: userID}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(user, nil)
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return auth.NewUserService(repo)
			},
			wantErr: "record login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.RecordLogin(t.Context(), tt.userID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			if tt.want != nil {
				tt.want(t, got)
			}
		})
	}
}

func TestUserService_BootstrapSync(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name     string
		input    *domain.User
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
	}{
		{
			name: "allows full reshape on system user",
			input: &domain.User{
				ID:     userID,
				Email:  "admin-v2@example.com",
				System: true,
			},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				prev := &domain.User{
					ID:     userID,
					Email:  "admin-v1@example.com",
					System: true,
				}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(prev, nil)
				repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

				return auth.NewUserService(repo)
			},
		},
		{
			name: "rejects non-system user",
			input: &domain.User{
				ID:     userID,
				System: false,
			},
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				prev := &domain.User{ID: userID, System: false}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(prev, nil)

				return auth.NewUserService(repo)
			},
			errIs: domain.ErrSystemImmutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.BootstrapSync(t.Context(), tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

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

func TestUserService_Deactivate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name     string
		userID   uuid.UUID
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
		want     domain.UserStatus
	}{
		{
			name:   "loads applies persists",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				loaded := &domain.User{ID: userID, Status: domain.UserStatusActive}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(loaded, nil)
				repo.EXPECT().
					Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.Equal(t, domain.UserStatusDeactivated, u.Status)

						return nil
					})

				return auth.NewUserService(repo)
			},
			want: domain.UserStatusDeactivated,
		},
		{
			name:   "rejects system user",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				loaded := &domain.User{ID: userID, Status: domain.UserStatusActive, System: true}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(loaded, nil)

				return auth.NewUserService(repo)
			},
			errIs: domain.ErrSystemImmutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.Deactivate(t.Context(), tt.userID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
		})
	}
}

func TestUserService_Reactivate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name     string
		userID   uuid.UUID
		mockFunc func(*gomock.Controller) *auth.UserService
		wantErr  string
		errIs    error
		want     domain.UserStatus
	}{
		{
			name:   "transitions back",
			userID: userID,
			mockFunc: func(ctrl *gomock.Controller) *auth.UserService {
				repo := auth_mock.NewMockuserRepository(ctrl)
				loaded := &domain.User{ID: userID, Status: domain.UserStatusDeactivated}
				repo.EXPECT().GetByID(gomock.Any(), userID).Return(loaded, nil)
				repo.EXPECT().
					Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, u *domain.User) error {
						assert.Equal(t, domain.UserStatusActive, u.Status)

						return nil
					})

				return auth.NewUserService(repo)
			},
			want: domain.UserStatusActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.Reactivate(t.Context(), tt.userID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Status)
		})
	}
}
