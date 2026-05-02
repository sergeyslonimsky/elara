package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	authmock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestLoginUseCase_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockProvider := authmock.NewMockoidcProvider(ctrl)
	mockProvider.EXPECT().
		AuthURL(gomock.Any(), gomock.Any()).
		DoAndReturn(func(state, nonce string) string {
			return "http://idp/auth?state=" + state + "&nonce=" + nonce
		})

	uc := authuc.NewLoginUseCase(mockProvider)

	redirectURL, state, nonce, err := uc.Execute(t.Context())

	require.NoError(t, err)
	assert.NotEmpty(t, redirectURL)
	assert.NotEmpty(t, state)
	assert.NotEmpty(t, nonce)
	assert.Contains(t, redirectURL, state)
	assert.Contains(t, redirectURL, nonce)
}

func TestLoginUseCase_Execute_UniqueTokens(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockProvider := authmock.NewMockoidcProvider(ctrl)
	mockProvider.EXPECT().AuthURL(gomock.Any(), gomock.Any()).Return("http://idp/auth").AnyTimes()

	uc := authuc.NewLoginUseCase(mockProvider)

	_, state1, nonce1, err := uc.Execute(t.Context())
	require.NoError(t, err)

	_, state2, nonce2, err := uc.Execute(t.Context())
	require.NoError(t, err)

	assert.NotEqual(t, state1, state2, "state must be unique per call")
	assert.NotEqual(t, nonce1, nonce2, "nonce must be unique per call")
}
