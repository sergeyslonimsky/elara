package auth

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=bootstrap.go

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// PassthroughEmail is the synthetic user used when UI auth is disabled. It is
// seeded into the superadmin group during BootstrapPassthrough so that "no
// auth" requests still pass enforcement uniformly through the groups model —
// no direct user->role g-rule is needed.
const PassthroughEmail = "local-admin@elara.internal"

// AdminBootstrap manages the system superadmin identity. It is the single place
// that writes root permissions; it ensures the group, the (optional) user, and
// the break-glass wildcard policy rule exist and are correctly linked.
type AdminBootstrap struct {
	txm      storage.TxManager
	users    *bbolt.UserRepo
	groups   *bbolt.GroupRepo
	policies *bbolt.PolicyRepo
}

// NewAdminBootstrap returns a bootstrap helper backed by the given repositories.
func NewAdminBootstrap(
	txm storage.TxManager,
	users *bbolt.UserRepo,
	groups *bbolt.GroupRepo,
	policies *bbolt.PolicyRepo,
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
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		if err := a.ensureSuperAdminGroup(ctx, tx); err != nil {
			return err
		}

		if err := a.ensureSuperAdminPolicy(tx); err != nil {
			return err
		}

		return a.ensureBasicAdminUser(ctx, tx, username, password)
	}); err != nil {
		return fmt.Errorf("bootstrap basic: %w", err)
	}

	return nil
}

// BootstrapOIDC seeds the system for OIDC: only the superadmin group and the
// wildcard policy are created. The first OIDC login matching the configured
// admin email is elevated into the group via EnsureMember (see auth usecase).
func (a *AdminBootstrap) BootstrapOIDC(ctx context.Context) error {
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		if err := a.ensureSuperAdminGroup(ctx, tx); err != nil {
			return err
		}

		return a.ensureSuperAdminPolicy(tx)
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
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		if err := a.ensureSuperAdminGroup(ctx, tx); err != nil {
			return err
		}

		if err := a.ensureSuperAdminPolicy(tx); err != nil {
			return err
		}

		return a.ensurePassthroughUser(ctx, tx)
	}); err != nil {
		return fmt.Errorf("bootstrap passthrough: %w", err)
	}

	return nil
}

// EnsureMember ensures the given user is a member of the superadmin group.
// It is used to elevate the configured OIDC admin email after a successful
// OIDC login.
func (a *AdminBootstrap) EnsureMember(ctx context.Context, email string) error {
	if err := a.txm.Write(ctx, func(tx storage.Tx) error {
		return a.ensureMembership(ctx, tx, email)
	}); err != nil {
		return fmt.Errorf("ensure superadmin member: %w", err)
	}

	return nil
}

// ensureSuperAdminGroup creates the superadmin group with System=true if it
// does not yet exist, or upgrades the System flag if a legacy non-system group
// with the canonical name is found.
func (a *AdminBootstrap) ensureSuperAdminGroup(ctx context.Context, tx storage.Tx) error {
	groups := a.groups.WithTx(tx)

	group, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
	if err == nil {
		if !group.System {
			group.System = true
			if err := groups.Update(ctx, group); err != nil {
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
		ID:        uuid.New().String(),
		Name:      domain.SystemGroupSuperAdmin,
		Members:   []string{},
		System:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := groups.Create(ctx, group); err != nil {
		return fmt.Errorf("create superadmin group: %w", err)
	}

	return nil
}

// ensureSuperAdminPolicy writes the break-glass wildcard policy rule for the
// superadmin group if it is missing.
func (a *AdminBootstrap) ensureSuperAdminPolicy(tx storage.Tx) error {
	policy := a.policies.WithTx(tx)
	rule := []string{
		casbin.GroupSubject(domain.SystemGroupSuperAdmin),
		domain.DomainAll,
		domain.ObjectAll,
		domain.ActionAll,
	}

	if err := policy.AddPolicy("p", "p", rule); err != nil {
		return fmt.Errorf("add superadmin policy rule: %w", err)
	}

	return nil
}

// ensureBasicAdminUser creates the local basic-auth admin user (or upgrades
// its System flag) and ensures it is a member of the superadmin group.
func (a *AdminBootstrap) ensureBasicAdminUser(
	ctx context.Context,
	tx storage.Tx,
	username, password string,
) error {
	users := a.users.WithTx(tx)

	user, err := users.Get(ctx, username)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup superadmin user: %w", err)
		}

		hash, err := HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash superadmin password: %w", err)
		}

		now := time.Now().UTC()
		user = &domain.User{
			Email:                  username,
			Name:                   "Super Admin",
			Provider:               domain.ProviderBasicAuth,
			PasswordHash:           hash,
			PasswordChangeRequired: true,
			System:                 true,
			Source:                 domain.SourceLocal,
			CreatedAt:              now,
		}

		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("create superadmin user: %w", err)
		}
	} else if !user.System {
		user.System = true
		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("update superadmin user system flag: %w", err)
		}
	}

	return a.ensureMembership(ctx, tx, username)
}

// ensurePassthroughUser creates the synthetic passthrough user (or upgrades
// its System flag) and ensures it is a member of the superadmin group.
func (a *AdminBootstrap) ensurePassthroughUser(ctx context.Context, tx storage.Tx) error {
	users := a.users.WithTx(tx)

	user, err := users.Get(ctx, PassthroughEmail)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup passthrough user: %w", err)
		}

		now := time.Now().UTC()
		user = &domain.User{
			Email:     PassthroughEmail,
			Name:      "Local Admin",
			Provider:  domain.ProviderBasicAuth,
			System:    true,
			Source:    domain.SourceLocal,
			CreatedAt: now,
		}

		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("create passthrough user: %w", err)
		}
	} else if !user.System {
		user.System = true
		if err := users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("update passthrough user system flag: %w", err)
		}
	}

	return a.ensureMembership(ctx, tx, PassthroughEmail)
}

// ensureMembership adds the email to the superadmin group's Members list and
// writes the corresponding membership g-rule. Idempotent.
func (a *AdminBootstrap) ensureMembership(ctx context.Context, tx storage.Tx, email string) error {
	groups := a.groups.WithTx(tx)

	group, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
	if err != nil {
		return fmt.Errorf("lookup superadmin group for membership: %w", err)
	}

	if !slices.Contains(group.Members, email) {
		if err := group.AddMember(email); err != nil {
			return fmt.Errorf("add user to group: %w", err)
		}

		if err := groups.Update(ctx, group); err != nil {
			return fmt.Errorf("persist superadmin membership: %w", err)
		}
	}

	policy := a.policies.WithTx(tx)
	rule := []string{email, casbin.GroupSubject(domain.SystemGroupSuperAdmin), domain.MembershipDomain}

	if err := policy.AddPolicy("g", "g", rule); err != nil {
		return fmt.Errorf("add superadmin membership rule: %w", err)
	}

	return nil
}
