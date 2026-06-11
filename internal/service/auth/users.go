package auth

//go:generate mockgen -destination=mocks/users_mock.go -package=auth_mock -source=users.go

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// mapStorageErr converts storage-layer sentinels to the domain sentinels
// service-layer consumers already errors.Is-check. Repo functions returning
// other domain errors (ErrEmailTaken / ErrIdentityTaken / ErrSystemImmutable)
// are passed through unchanged.
func mapStorageErr(err error, resource, identifier string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, storage.ErrResourceNotFound):
		return fmt.Errorf("map storage error : %w", domain.NewNotFoundError(resource, identifier))
	case errors.Is(err, storage.ErrResourceAlreadyExists):
		return fmt.Errorf("map storage error : %w", domain.NewAlreadyExistsError(resource, identifier))
	default:
		return err
	}
}

// userRepository is the storage surface UserService consumes. It mirrors a
// subset of bbolt.UserRepo — kept narrow so service-level tests can fake the
// repo without dragging in the rest of the storage layer.
type userRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetSystemUser(ctx context.Context) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserService is the user-record primitive layer: it knows HOW to mutate a
// user record correctly (normalize input, enforce per-field immutability,
// keep secondary indexes consistent). It does NOT decide WHY a mutation
// happens — that is the usecase layer's responsibility, including
// orchestration across multiple services, audit emission, and transaction
// boundaries (EL-51 §3.3 — services don't open tx, usecases do via
// storage.Manager.WithTx).
//
// Mutable fields after Create (per EL-50 §6.2, post scope-cut):
//   - Identities: append-only, via LinkIdentity.
//   - Status: via Deactivate/Reactivate transitions.
//   - LastLoginAt: via RecordLogin.
//   - PasswordHash/PasswordChangeRequired: via ResetPassword (lives in
//     password.go).
//
// Everything else (Email, DisplayName/Picture, Identity.Subject) is
// read-only through the API. BootstrapSync is the explicit cmd-bootstrap
// escape hatch for syncing the system user with config-driven parameters
// and never runs from handler/usecase paths.
type UserService struct {
	repo userRepository
}

// NewUserService constructs a UserService over the given user repository.
func NewUserService(repo userRepository) *UserService {
	return &UserService{repo: repo}
}

// GetByID is a thin read delegate. Lifecycle/orchestration callers go
// through the service even for reads so the dependency surface stays single
// (one *UserService injection per consumer rather than service+repo pair).
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", mapStorageErr(err, "user", id.String()))
	}

	return user, nil
}

// GetByIdentity is a thin read delegate used by the auth usecase and the
// bootstrap procedure to find users by an external identity tuple.
func (s *UserService) GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error) {
	user, err := s.repo.GetByIdentity(ctx, provider, subject)
	if err != nil {
		return nil, fmt.Errorf(
			"get user by identity: %w",
			mapStorageErr(err, "user identity", provider+":"+subject),
		)
	}

	return user, nil
}

// GetByEmail looks up a user by their normalized email. Input is normalized
// via domain.NormalizeEmail before the lookup, so callers can pass raw
// user-supplied input without pre-normalizing. Returns ErrNotFound (wrapped
// in the not-found sentinel) when no user matches.
func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return nil, fmt.Errorf("normalize email: %w", err)
	}

	user, err := s.repo.GetByEmail(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", mapStorageErr(err, "user", normalized))
	}

	return user, nil
}

// GetSystemUser is the bootstrap-side lookup for the unique system user.
// Returns ErrNotFound when none exists — bootstrap will then create one.
func (s *UserService) GetSystemUser(ctx context.Context) (*domain.User, error) {
	user, err := s.repo.GetSystemUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get system user: %w", mapStorageErr(err, "system user", ""))
	}

	return user, nil
}

// Create persists a new user, applying default ID and Status when the caller
// did not. Identity-uniqueness is enforced by the repo's secondary index
// (returns domain.ErrIdentityTaken); duplicate user.ID returns
// domain.ErrAlreadyExists.
//
// Domain validation (Email/Name shape) is the caller's responsibility — the
// service trusts that the constructed user already passed user.Validate()
// at the usecase/handler boundary. Bootstrap is the exception: it builds
// users from trusted config and skips validation by contract.
func (s *UserService) Create(ctx context.Context, user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.Status == "" {
		user.Status = domain.UserStatusActive
	}
	if user.Email != "" {
		normalized, err := domain.NormalizeEmail(user.Email)
		if err != nil {
			return fmt.Errorf("normalize email: %w", err)
		}
		user.Email = normalized
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", mapStorageErr(err, "user", user.ID.String()))
	}

	return nil
}

// LinkIdentity appends a new external identity to an existing user. The
// operation is append-only by contract (EL-50 §6.2 inv 3): existing
// identities cannot be substituted or removed through this path.
//
// System users are normally locked, but the BootstrapOIDC placeholder
// (System=true, Identities=nil) MUST accept its first append — otherwise
// the first OIDC callback can never adopt the placeholder and login is
// permanently rejected with ErrIdentityNotProvisioned. Once that first
// identity lands, the System lock re-engages and further mutations reject
// with ErrSystemImmutable.
//
// Returns the freshly loaded user with the new identity appended. If the
// identity is already present on the user, the call is a no-op (idempotent)
// and the user is returned unchanged.
//
// Anti-hijack and provider-uniqueness invariants are the caller's
// responsibility — LinkIdentity only enforces the per-record append rule.
// The OIDC first-login flow (usecase/auth.Callback) wraps LinkIdentity in
// the same WithTx as the resolution lookups so the composite is atomic.
func (s *UserService) LinkIdentity(
	ctx context.Context,
	userID uuid.UUID,
	identity domain.Identity,
) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", mapStorageErr(err, "user", userID.String()))
	}
	// The BootstrapOIDC placeholder (System=true, Identities=nil) MUST
	// accept its first append; once any identity is linked, the System
	// lock re-engages and every further mutation rejects.
	if user.System && len(user.Identities) > 0 {
		return nil, fmt.Errorf("system user identities are immutable: %w", domain.ErrSystemImmutable)
	}

	for _, existing := range user.Identities {
		if existing.Provider == identity.Provider && existing.Subject == identity.Subject {
			return user, nil
		}
	}

	user.Identities = append(user.Identities, identity)
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("link identity: %w", err)
	}

	return user, nil
}

// RecordLogin stamps a successful authentication on the user record. The
// service re-loads the user inside the call (consistent with LinkIdentity
// and transitionStatus) so a stale `*domain.User` held by the caller
// cannot silently clobber concurrently-changed fields (Status,
// MembershipVersion, DisplayName, …). Returns the freshly-stamped user so
// callers can return it from their flow without an extra read.
func (s *UserService) RecordLogin(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", mapStorageErr(err, "user", userID.String()))
	}

	user.LastLoginAt = time.Now()
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("record login: %w", err)
	}

	return user, nil
}

// BootstrapSync is the bootstrap-only escape hatch for syncing a system
// user with config-driven parameters (SUPERADMIN_USERNAME changed, etc.).
// It bypasses the API-side immutability of Email/Identities but still
// requires the persisted user to have System == true — bootstrap cannot be
// misused to mutate ordinary users.
//
// Callers: AdminBootstrap only. Never invoked from handler/usecase paths.
func (s *UserService) BootstrapSync(ctx context.Context, user *domain.User) error {
	prev, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("load existing user: %w", mapStorageErr(err, "user", user.ID.String()))
	}
	if !prev.System {
		return fmt.Errorf("bootstrap sync forbidden on non-system user: %w", domain.ErrSystemImmutable)
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return fmt.Errorf("bootstrap sync: %w", err)
	}

	return nil
}

// Delete removes a user from storage along with all their identity index
// entries. Caller is responsible for revoking sessions, deleting Casbin
// policy rules, and enforcing the last-admin guard — Delete is a primitive,
// not the orchestrated lifecycle event.
func (s *UserService) Delete(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", mapStorageErr(err, "user", userID.String()))
	}

	return nil
}

// Deactivate transitions a user to Status=Deactivated. Returns the updated
// user. System users are rejected by domain.User.Deactivate() via the
// EnsureMutable guard.
func (s *UserService) Deactivate(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.transitionStatus(ctx, userID, (*domain.User).Deactivate)
}

// Reactivate transitions a user to Status=Active. Mirrors Deactivate.
func (s *UserService) Reactivate(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.transitionStatus(ctx, userID, (*domain.User).Reactivate)
}

func (s *UserService) transitionStatus(
	ctx context.Context,
	userID uuid.UUID,
	apply func(*domain.User) error,
) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", mapStorageErr(err, "user", userID.String()))
	}
	if err := apply(user); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("persist status transition: %w", err)
	}

	return user, nil
}
