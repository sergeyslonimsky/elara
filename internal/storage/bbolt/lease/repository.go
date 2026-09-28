// Package lease is the bbolt-backed repository for domain.Lease — the TTL
// grants that etcd clients attach keys to for locks and leader election.
//
// Repository contract:
//   - Three buckets: `leases` holds the record, `lease_keys` maps a lease to the
//     keys attached to it, `lease_expiry` orders leases by due time so the
//     expiry sweep is a bounded prefix walk rather than a full scan.
//   - Every write touches more than one bucket. Atomicity is the caller's
//     responsibility: wrap in Manager.WithTx when the partial state matters —
//     and for a lease it always matters, because a record without its expiry
//     index entry never expires.
//   - Renew deletes the old expiry-index entry before writing the new one.
//     Skipping that leaves the lease reachable under a stale due time, and the
//     sweep would revoke a lease that was renewed on schedule.
//   - Expiry is read from the store rather than from a process-local timer, so
//     the sweep survives a restart and stays correct once the store replicates
//     (README roadmap: Raft-based HA).
//   - All methods read/write through the pkg/bbolt querier, which joins the
//     surrounding transaction when present in ctx.
package lease

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/storage/internal"
	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// errStopIteration ends a ForEach walk early. ForEach is used rather than a
// cursor on purpose: a cursor needs a live transaction and silently yields
// nothing on an auto-querier, which would turn "no lease is due" and "this call
// was made outside a transaction" into the same answer.
var errStopIteration = errors.New("stop iteration")

type Repository struct {
	dbm bbolt.Manager
}

func NewRepository(dbm bbolt.Manager) *Repository {
	return &Repository{dbm: dbm}
}

// Grant stores a new lease and its expiry-index entry.
//
// Returns storage.ErrResourceAlreadyExists when the ID is taken, which is how a
// caller detects the (astronomically unlikely) random-ID collision and redraws.
func (r *Repository) Grant(ctx context.Context, l *domain.Lease) error {
	q := r.dbm.GetQuerier(ctx)

	if bbolt.Exists(q, bucketLeases, leaseKey(l.ID)) {
		return fmt.Errorf("lease %d: %w", l.ID, storage.ErrResourceAlreadyExists)
	}

	if err := bbolt.Put(q, bucketLeases, leaseKey(l.ID), internal.DomainToLeaseMeta(l)); err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	if err := q.Bucket(bucketLeaseExpiry).Put(expiryKey(l.ExpiresAt, l.ID), nil); err != nil {
		return fmt.Errorf("grant lease: expiry index: %w", err)
	}

	return nil
}

// Get returns the lease identified by id.
// Returns storage.ErrResourceNotFound if missing.
func (r *Repository) Get(ctx context.Context, id int64) (*domain.Lease, error) {
	m, err := r.load(ctx, id)
	if err != nil {
		return nil, err
	}

	return internal.LeaseMetaToDomain(m), nil
}

// Renew moves a lease's expiry, keeping the expiry index in step.
// Returns storage.ErrResourceNotFound if missing.
func (r *Repository) Renew(ctx context.Context, id int64, expiresAt time.Time) error {
	q := r.dbm.GetQuerier(ctx)

	m, err := r.load(ctx, id)
	if err != nil {
		return err
	}

	// Delete before write: the index key encodes the expiry, so a new entry does
	// not overwrite the old one — it joins it, and the stale entry would make the
	// sweep revoke a live lease.
	if err := q.Bucket(bucketLeaseExpiry).Delete(expiryKey(m.ExpiresAt, id)); err != nil {
		return fmt.Errorf("renew lease: drop stale expiry index: %w", err)
	}

	m.ExpiresAt = expiresAt

	if err := bbolt.Put(q, bucketLeases, leaseKey(id), m); err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}

	if err := q.Bucket(bucketLeaseExpiry).Put(expiryKey(expiresAt, id), nil); err != nil {
		return fmt.Errorf("renew lease: expiry index: %w", err)
	}

	return nil
}

// Revoke removes a lease, its expiry entry and every lease_keys entry, and
// returns the keys that were attached to it.
//
// Deleting the keys themselves is not this repository's business — it owns lease
// state, not config content. The caller deletes the returned keys through the
// config write path so that watch notification and revision allocation stay in
// one place.
//
// Returns storage.ErrResourceNotFound if the lease is already gone.
func (r *Repository) Revoke(ctx context.Context, id int64) ([]domain.KeyRef, error) {
	q := r.dbm.GetQuerier(ctx)

	m, err := r.load(ctx, id)
	if err != nil {
		return nil, err
	}

	refs, err := r.KeysOf(ctx, id)
	if err != nil {
		return nil, err
	}

	idx := q.Bucket(bucketLeaseKeys)

	for _, ref := range refs {
		if err := idx.Delete(leaseKeysKey(id, ref)); err != nil {
			return nil, fmt.Errorf("revoke lease: drop key index: %w", err)
		}
	}

	if err := q.Bucket(bucketLeaseExpiry).Delete(expiryKey(m.ExpiresAt, id)); err != nil {
		return nil, fmt.Errorf("revoke lease: drop expiry index: %w", err)
	}

	if err := bbolt.Delete(q, bucketLeases, leaseKey(id)); err != nil {
		return nil, fmt.Errorf("revoke lease: %w", err)
	}

	return refs, nil
}

// AttachKey records that ref is held by the lease.
//
// Does not verify that the lease exists or is live — the caller checks that
// before writing the key itself, and re-reading here would double the work on
// every Put that carries a lease.
func (r *Repository) AttachKey(ctx context.Context, id int64, ref domain.KeyRef) error {
	q := r.dbm.GetQuerier(ctx)

	if err := q.Bucket(bucketLeaseKeys).Put(leaseKeysKey(id, ref), nil); err != nil {
		return fmt.Errorf("attach key to lease: %w", err)
	}

	return nil
}

// DetachKey drops the lease's claim on ref. Deleting an entry that is not there
// is not an error: a key overwritten without a lease, or deleted twice, both
// arrive here as a no-op.
func (r *Repository) DetachKey(ctx context.Context, id int64, ref domain.KeyRef) error {
	q := r.dbm.GetQuerier(ctx)

	if err := q.Bucket(bucketLeaseKeys).Delete(leaseKeysKey(id, ref)); err != nil {
		return fmt.Errorf("detach key from lease: %w", err)
	}

	return nil
}

// KeysOf returns the keys attached to the lease, in index order.
func (r *Repository) KeysOf(ctx context.Context, id int64) ([]domain.KeyRef, error) {
	var refs []domain.KeyRef

	prefix := leaseKeysPrefix(id)

	err := r.dbm.GetQuerier(ctx).Bucket(bucketLeaseKeys).ForEach(func(k, _ []byte) error {
		if len(k) <= len(prefix) || !bytes.HasPrefix(k, prefix) {
			return nil
		}

		refs = append(refs, parseLeaseKeysKey(k[len(prefix):]))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan lease keys: %w", err)
	}

	return refs, nil
}

// List returns every lease, in ID order.
func (r *Repository) List(ctx context.Context) ([]*domain.Lease, error) {
	metas, err := bbolt.List[internal.LeaseMeta](r.dbm.GetQuerier(ctx), bucketLeases)
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	leases := make([]*domain.Lease, 0, len(metas))
	for _, m := range metas {
		leases = append(leases, internal.LeaseMetaToDomain(m))
	}

	return leases, nil
}

// ExpiredBefore returns the IDs of leases whose expiry is strictly before t, at
// most limit of them, soonest first. A limit of zero or less means no cap.
//
// Strictly before, to agree with domain.Lease.IsExpired: a lease is still alive
// at exactly its expiry instant.
//
// The walk stops at the first lease that is not due, so cost is proportional to
// what expired rather than to how many leases exist — that is what the
// timestamp-first index key buys.
func (r *Repository) ExpiredBefore(ctx context.Context, t time.Time, limit int) ([]int64, error) {
	var ids []int64

	cutoff := nanoBytes(t)

	err := r.dbm.GetQuerier(ctx).Bucket(bucketLeaseExpiry).ForEach(func(k, _ []byte) error {
		if len(k) < idSize || bytes.Compare(k[:idSize], cutoff) >= 0 {
			return errStopIteration
		}

		id, ok := parseExpiryKey(k)
		if !ok {
			return nil
		}

		ids = append(ids, id)

		if limit > 0 && len(ids) >= limit {
			return errStopIteration
		}

		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return nil, fmt.Errorf("scan lease expiry index: %w", err)
	}

	return ids, nil
}

// load fetches the raw record, translating a missing key into the storage-level
// not-found error every caller here needs.
func (r *Repository) load(ctx context.Context, id int64) (internal.LeaseMeta, error) {
	m, err := bbolt.Get[internal.LeaseMeta](r.dbm.GetQuerier(ctx), bucketLeases, leaseKey(id))
	if errors.Is(err, bbolt.ErrNotFound) {
		return internal.LeaseMeta{}, fmt.Errorf("lease %d: %w", id, storage.ErrResourceNotFound)
	}

	if err != nil {
		return internal.LeaseMeta{}, fmt.Errorf("get lease: %w", err)
	}

	return m, nil
}
