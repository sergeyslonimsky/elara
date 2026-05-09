package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

func TestService_Login(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authuc.Service
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().
					AuthURL(gomock.Any(), gomock.Any()).
					DoAndReturn(func(state, nonce string) string {
						return "http://idp/auth?state=" + state + "&nonce=" + nonce
					})

				return svc
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			redirectURL, state, nonce, err := sut.Login(t.Context())

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, redirectURL)
			assert.NotEmpty(t, state)
			assert.NotEmpty(t, nonce)
			assert.Contains(t, redirectURL, state)
			assert.Contains(t, redirectURL, nonce)
		})
	}
}

func TestService_Login_UniqueTokens(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	sut, m := setupService(t, ctrl)
	m.provider.EXPECT().AuthURL(gomock.Any(), gomock.Any()).Return("http://idp/auth").AnyTimes()

	_, state1, nonce1, err := sut.Login(t.Context())
	require.NoError(t, err)

	_, state2, nonce2, err := sut.Login(t.Context())
	require.NoError(t, err)

	assert.NotEqual(t, state1, state2, "state must be unique per call")
	assert.NotEqual(t, nonce1, nonce2, "nonce must be unique per call")
}
