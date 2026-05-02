package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	authmock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestMeUseCase_Execute_WithClaims(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockEnforcer := authmock.NewMockroleGetter(ctrl)
	mockEnforcer.EXPECT().
		GetRolesForUser("user@example.com", "*").
		Return([]string{"role:viewer"}, nil)

	uc := authuc.NewMeUseCase(mockEnforcer)

	claims := &auth.Claims{Email: "user@example.com", Name: "Test User"}
	ctx := auth.WithClaims(t.Context(), claims)

	user, roles, err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, []string{"role:viewer"}, roles)
}

func TestMeUseCase_Execute_NoClaims(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockEnforcer := authmock.NewMockroleGetter(ctrl)

	uc := authuc.NewMeUseCase(mockEnforcer)

	_, _, err := uc.Execute(t.Context())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}
