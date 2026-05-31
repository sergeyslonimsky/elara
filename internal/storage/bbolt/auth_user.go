package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

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

// Upsert creates or updates a user. It is called on every OIDC login.
// When the user already exists, only Name, Picture, and LastLoginAt are updated.
func (r *UserRepo) Upsert(ctx context.Context, user *domain.User) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(user.Email)
		// ...

		existing := b.Get(key)
		if existing == nil {
			// New user — set CreatedAt now.
			user.CreatedAt = time.Now()
		} else {
			// Existing user — preserve CreatedAt from storage.
			var m authUserMeta
			if err := json.Unmarshal(existing, &m); err != nil {
				return fmt.Errorf(errUnmarshalUser, err)
			}

			user.CreatedAt = m.CreatedAt
			user.System = m.System
			if user.Source == "" {
				user.Source = m.Source
			}
			user.PasswordHash = m.PasswordHash
			user.PasswordChangeRequired = m.PasswordChangeRequired
			// Membership version is owned by the membership usecase — Upsert
			// preserves whatever bbolt already holds so concurrent OIDC
			// logins don't reset the optimistic-lock counter.
			user.MembershipVersion = m.MembershipVersion
		}

		data, err := json.Marshal(domainToAuthUserMeta(user))
		if err != nil {
			return fmt.Errorf("marshal user: %w", err)
		}

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	return nil
}

// SetMembershipVersion bumps the optimistic-lock counter for group
// memberships. Owned by the membership usecase — must run inside the same
// PAP write transaction as the underlying g-rule changes.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) SetMembershipVersion(ctx context.Context, email string, version int64) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(email)

		existing := b.Get(key)
		if existing == nil {
			return domain.NewNotFoundError("user", email)
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

		return b.Put(key, data)
	})
	if err != nil {
		return fmt.Errorf("set membership version: %w", err)
	}

	return nil
}

// SetPassword updates the password hash and password_change_required flag for a user.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) SetPassword(ctx context.Context, email, hash string, changeRequired bool) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(email)

		existing := b.Get(key)
		if existing == nil {
			return domain.NewNotFoundError("user", email)
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

// Get returns the user with the given email.
// Returns domain.ErrNotFound if no such user exists.
func (r *UserRepo) Get(ctx context.Context, email string) (*domain.User, error) {
	var user *domain.User

	err := r.manager.View(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		data := b.Get([]byte(email))

		if data == nil {
			return domain.NewNotFoundError("user", email)
		}

		var m authUserMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf(errUnmarshalUser, err)
		}

		user = authUserMetaToDomain(&m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

// Delete removes the user with the given email.
// Returns domain.ErrNotFound if the user does not exist.
func (r *UserRepo) Delete(ctx context.Context, email string) error {
	err := r.manager.Update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketAuthUsers))
		key := []byte(email)

		if b.Get(key) == nil {
			return domain.NewNotFoundError("user", email)
		}

		return b.Delete(key)
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
// bbolt keys users by Email, so an explicit AnyUser=false scope could in
// principle do per-email point-lookups. We keep a single ForEach scan because
// the search filter still requires touching every record and user cardinality
// is low (tens to hundreds).
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
				if _, ok := filter.Usernames[u.Email]; !ok {
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
		strings.Contains(strings.ToLower(u.Name), needle)
}

func sortUsers(users []*domain.User, params domain.SortParams) {
	sort.Slice(users, func(i, j int) bool {
		a, b := users[i], users[j]

		var less bool

		switch params.Field {
		case "name":
			less = a.Name < b.Name
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
