package auth

import (
	"context"
	"fmt"
)

// Login generates random state and nonce values and returns the OIDC authorization redirect URL.
func (s *Service) Login(_ context.Context) (string, string, string, error) {
	state, err := randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate state: %w", err)
	}

	nonce, err := randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate nonce: %w", err)
	}

	return s.provider.AuthURL(state, nonce), state, nonce, nil
}
