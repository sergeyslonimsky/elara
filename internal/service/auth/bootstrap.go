package auth

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=bootstrap.go

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// PassthroughEmail is the synthetic user used when UI auth is disabled. It is
// seeded into the superadmin group during BootstrapPassthrough so that "no
// auth" requests still pass enforcement uniformly through the groups model —
// no direct user->role g-rule is needed.
const PassthroughEmail = "local-admin@elara.internal"

type (
	userStore interface {
		Get(ctx context.Context, email string) (*domain.User, error)
		Upsert(ctx context.Context, user *domain.User) error
	}

	groupStore interface {
		FindByName(ctx context.Context, name string) (*domain.Group, error)
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
	users    userStore
	groups   groupStore
	policies policyStore
}

// NewAdminBootstrap returns a bootstrap helper backed by the given repositories.
func NewAdminBootstrap(
	txm storage.Manager,
	users userStore,
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
// It is used to elevate the configured OIDC admin email after a successful
// OIDC login.
func (a *AdminBootstrap) EnsureMember(ctx context.Context, email string) error {
	if err := a.txm.WithTx(ctx, func(ctx context.Context) error {
		return a.ensureMembership(ctx, email)
	}); err != nil {
		return fmt.Errorf("ensure superadmin member: %w", err)
	}

	return nil
}

// ensureSuperAdminGroup creates the superadmin group with System=true if it
// does not yet exist, or upgrades the System flag if a legacy non-system group
// with the canonical name is found.
func (a *AdminBootstrap) ensureSuperAdminGroup(ctx context.Context) error {
	group, err := a.groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
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
		ID:        uuid.New().String(),
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
		casbin.GroupSubject(domain.SystemGroupSuperAdmin),
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
// its System flag) and ensures it is a member of the superadmin group.
func (a *AdminBootstrap) ensureBasicAdminUser(
	ctx context.Context,
	username, password string,
) error {
	user, err := a.users.Get(ctx, username)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup superadmin user: %w", err)
		}

		if err := a.createUser(ctx, a.users, username, password); err != nil {
			return fmt.Errorf("upsert superadmin user: %w", err)
		}
	} else if !user.System {
		user.System = true
		if err := a.users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("update superadmin user system flag: %w", err)
		}
	}

	return a.ensureMembership(ctx, username)
}

// ensurePassthroughUser creates the synthetic passthrough user (or upgrades
// its System flag) and ensures it is a member of the superadmin group.
func (a *AdminBootstrap) ensurePassthroughUser(ctx context.Context) error {
	user, err := a.users.Get(ctx, PassthroughEmail)
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

		if err := a.users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("create passthrough user: %w", err)
		}
	} else if !user.System {
		user.System = true
		if err := a.users.Upsert(ctx, user); err != nil {
			return fmt.Errorf("update passthrough user system flag: %w", err)
		}
	}

	return a.ensureMembership(ctx, PassthroughEmail)
}

// ensureMembership writes the user→superadmin g-rule into Casbin. Casbin is
// the source of truth for membership — bbolt's group record only carries
// metadata. Idempotent: casbin.AddPolicy returns (false, nil) when the rule
// already exists.
func (a *AdminBootstrap) ensureMembership(ctx context.Context, email string) error {
	if _, err := a.groups.FindByName(ctx, domain.SystemGroupSuperAdmin); err != nil {
		return fmt.Errorf("lookup superadmin group for membership: %w", err)
	}

	rule := []string{
		email,
		casbin.GroupSubject(domain.SystemGroupSuperAdmin),
		domain.MembershipDomain,
	}

	if err := a.policies.AddPolicyCtx(ctx, "g", "g", rule); err != nil {
		return fmt.Errorf("add superadmin membership rule: %w", err)
	}

	return nil
}

func (a *AdminBootstrap) createUser(ctx context.Context, repo userStore, username, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash superadmin password: %w", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		Email:                  username,
		Name:                   "Super Admin",
		Provider:               domain.ProviderBasicAuth,
		PasswordHash:           hash,
		PasswordChangeRequired: true,
		System:                 true,
		Source:                 domain.SourceLocal,
		CreatedAt:              now,
	}

	if err := repo.Upsert(ctx, user); err != nil {
		return fmt.Errorf("create superadmin user: %w", err)
	}

	return nil
}
