package token

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Authenticate resolves an etcd-style (user name, password) pair to a token:
// username is the token's Username, password is the raw token itself.
//
// Every failure — unknown username, wrong secret, expired token — wraps
// domain.ErrInvalidToken so the transport can answer all of them with one
// indistinguishable error. The wrapped text says which one it was, for logs.
func (s *Service) Authenticate(ctx context.Context, username, password string) (*domain.Token, error) {
	token, err := s.store.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w (%w)", username, domain.ErrInvalidToken, err)
	}

	if subtle.ConstantTimeCompare([]byte(HashRawToken(password)), []byte(token.TokenHash)) != 1 {
		return nil, fmt.Errorf("token %s: secret mismatch: %w", token.ID, domain.ErrInvalidToken)
	}

	if token.IsExpired() {
		return nil, fmt.Errorf("token %s: expired: %w", token.ID, domain.ErrInvalidToken)
	}

	return token, nil
}
