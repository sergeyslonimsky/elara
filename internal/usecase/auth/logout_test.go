package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

func TestLogoutUseCase_Execute_ReturnsNil(t *testing.T) {
	t.Parallel()

	uc := authuc.NewLogoutUseCase()

	err := uc.Execute(t.Context())

	require.NoError(t, err)
}
