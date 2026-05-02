package auth_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	authmock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestCallbackUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	mockProvider := authmock.NewMockcallbackProvider(ctrl)
	mockUsers := authmock.NewMockuserUpserter(ctrl)
	mockLoader := authmock.NewMockpolicyLoader(ctrl)

	identity := &auth.Identity{
		Email:   "user@example.com",
		Name:    "Test User",
		Picture: "https://example.com/pic.png",
	}
	mockProvider.EXPECT().Exchange(gomock.Any(), "auth-code", gomock.Any()).Return(identity, nil)
	mockUsers.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
	mockLoader.EXPECT().Load(gomock.Any()).Return([][]string{}, nil).AnyTimes()
	mockLoader.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	session := auth.NewSessionManager("test-secret", 0)
	uc := authuc.NewCallbackUseCase(mockProvider, mockUsers, session, nil, mockLoader, []string{})

	token, user, err := uc.Execute(t.Context(), "auth-code", "test-nonce")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, "user@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
}

func TestCallbackUseCase_Execute_ExchangeError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	mockProvider := authmock.NewMockcallbackProvider(ctrl)
	mockUsers := authmock.NewMockuserUpserter(ctrl)
	mockLoader := authmock.NewMockpolicyLoader(ctrl)

	mockProvider.EXPECT().Exchange(gomock.Any(), "bad-code", gomock.Any()).Return(nil, errors.New("exchange failed"))

	session := auth.NewSessionManager("test-secret", 0)
	uc := authuc.NewCallbackUseCase(mockProvider, mockUsers, session, nil, mockLoader, []string{})

	_, _, err := uc.Execute(t.Context(), "bad-code", "test-nonce")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchange code")
}

func TestCallbackUseCase_Execute_UpsertError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	mockProvider := authmock.NewMockcallbackProvider(ctrl)
	mockUsers := authmock.NewMockuserUpserter(ctrl)
	mockLoader := authmock.NewMockpolicyLoader(ctrl)

	identity := &auth.Identity{Email: "user@example.com", Name: "User"}
	mockProvider.EXPECT().Exchange(gomock.Any(), "auth-code", gomock.Any()).Return(identity, nil)
	mockUsers.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	session := auth.NewSessionManager("test-secret", 0)
	uc := authuc.NewCallbackUseCase(mockProvider, mockUsers, session, nil, mockLoader, []string{})

	_, _, err := uc.Execute(t.Context(), "auth-code", "test-nonce")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert user")
}
