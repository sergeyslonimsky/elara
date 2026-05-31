package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

// CallbackParams carries the parameters for OIDC Callback.
type CallbackParams struct {
	Code      string
	Nonce     string
	IP        string
	UserAgent string
}

// Callback handles the OIDC callback: exchanges the code for an identity,
// upserts the user, bootstraps the configured admin, and creates a new session.
//
// All operations are performed atomically within a single transaction.
func (s *Service) Callback(
	ctx context.Context,
	params CallbackParams,
) (*domain.User, *domain.Session, error) {
	identity, err := s.provider.Exchange(ctx, params.Code, params.Nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code: %w", err)
	}

	user := &domain.User{
		Email:       identity.Email,
		Name:        identity.Name,
		Picture:     identity.Picture,
		Provider:    domain.ProviderOIDC,
		LastLoginAt: time.Now(),
	}

	var sess *domain.Session

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
		if err = s.users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("upsert user: %w", err)
		}

		if identity.Email == s.oidcAdminEmail {
			if err = s.admin.EnsureMember(ctx, identity.Email); err != nil {
				return fmt.Errorf("bootstrap admin: %w", err)
			}
		}

		newSess, err := s.sessions.Create(ctx, sessions.CreateParams{
			UserID:     user.Email,
			ClientType: string(domain.ClientTypeWeb),
			IP:         params.IP,
			UserAgent:  params.UserAgent,
		})
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		sess = newSess

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("oidc callback tx: %w", err)
	}

	return user, sess, nil
}
