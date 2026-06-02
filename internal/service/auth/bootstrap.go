package auth

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=bootstrap.go

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// PassthroughEmail is the synthetic user used when UI auth is disabled. It is
// seeded into the superadmin group during BootstrapPassthrough so that "no
// auth" requests still pass enforcement uniformly through the groups model —
// no direct user->role g-rule is needed.
const PassthroughEmail = "local-admin@elara.internal"

type (
	// bootstrapUserService is the narrow surface the bootstrap procedure
	// consumes from UserService. It is intentionally smaller than the full
	// UserService API — bootstrap only ever creates the seed admin or syncs
	// its identities/System flag, never the rest of the lifecycle.
	//
	// BootstrapSync is the bypass path for syncing config-driven changes on a
	// system user (e.g. SUPERADMIN_USERNAME rename). API-facing Update would
	// reject those mutations per EL-50 §6.2 inv 8.
	bootstrapUserService interface {
		GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error)
		GetSystemUser(ctx context.Context) (*domain.User, error)
		Create(ctx context.Context, user *domain.User) error
		BootstrapSync(ctx context.Context, user *domain.User) error
	}

	groupStore interface {
		Get(ctx context.Context, name string) (*domain.Group, error)
		Create(ctx context.Context, group *domain.Group) error
		Update(ctx context.Context, group *domain.Group) error
	}

	policyStore interface {
		AddPolicyCtx(ctx context.Context, sec, ptype string, rule []string) error
	}
)

// AdminBootstrap manages the system superadmin identity. It is the single place
// that writes root permissions; it ensures the group, the (optional) user, and
// the break-glass wildcard policy rule exist and are correctly linked.
type AdminBootstrap struct {
	txm      storage.Manager
	users    bootstrapUserService
	groups   groupStore
	policies policyStore
}

// NewAdminBootstrap returns a bootstrap helper backed by the given repositories.
func NewAdminBootstrap(
	txm storage.Manager,
	users bootstrapUserService,
	groups groupStore,
	policies policyStore,
) *AdminBootstrap {
	return &AdminBootstrap{
		txm:      txm,
		users:    users,
		groups:   groups,
		policies: policies,
	}
}

// BootstrapBasic seeds the system for basic-auth: group + policy + a local
// admin user with the given credentials, added to the superadmin group.
func (a *AdminBootstrap) BootstrapBasic(ctx context.Context, username, password string) error {
	if err := a.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := a.ensureSuperAdminGroup(ctx); err != nil {
			return err
		}

		if err := a.ensureSuperAdminPolicy(ctx); err != nil {
			return err
		}

		return a.ensureBasicAdminUser(ctx, username, password)
	}); err != nil {
		return fmt.Errorf("bootstrap basic: %w", err)
	}

	return nil
}

// BootstrapOIDC seeds the system for OIDC: only the superadmin group and the
// wildcard policy are created. The first OIDC login matching the configured
// admin email is elevated into the group via EnsureMember (see auth usecase).
func (a *AdminBootstrap) BootstrapOIDC(ctx context.Context) error {
	if err := a.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := a.ensureSuperAdminGroup(ctx); err != nil {
			return err
		}

		return a.ensureSuperAdminPolicy(ctx)
	}); err != nil {
		return fmt.Errorf("bootstrap oidc: %w", err)
	}

	return nil
}

// BootstrapPassthrough seeds the system for "no auth" mode: group + policy +
// a synthetic placeholder user (PassthroughEmail) added to the superadmin
// group. The passthrough auth interceptor injects this email into every
// request, so enforcement uniformly resolves to superadmin.
func (a *AdminBootstrap) BootstrapPassthrough(ctx context.Context) error {
	if err := a.txm.WithTx(ctx, func(ctx context.Context) error {
		if err := a.ensureSuperAdminGroup(ctx); err != nil {
			return err
		}

		if err := a.ensureSuperAdminPolicy(ctx); err != nil {
			return err
		}

		return a.ensurePassthroughUser(ctx)
	}); err != nil {
		return fmt.Errorf("bootstrap passthrough: %w", err)
	}

	return nil
}

// EnsureMember ensures the given user is a member of the superadmin group.
// userID must be the User.ID (UUID) of an already-provisioned user. It is
// used to elevate the configured OIDC admin after a successful login.
func (a *AdminBootstrap) EnsureMember(ctx context.Context, userID string) error {
	if err := a.txm.WithTx(ctx, func(ctx context.Context) error {
		return a.ensureMembership(ctx, userID)
	}); err != nil {
		return fmt.Errorf("ensure superadmin member: %w", err)
	}

	return nil
}

// ensureSuperAdminGroup creates the superadmin group with System=true if it
// does not yet exist, or upgrades the System flag if a legacy non-system group
// with the canonical name is found.
func (a *AdminBootstrap) ensureSuperAdminGroup(ctx context.Context) error {
	group, err := a.groups.Get(ctx, domain.SystemGroupSuperAdmin)
	if err == nil {
		if !group.System {
			group.System = true
			if err := a.groups.Update(ctx, group); err != nil {
				return fmt.Errorf("update superadmin group system flag: %w", err)
			}
		}

		return nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("lookup superadmin group: %w", err)
	}

	now := time.Now().UTC()
	group = &domain.Group{
		Name:      domain.SystemGroupSuperAdmin,
		System:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.groups.Create(ctx, group); err != nil {
		return fmt.Errorf("create superadmin group: %w", err)
	}

	return nil
}

// ensureSuperAdminPolicy writes the break-glass wildcard policy rule for the
// superadmin group if it is missing.
func (a *AdminBootstrap) ensureSuperAdminPolicy(ctx context.Context) error {
	rule := []string{
		domain.GroupResource(domain.SystemGroupSuperAdmin),
		domain.DomainAll,
		string(domain.ObjectAll),
		string(domain.ActionAll),
	}

	if err := a.policies.AddPolicyCtx(ctx, "p", "p", rule); err != nil {
		return fmt.Errorf("add superadmin policy rule: %w", err)
	}

	return nil
}

// ensureBasicAdminUser creates the local basic-auth admin user (or upgrades
// its System flag and syncs the basic identity) and ensures it is a member of
// the superadmin group.
func (a *AdminBootstrap) ensureBasicAdminUser(
	ctx context.Context,
	username, password string,
) error {
	user, err := a.users.GetSystemUser(ctx)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup superadmin user: %w", err)
		}

		created, createErr := a.createUser(ctx, username, password)
		if createErr != nil {
			return fmt.Errorf("create superadmin user: %w", createErr)
		}

		return a.ensureMembership(ctx, created.ID.String())
	}

	changed := syncBasicIdentity(user, username)
	if user.Email != username {
		// Email tracks the basic-identity Subject for the bootstrap admin —
		// both are the "login handle" surfaced to the operator. Keep them in
		// lockstep when SUPERADMIN_USERNAME changes.
		user.Email = username
		changed = true
	}

	if changed {
		if err := a.users.BootstrapSync(ctx, user); err != nil {
			return fmt.Errorf("sync superadmin user: %w", err)
		}
	}

	return a.ensureMembership(ctx, user.ID.String())
}

// syncBasicIdentity updates or adds the basic-auth identity entry in the
// user's identity list when the subject has changed or is absent.
// Returns true if the slice was modified.
func syncBasicIdentity(user *domain.User, username string) bool {
	for i, id := range user.Identities {
		if id.Provider == domain.ProviderBasic {
			if id.Subject == username {
				return false
			}

			user.Identities[i].Subject = username

			return true
		}
	}

	user.Identities = append(user.Identities, domain.Identity{
		Provider: domain.ProviderBasic,
		Subject:  username,
	})

	return true
}

// ensurePassthroughUser creates the synthetic passthrough user with a basic
// identity (per EL-50 §7.3 — "system" is not a login channel) and ensures it
// is a member of the superadmin group. The user is marked System: true; the
// basic identity is what the passthrough interceptor matches at request time.
func (a *AdminBootstrap) ensurePassthroughUser(ctx context.Context) error {
	user, err := a.users.GetByIdentity(ctx, string(domain.ProviderBasic), PassthroughEmail)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup passthrough user: %w", err)
		}

		now := time.Now().UTC()
		user = &domain.User{
			ID:          uuid.New(),
			Email:       PassthroughEmail,
			DisplayName: "Local Admin",
			Status:      domain.UserStatusActive,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: PassthroughEmail},
			},
			System:    true,
			CreatedAt: now,
		}

		if err := a.users.Create(ctx, user); err != nil {
			return fmt.Errorf("create passthrough user: %w", err)
		}
	}

	return a.ensureMembership(ctx, user.ID.String())
}

// ensureMembership writes the user→superadmin g-rule into Casbin. Casbin is
// the source of truth for membership — bbolt's group record only carries
// metadata. Idempotent: casbin.AddPolicy returns (false, nil) when the rule
// already exists. userID is User.ID (UUID), not the email.
func (a *AdminBootstrap) ensureMembership(ctx context.Context, userID string) error {
	if _, err := a.groups.Get(ctx, domain.SystemGroupSuperAdmin); err != nil {
		return fmt.Errorf("lookup superadmin group for membership: %w", err)
	}

	rule := []string{
		userID,
		domain.GroupResource(domain.SystemGroupSuperAdmin),
		domain.MembershipDomain,
	}

	if err := a.policies.AddPolicyCtx(ctx, "g", "g", rule); err != nil {
		return fmt.Errorf("add superadmin membership rule: %w", err)
	}

	return nil
}

func (a *AdminBootstrap) createUser(
	ctx context.Context,
	username, password string,
) (*domain.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash superadmin password: %w", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:          uuid.New(),
		Email:       username,
		DisplayName: "Super Admin",
		Status:      domain.UserStatusActive,
		Identities: []domain.Identity{
			{Provider: domain.ProviderBasic, Subject: username},
		},
		PasswordHash:           hash,
		PasswordChangeRequired: true,
		System:                 true,
		CreatedAt:              now,
	}

	if err := a.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create superadmin user: %w", err)
	}

	return user, nil
}
