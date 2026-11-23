package token

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// EnsureUsernames gives every token created before Username existed a login
// derived from its old free-form name. It is idempotent: tokens that already
// have a Username are skipped, so running it on every start is safe. Returns
// how many tokens received a Username.
//
// Each SetUsername is its own single-step write; a crash halfway leaves the
// remaining tokens for the next start rather than a half-written token.
func (s *Service) EnsureUsernames(ctx context.Context) (int, error) {
	tokens, err := s.store.ListAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("list tokens: %w", err)
	}

	assigned := 0

	for _, t := range tokens {
		if t.Username != "" {
			continue
		}

		if err := s.assignUsername(ctx, t); err != nil {
			return assigned, err
		}

		assigned++
	}

	return assigned, nil
}

func (s *Service) assignUsername(ctx context.Context, t *domain.Token) error {
	prefix := domain.SanitizeTokenNamePrefix(t.Name)
	if err := domain.ValidateTokenNamePrefix(prefix); err != nil {
		prefix = domain.TokenUsernameFallbackPrefix
	}

	for range maxUsernameAttempts {
		username := domain.NewTokenUsername(prefix, generateUsernameSuffix())

		err := s.store.SetUsername(ctx, t.ID, username)
		if errors.Is(err, domain.ErrTokenUsernameTaken) {
			continue
		}
		if err != nil {
			return fmt.Errorf("set username for token %s: %w", t.ID, err)
		}

		return nil
	}

	return fmt.Errorf("set username for token %s: %w", t.ID, domain.ErrTokenUsernameTaken)
}
