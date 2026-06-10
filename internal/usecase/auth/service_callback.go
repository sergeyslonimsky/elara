package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
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
// resolves the provisioned user (with first-login email-fallback linking),
// stamps the login, and creates a new session.
//
// JIT-provisioning is not supported: unknown identities are rejected with
// ErrIdentityNotProvisioned.
//
// The entire post-exchange flow — identity resolution, optional linking,
// status check, LastLoginAt stamp, admin-bootstrap, session creation —
// runs in a single storage.Manager.WithTx (EL-51 §3.3). This makes the
// composite atomic and closes the OIDC linking race: two concurrent
// callbacks racing on the same email cannot both pass the anti-hijack
// check, because the second observes the first's appended identity inside
// the same serialized bbolt transaction.
func (s *Service) Callback(
	ctx context.Context,
	params CallbackParams,
) (*domain.User, *domain.Session, error) {
	identity, err := s.provider.Exchange(ctx, params.Code, params.Nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code: %w", err)
	}

	var (
		user *domain.User
		sess *domain.Session
	)

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
		resolved, err := s.resolveOIDCUser(ctx, identity)
		if err != nil {
			return err
		}
		if resolved.Status != domain.UserStatusActive {
			return domain.ErrUserDeactivated
		}

		stamped, err := s.users.RecordLogin(ctx, resolved.ID)
		if err != nil {
			return fmt.Errorf("record login: %w", err)
		}
		resolved = stamped

		if identity.Email == s.oidcAdminEmail {
			if err := s.admin.EnsureMember(ctx, resolved.ID.String()); err != nil {
				return fmt.Errorf("bootstrap admin: %w", err)
			}
		}

		newSess, err := s.sessions.Create(ctx, sessions.CreateParams{
			UserID:     resolved.ID.String(),
			ClientType: string(domain.ClientTypeWeb),
			IP:         params.IP,
			UserAgent:  params.UserAgent,
		})
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		user, sess = resolved, newSess

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("oidc callback tx: %w", err)
	}

	return user, sess, nil
}

// resolveOIDCUser implements the OIDC first-login flow inline (EL-50
// §3.3.1):
//
//  1. Fast path — direct lookup by (provider, sub) for already-linked users.
//  2. On miss — email-fallback: require email_verified=true, look up the
//     user by normalized email claim, and enforce the anti-hijack invariant
//     (EL-50 §6.2 inv 9) — the target must not already have an identity
//     for this provider, otherwise a second IdP user with the same email
//     would steal the account.
//  3. Otherwise — ErrIdentityNotProvisioned (no JIT).
//
// Must be called from inside Callback's WithTx so the lookup → check →
// LinkIdentity sequence is serialized; calling out of tx reopens the race
// the EL-50 review flagged as #1.
func (s *Service) resolveOIDCUser(
	ctx context.Context,
	identity *auth.Identity,
) (*domain.User, error) {
	provider := string(domain.ProviderOIDC)

	user, err := s.users.GetByIdentity(ctx, provider, identity.Subject)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("resolve oidc identity: %w", err)
	}

	return s.linkOIDCByEmail(ctx, provider, identity)
}

// linkOIDCByEmail runs the email-fallback half of the OIDC first-login
// flow: validate the email claim, look the user up, enforce anti-hijack,
// and append the new identity via UserService.LinkIdentity. Split out of
// resolveOIDCUser purely for cyclomatic — both halves run inside the
// same Callback WithTx, so atomicity is unchanged.
//
// When multi-issuer support lands (EL-50 §3.3 — Provider becomes
// "oidc:<issuerID>"), the equality check below needs to match the full
// qualified provider string. Today the system uses ProviderOIDC = "oidc"
// only, so the shortcut comparison is correct.
func (s *Service) linkOIDCByEmail(
	ctx context.Context,
	provider string,
	identity *auth.Identity,
) (*domain.User, error) {
	if !identity.EmailVerified || identity.Email == "" {
		return nil, domain.ErrIdentityNotProvisioned
	}

	normalized, err := domain.NormalizeEmail(identity.Email)
	if err != nil {
		return nil, domain.ErrIdentityNotProvisioned
	}

	candidate, err := s.users.GetByEmail(ctx, normalized)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrIdentityNotProvisioned
	}
	if err != nil {
		return nil, fmt.Errorf("resolve oidc identity by email: %w", err)
	}

	// Anti-hijack scan reads candidate.Identities from the same tx that
	// LinkIdentity will write into below, so the «no provider conflict»
	// observation cannot be invalidated by a concurrent callback — bbolt
	// serializes the write-tx that contains this whole block.
	for _, existing := range candidate.Identities {
		if string(existing.Provider) == provider {
			return nil, domain.ErrIdentityNotProvisioned
		}
	}

	linked, err := s.users.LinkIdentity(ctx, candidate.ID, domain.Identity{
		Provider: domain.IdentityProvider(provider),
		Subject:  identity.Subject,
	})
	if err != nil {
		return nil, fmt.Errorf("link oidc identity: %w", err)
	}

	return linked, nil
}
