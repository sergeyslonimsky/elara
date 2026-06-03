package bbolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const errUnmarshalUser = "unmarshal user: %w"

// UserRepo stores and retrieves auth users in bbolt.
type UserRepo struct {
	manager *Manager
}

// NewUserRepo creates a new UserRepo backed by the given Manager.
func NewUserRepo(manager *Manager) *UserRepo {
	return &UserRepo{manager: manager}
}

// Create persists a brand-new user along with its identity-index entries.
//
// Contract:
//   - user.ID must be non-nil (caller mints the UUID).
//   - user.CreatedAt is set to now if zero.
//   - Returns domain.ErrAlreadyExists if a record with the same ID exists.
//   - Returns domain.ErrIdentityTaken if any of user.Identities maps to a
//     different user in the secondary index.
//
// Create is mechanical: it does not apply defaults beyond CreatedAt and does
// not validate Status or Identities semantics. UserService is the validated
// path; the repo only guards index integrity.
func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		users := tx.Bucket([]byte(bucketAuthUsers))
		idx := tx.Bucket([]byte(bucketUserIdentities))
		emails := tx.Bucket([]byte(bucketUsersByEmail))

		key := []byte(user.ID.String())
		if users.Get(key) != nil {
			return domain.NewAlreadyExistsError("user", user.ID.String())
		}

		if err := ensureEmailFree(emails, user.Email, key); err != nil {
			return err
		}
		if err := ensureIdentitiesFree(idx, user.Identities, key); err != nil {
			return err
		}

		if user.CreatedAt.IsZero() {
			user.CreatedAt = time.Now()
		}

		data, err := json.Marshal(domainToAuthUserMeta(user))
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		if err = users.Put(key, data); err != nil {
			return fmt.Errorf("put user: %w", err)
		}

		if err := writeIdentityIndex(idx, user.Identities, key); err != nil {
			return err
		}

		return writeEmailIndex(emails, user.Email, key)
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

// ensureEmailFree returns ErrEmailTaken if the email is owned by a different user.
// A nil or empty email is a no-op (email is optional).
func ensureEmailFree(emails *bolt.Bucket, email string, ownerKey []byte) error {
	if email == "" {
		return nil
	}
	if owner := emails.Get([]byte(email)); owner != nil && !bytes.Equal(owner, ownerKey) {
		return fmt.Errorf("email %s: %w", email, domain.ErrEmailTaken)
	}

	return nil
}

// ensureIdentitiesFree returns ErrIdentityTaken if any identity is owned by a different user.
func ensureIdentitiesFree(idx *bolt.Bucket, identities []domain.Identity, ownerKey []byte) error {
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
func writeIdentityIndex(idx *bolt.Bucket, identities []domain.Identity, ownerKey []byte) error {
	for _, ident := range identities {
		if err := idx.Put(identityKey(ident), ownerKey); err != nil {
			return fmt.Errorf("put identity index: %w", err)
		}
	}

	return nil
}

// writeEmailIndex puts email → ownerKey. No-op when email is empty.
func writeEmailIndex(emails *bolt.Bucket, email string, ownerKey []byte) error {
	if email == "" {
		return nil
	}
	if err := emails.Put([]byte(email), ownerKey); err != nil {
		return fmt.Errorf("put email index: %w", err)
	}

	return nil
}

// Update persists changes to an existing user and reconciles the identity
// index against the new user.Identities slice.
//
// Contract:
//   - user.ID must reference an existing record; otherwise returns ErrNotFound.
//   - Identities removed from the slice → their index entries are deleted.
//   - Identities newly added → their index entries are written; if any of
//     them already maps to a different user, returns ErrIdentityTaken.
//   - All other fields of the persisted record are OVERWRITTEN with what the
//     caller passes. There is no field-level merge — callers MUST first read
//     the user (GetByID) and modify, otherwise zero-valued fields wipe disk
//     state. The dedicated setters SetPassword and SetMembershipVersion stay
//     the canonical way to mutate those two slots without read-modify-write.
//
// Update does NOT enforce append-only identities or per-field
// immutability; those policies live in the *auth.UserService primitives
// (LinkIdentity / RecordLogin / transitionStatus / BootstrapSync) where
// the System flag is consulted. Callers outside those primitives must
// keep the read-modify-write contract above.
func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		users := tx.Bucket([]byte(bucketAuthUsers))
		idx := tx.Bucket([]byte(bucketUserIdentities))
		emails := tx.Bucket([]byte(bucketUsersByEmail))

		key := []byte(user.ID.String())
		prevBytes := users.Get(key)
		if prevBytes == nil {
			return domain.NewNotFoundError("user", user.ID.String())
		}

		var prev authUserMeta
		if err := json.Unmarshal(prevBytes, &prev); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		if err := reconcileIdentityIndex(idx, prev.Identities, user.Identities, key); err != nil {
			return err
		}
		if err := reconcileEmailIndex(emails, prev.Email, user.Email, key); err != nil {
			return err
		}

		data, err := json.Marshal(domainToAuthUserMeta(user))
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}
		if err := users.Put(key, data); err != nil {
			return fmt.Errorf("put user: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

// reconcileIdentityIndex diffs prev vs next identity slices and applies the
// minimal set of index mutations: delete entries no longer present, then
// add entries newly present (rejecting collisions with ErrIdentityTaken).
// Identities unchanged across prev/next are left untouched.
func reconcileIdentityIndex(
	idx *bolt.Bucket,
	prev, next []domain.Identity,
	ownerKey []byte,
) error {
	oldKeys := identityKeySet(prev)
	newKeys := identityKeySet(next)

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
// Empty prev means "no old entry to remove"; empty next means "no new entry to add".
// A change to an email already owned by another user returns ErrEmailTaken.
func reconcileEmailIndex(emails *bolt.Bucket, prev, next string, ownerKey []byte) error {
	if prev == next {
		return nil
	}
	if prev != "" {
		if err := emails.Delete([]byte(prev)); err != nil {
			return fmt.Errorf("delete old email index: %w", err)
		}
	}
	if next == "" {
		return nil
	}
	if err := ensureEmailFree(emails, next, ownerKey); err != nil {
		return err
	}

	return writeEmailIndex(emails, next, ownerKey)
}

func identityKey(i domain.Identity) []byte {
	return []byte(string(i.Provider) + "\x00" + i.Subject)
}

func identityKeySet(identities []domain.Identity) map[string]struct{} {
	out := make(map[string]struct{}, len(identities))
	for _, i := range identities {
		out[string(identityKey(i))] = struct{}{}
	}

	return out
}

// SetMembershipVersion bumps the optimistic-lock counter for group
// memberships. Owned by the membership usecase — must run inside the same
// PAP write transaction as the underlying g-rule changes.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) SetMembershipVersion(ctx context.Context, userID uuid.UUID, version int64) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(userID.String())

		existing := b.Get(key)
		if existing == nil {
			return domain.NewNotFoundError("user", userID.String())
		}

		var m authUserMeta
		if err := json.Unmarshal(existing, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		m.MembershipVersion = version

		data, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("put user membership metadata: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("set membership version: %w", err)
	}

	return nil
}

// SetPassword updates the password hash and password_change_required flag for a user.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) SetPassword(ctx context.Context, userID uuid.UUID, hash string, changeRequired bool) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(userID.String())

		existing := b.Get(key)
		if existing == nil {
			return domain.NewNotFoundError("user", userID.String())
		}

		var m authUserMeta
		if err := json.Unmarshal(existing, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		m.PasswordHash = hash
		m.PasswordChangeRequired = changeRequired

		data, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}

// GetByID returns the user with the given ID.
// Returns domain.ErrNotFound if no such user exists.
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user *domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		data := b.Get([]byte(id.String()))

		if data == nil {
			return domain.NewNotFoundError("user", id.String())
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		user = authUserMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}

// GetByIdentity looks up a user by one of their identities.
// Returns domain.ErrNotFound if no such user exists.
func (r *UserRepo) GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error) {
	var user *domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(bucketUserIdentities))
		idKey := []byte(provider + "\x00" + subject)
		userID := idx.Get(idKey)

		if userID == nil {
			return domain.NewNotFoundError("user identity", fmt.Sprintf("%s:%s", provider, subject))
		}

		b := tx.Bucket([]byte(bucketAuthUsers))
		data := b.Get(userID)
		if data == nil {
			// Inconsistent index — should not happen but we handle it.
			return domain.NewNotFoundError("user", string(userID))
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		user = authUserMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user by identity: %w", err)
	}

	return user, nil
}

// Delete removes the user with the given ID.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(id.String())

		data := b.Get(key)
		if data == nil {
			return domain.NewNotFoundError("user", id.String())
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		if err := b.Delete(key); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		// Clean up identity index
		idx := tx.Bucket([]byte(bucketUserIdentities))
		for _, ident := range m.Identities {
			idKey := []byte(string(ident.Provider) + "\x00" + ident.Subject)
			if err := idx.Delete(idKey); err != nil {
				return fmt.Errorf("delete identity index: %w", err)
			}
		}

		// Clean up email index
		if m.Email != "" {
			emails := tx.Bucket([]byte(bucketUsersByEmail))
			if err := emails.Delete([]byte(m.Email)); err != nil {
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

// GetSystemUser returns the unique user with System == true. Used by the
// bootstrap procedure to find an already-provisioned superadmin without
// keying off any external identity (since post EL-50 §3.3.2 the identity
// slice no longer carries a stable origin marker).
//
// Returns ErrNotFound when no system user exists. The bbolt schema does NOT
// enforce uniqueness — if multiple system users somehow exist (data corruption
// or hand-written DB edits), the first scan-order match is returned. Callers
// that need to detect that invariant violation must do an additional pass.
func (r *UserRepo) GetSystemUser(ctx context.Context) (*domain.User, error) {
	var user *domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))

		return b.ForEach(func(_, v []byte) error {
			if user != nil {
				return nil
			}
			var m authUserMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}
			if m.System {
				user = authUserMetaToDomain(&m)
			}

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("get system user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("empty system user: %w", domain.ErrNotFound)
	}

	return user, nil
}

// GetByEmail returns the user whose normalized Email matches.
// Returns domain.ErrNotFound if no such user exists.
//
// Caller MUST pre-normalize the input via domain.NormalizeEmail — the index
// is keyed by the canonical form, so a raw uppercase or NFD-decomposed query
// will miss. Repo trusts caller; UserService.GetByEmail does the normalize
// step centrally.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user *domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		emails := tx.Bucket([]byte(bucketUsersByEmail))
		userID := emails.Get([]byte(email))
		if userID == nil {
			return domain.NewNotFoundError("user", email)
		}

		users := tx.Bucket([]byte(bucketAuthUsers))
		data := users.Get(userID)
		if data == nil {
			// Inconsistent index — should not happen.
			return domain.NewNotFoundError("user", string(userID))
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		user = authUserMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

// List returns users matching filter, applies search + sort, and slices the
// result by params.Offset / params.Limit. Total is the count after
// filter+search but before pagination so callers can render page indicators.
//
// bbolt keys users by ID, not Email, so an explicit AnyUser=false scope still
// requires a full bucket scan to match Emails. This is acceptable at the
// current user cardinality (tens to hundreds).
func (r *UserRepo) List(
	ctx context.Context,
	filter domain.UserFilter,
	params domain.UserListParams,
) ([]*domain.User, int, error) {
	var matches []*domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))

		return b.ForEach(func(_, v []byte) error {
			var m authUserMeta
			if err := json.Unmarshal(v, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			u := authUserMetaToDomain(&m)

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
func (r *UserRepo) ListAll(ctx context.Context) ([]*domain.User, error) {
	users, _, err := r.List(ctx, domain.UserFilter{AnyUser: true}, domain.UserListParams{})
	if err != nil {
		return nil, err
	}

	return users, nil
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
