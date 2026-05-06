package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

//go:generate mockgen -destination=mocks/login_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth oidcProvider

const tokenBytes = 16

type oidcProvider interface {
	AuthURL(state, nonce string) string
}

// LoginUseCase initiates the OIDC login flow by generating a redirect URL.
type LoginUseCase struct {
	provider oidcProvider
}

// NewLoginUseCase returns a new LoginUseCase backed by the given OIDC provider.
func NewLoginUseCase(provider oidcProvider) *LoginUseCase {
	return &LoginUseCase{provider: provider}
}

// Execute generates random state and nonce values and returns the OIDC authorization redirect URL.
func (uc *LoginUseCase) Execute(_ context.Context) (string, string, string, error) {
	state, err := randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate state: %w", err)
	}

	nonce, err := randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate nonce: %w", err)
	}

	return uc.provider.AuthURL(state, nonce), state, nonce, nil
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
