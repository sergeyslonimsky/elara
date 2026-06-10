// Package user is the bbolt-backed repository for domain.User.
//
// Repository contract:
//   - Pure CRUD (Create, Get*, Update, Delete) is "dumb" — callers prepare
//     all fields, the repository writes verbatim. The only server-side
//     mutation is CreatedAt defaulting to time.Now() on Create when unset.
//   - Multi-bucket writes (Create / Update / Delete) reconcile the
//     auth_users, users_by_identity, and users_by_email indexes inside a
//     single WithTx so reads never observe a half-applied state.
//   - Errors are storage-level (storage.ErrResourceNotFound,
//     storage.ErrResourceAlreadyExists) except for the secondary-index
//     collisions which remain as domain errors (ErrIdentityTaken,
//     ErrEmailTaken). Usecase callers translate storage sentinels to
//     domain sentinels at the boundary.
package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

const errUnmarshalUser = "unmarshal user: %w"

// Repository stores and retrieves auth users in bbolt.
type Repository struct {
	dbm bbolt.Manager
}

// NewRepository creates a new Repository backed by the given Manager.
func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Create persists a brand-new user along with its identity-index and
// email-index entries. Defaults CreatedAt to time.Now() when unset.
//
// Returns storage.ErrResourceAlreadyExists if the user ID already exists.
// Returns domain.ErrEmailTaken / domain.ErrIdentityTaken on index collision.
func (r *Repository) Create(ctx context.Context, user *domain.User) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := []byte(user.ID.String())

		if bbolt.Exists(q, bucketName, key) {
			return fmt.Errorf("user %s: %w", user.ID, storage.ErrResourceAlreadyExists)
		}

		if err := ensureEmailFree(q, user.Email, key); err != nil {
			return err
		}
		if err := ensureIdentitiesFree(q, user.Identities, key); err != nil {
			return err
		}

		if user.CreatedAt.IsZero() {
			user.CreatedAt = time.Now()
		}

		if err := bbolt.Put(q, bucketName, key, internal.DomainToAuthUserMeta(user)); err != nil {
			return fmt.Errorf("put user: %w", err)
		}

		if err := writeIdentityIndex(q, user.Identities, key); err != nil {
			return err
		}

		return writeEmailIndex(q, user.Email, key)
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

// Update persists changes to an existing user and reconciles the identity
// and email indexes against the new slice / email value.
//
// Returns storage.ErrResourceNotFound if no record with user.ID exists.
// Returns domain.ErrIdentityTaken / domain.ErrEmailTaken on index collision.
//
// Caller MUST first read the user (GetByID) and modify it — Update has no
// field-level merge. Zero-valued fields wipe disk state. SetPassword and
// SetMembershipVersion are the dedicated read-modify-write setters for
// those two slots.
func (r *Repository) Update(ctx context.Context, user *domain.User) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := []byte(user.ID.String())

		prev, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("user %s: %w", user.ID, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		if err := reconcileIdentityIndex(q, prev.Identities, user.Identities, key); err != nil {
			return err
		}
		if err := reconcileEmailIndex(q, prev.Email, user.Email, key); err != nil {
			return err
		}

		if err := bbolt.Put(q, bucketName, key, internal.DomainToAuthUserMeta(user)); err != nil {
			return fmt.Errorf("put user: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

// SetMembershipVersion bumps the optimistic-lock counter for group
// memberships. Owned by the membership usecase — must run inside the same
// PAP write transaction as the underlying g-rule changes.
// Returns storage.ErrResourceNotFound if the user does not exist.
func (r *Repository) SetMembershipVersion(ctx context.Context, userID uuid.UUID, version int64) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := []byte(userID.String())

		m, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("user %s: %w", userID, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		m.MembershipVersion = version

		if err := bbolt.Put(q, bucketName, key, m); err != nil {
			return fmt.Errorf("put user membership metadata: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("set membership version: %w", err)
	}

	return nil
}

// SetPassword updates the password hash and password_change_required flag for
// a user. Returns storage.ErrResourceNotFound if the user does not exist.
func (r *Repository) SetPassword(
	ctx context.Context,
	userID uuid.UUID,
	hash string,
	changeRequired bool,
) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := []byte(userID.String())

		m, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("user %s: %w", userID, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		m.PasswordHash = hash
		m.PasswordChangeRequired = changeRequired

		if err := bbolt.Put(q, bucketName, key, m); err != nil {
			return fmt.Errorf("put user: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}

// GetByID returns the user with the given ID.
// Returns storage.ErrResourceNotFound if no such user exists.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	m, err := bbolt.Get[internal.AuthUserMeta](r.dbm.GetQuerier(ctx), bucketName, []byte(id.String()))
	if errors.Is(err, bbolt.ErrNotFound) {
		return nil, fmt.Errorf("user %s: %w", id, storage.ErrResourceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return internal.AuthUserMetaToDomain(m), nil
}

// GetByIdentity looks up a user by one of their identities.
// Returns storage.ErrResourceNotFound if no such user exists.
func (r *Repository) GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error) {
	var user *domain.User

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		idKey := []byte(provider + identitySep + subject)

		userID := q.Bucket(bucketIdentities).Get(idKey)
		if userID == nil {
			return fmt.Errorf("user identity %s:%s: %w", provider, subject, storage.ErrResourceNotFound)
		}

		// Copy: the index bucket slice is only valid for this tx, and we use
		// it again as a bucket key below.
		idCopy := make([]byte, len(userID))
		copy(idCopy, userID)

		m, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, idCopy)
		if errors.Is(err, bbolt.ErrNotFound) {
			// Inconsistent index — should not happen but we handle it.
			return fmt.Errorf("user %s: %w", idCopy, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		user = internal.AuthUserMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user by identity: %w", err)
	}

	return user, nil
}

// GetByEmail returns the user whose normalized Email matches.
// Returns storage.ErrResourceNotFound if no such user exists.
//
// Caller MUST pre-normalize the input via domain.NormalizeEmail — the index
// is keyed by the canonical form, so a raw uppercase or NFD-decomposed query
// will miss.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user *domain.User

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)

		userID := q.Bucket(bucketEmails).Get([]byte(email))
		if userID == nil {
			return fmt.Errorf("user %s: %w", email, storage.ErrResourceNotFound)
		}

		idCopy := make([]byte, len(userID))
		copy(idCopy, userID)

		m, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, idCopy)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("user %s: %w", idCopy, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		user = internal.AuthUserMetaToDomain(m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

// GetSystemUser returns the unique user with System == true. Used by the
// bootstrap procedure to find an already-provisioned superadmin without
// keying off any external identity.
//
// Returns storage.ErrResourceNotFound when no system user exists. The bbolt
// schema does NOT enforce uniqueness — if multiple system users somehow
// exist (data corruption or hand-written DB edits), the first scan-order
// match is returned.
func (r *Repository) GetSystemUser(ctx context.Context) (*domain.User, error) {
	var user *domain.User

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
		codec := bbolt.JSONCodec[internal.AuthUserMeta]{}

		return bucket.ForEach(func(_, v []byte) error {
			if user != nil {
				return nil
			}

			var m internal.AuthUserMeta
			if err := codec.Unmarshal(v, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			if m.System {
				user = internal.AuthUserMetaToDomain(m)
			}

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("get system user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("system user: %w", storage.ErrResourceNotFound)
	}

	return user, nil
}

// Delete removes the user with the given ID along with all index entries.
// Returns storage.ErrResourceNotFound if the user does not exist.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.dbm.WithTx(ctx, func(ctx context.Context) error {
		q := r.dbm.GetQuerier(ctx)
		key := []byte(id.String())

		m, err := bbolt.Get[internal.AuthUserMeta](q, bucketName, key)
		if errors.Is(err, bbolt.ErrNotFound) {
			return fmt.Errorf("user %s: %w", id, storage.ErrResourceNotFound)
		}
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}

		if err := bbolt.Delete(q, bucketName, key); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		idx := q.Bucket(bucketIdentities)
		for _, ident := range m.Identities {
			if err := idx.Delete(identityKey(ident)); err != nil {
				return fmt.Errorf("delete identity index: %w", err)
			}
		}

		if m.Email != "" {
			if err := q.Bucket(bucketEmails).Delete([]byte(m.Email)); err != nil {
				return fmt.Errorf("delete email index: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

// List returns users matching filter, applies search + sort, and slices the
// result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// bbolt keys users by ID, not Email, so an explicit AnyUser=false scope still
// requires a full bucket scan to match Emails. This is acceptable at the
// current user cardinality (tens to hundreds).
func (r *Repository) List(
	ctx context.Context,
	filter domain.UserFilter,
	params domain.UserListParams,
) ([]*domain.User, int, error) {
	var matches []*domain.User

	err := r.dbm.WithReadTx(ctx, func(ctx context.Context) error {
		bucket := r.dbm.GetQuerier(ctx).Bucket(bucketName)
		codec := bbolt.JSONCodec[internal.AuthUserMeta]{}

		return bucket.ForEach(func(_, v []byte) error {
			var m internal.AuthUserMeta
			if err := codec.Unmarshal(v, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			u := internal.AuthUserMetaToDomain(m)

			if !filter.AnyUser {
				if _, ok := filter.UserIDs[u.ID.String()]; !ok {
					return nil
				}
			}

			if !matchesUserSearch(u, filter.Search) {
				return nil
			}

			matches = append(matches, u)

			return nil
		})
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	sortUsers(matches, params.Sort)
	total := len(matches)
	paginated := paginateUsers(matches, params.Offset, params.Limit)

	return paginated, total, nil
}

// ListAll returns every user without filter / pagination. Convenience for
// callers that genuinely need the global view; new code that filters by
// caller permissions MUST use List.
func (r *Repository) ListAll(ctx context.Context) ([]*domain.User, error) {
	users, _, err := r.List(ctx, domain.UserFilter{AnyUser: true}, domain.UserListParams{})
	if err != nil {
		return nil, err
	}

	return users, nil
}

// ensureEmailFree returns domain.ErrEmailTaken if email is owned by a
// different user. A nil or empty email is a no-op (email is optional).
func ensureEmailFree(q bbolt.Querier, email string, ownerKey []byte) error {
	if email == "" {
		return nil
	}
	if owner := q.Bucket(bucketEmails).Get([]byte(email)); owner != nil && !bytes.Equal(owner, ownerKey) {
		return fmt.Errorf("email %s: %w", email, domain.ErrEmailTaken)
	}

	return nil
}

// ensureIdentitiesFree returns domain.ErrIdentityTaken if any identity is
// owned by a different user.
func ensureIdentitiesFree(q bbolt.Querier, identities []domain.Identity, ownerKey []byte) error {
	idx := q.Bucket(bucketIdentities)
	for _, ident := range identities {
		ikey := identityKey(ident)
		if owner := idx.Get(ikey); owner != nil && !bytes.Equal(owner, ownerKey) {
			return fmt.Errorf(
				"identity %s:%s: %w",
				ident.Provider, ident.Subject, domain.ErrIdentityTaken,
			)
		}
	}

	return nil
}

// writeIdentityIndex puts every identity → ownerKey mapping. Caller must have
// pre-validated freedom via ensureIdentitiesFree.
func writeIdentityIndex(q bbolt.Querier, identities []domain.Identity, ownerKey []byte) error {
	idx := q.Bucket(bucketIdentities)
	for _, ident := range identities {
		if err := idx.Put(identityKey(ident), ownerKey); err != nil {
			return fmt.Errorf("put identity index: %w", err)
		}
	}

	return nil
}

// writeEmailIndex puts email → ownerKey. No-op when email is empty.
func writeEmailIndex(q bbolt.Querier, email string, ownerKey []byte) error {
	if email == "" {
		return nil
	}
	if err := q.Bucket(bucketEmails).Put([]byte(email), ownerKey); err != nil {
		return fmt.Errorf("put email index: %w", err)
	}

	return nil
}

// reconcileIdentityIndex diffs prev vs next identity slices and applies the
// minimal set of index mutations: delete entries no longer present, then
// add entries newly present (rejecting collisions with ErrIdentityTaken).
// Identities unchanged across prev/next are left untouched.
func reconcileIdentityIndex(
	q bbolt.Querier,
	prev, next []domain.Identity,
	ownerKey []byte,
) error {
	oldKeys := identityKeySet(prev)
	newKeys := identityKeySet(next)
	idx := q.Bucket(bucketIdentities)

	for k := range oldKeys {
		if _, kept := newKeys[k]; kept {
			continue
		}
		if err := idx.Delete([]byte(k)); err != nil {
			return fmt.Errorf("delete stale identity index %q: %w", k, err)
		}
	}
	for k := range newKeys {
		if _, was := oldKeys[k]; was {
			continue
		}
		if owner := idx.Get([]byte(k)); owner != nil && !bytes.Equal(owner, ownerKey) {
			return fmt.Errorf("identity %q: %w", k, domain.ErrIdentityTaken)
		}
		if err := idx.Put([]byte(k), ownerKey); err != nil {
			return fmt.Errorf("put identity index %q: %w", k, err)
		}
	}

	return nil
}

// reconcileEmailIndex updates the email→user index when the email changed.
// Empty prev means "no old entry to remove"; empty next means "no new entry
// to add". A change to an email already owned by another user returns
// domain.ErrEmailTaken.
func reconcileEmailIndex(q bbolt.Querier, prev, next string, ownerKey []byte) error {
	if prev == next {
		return nil
	}

	emails := q.Bucket(bucketEmails)
	if prev != "" {
		if err := emails.Delete([]byte(prev)); err != nil {
			return fmt.Errorf("delete old email index: %w", err)
		}
	}
	if next == "" {
		return nil
	}
	if err := ensureEmailFree(q, next, ownerKey); err != nil {
		return err
	}

	return writeEmailIndex(q, next, ownerKey)
}

func matchesUserSearch(u *domain.User, search string) bool {
	if search == "" {
		return true
	}

	needle := strings.ToLower(search)

	return strings.Contains(strings.ToLower(u.Email), needle) ||
		strings.Contains(strings.ToLower(u.DisplayName), needle)
}

func sortUsers(users []*domain.User, params domain.SortParams) {
	sort.Slice(users, func(i, j int) bool {
		a, b := users[i], users[j]

		var less bool

		switch params.Field {
		case "name":
			less = a.DisplayName < b.DisplayName
		case "last_login":
			less = a.LastLoginAt.Before(b.LastLoginAt)
		default:
			less = a.Email < b.Email
		}

		if params.Desc {
			return !less
		}

		return less
	})
}

func paginateUsers(users []*domain.User, offset, limit int) []*domain.User {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(users) {
		return []*domain.User{}
	}

	end := len(users)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return users[offset:end]
}
