package config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// ensureLeaseAssignable rejects a Put whose lease cannot hold the key.
//
// Two cases, both of which etcd rejects as well:
//   - a requested lease that does not exist, or has already expired — attaching
//     to it would write a key that the next expiry sweep immediately deletes;
//   - ignore_lease against a key that does not exist — there is no existing
//     attachment to keep.
//
// Called from inside the caller's transaction, so neither check can race the
// write that follows it.
func (s *Service) ensureLeaseAssignable(
	ctx context.Context,
	namespace, path string,
	lease domain.LeaseAssignment,
) error {
	if lease.Ignore {
		found, err := s.keyExists(ctx, namespace, path)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("ignore_lease: %w", domain.NewNotFoundError("config", path))
		}

		return nil
	}

	if lease.ID == 0 {
		return nil
	}

	l, err := s.leases.Get(ctx, lease.ID)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return fmt.Errorf("lease %d: %w", lease.ID, domain.ErrLeaseNotFound)
		}

		return fmt.Errorf("get lease: %w", err)
	}

	if err := l.EnsureLive(time.Now()); err != nil {
		return fmt.Errorf("lease %d: %w", lease.ID, err)
	}

	return nil
}

// moveLeaseClaim brings the lease-to-keys index in line with the attachment the
// Put just wrote: the old claim goes, the new one arrives, and an unchanged
// attachment costs nothing.
func (s *Service) moveLeaseClaim(
	ctx context.Context,
	namespace, path string,
	prev *domain.KVPair,
	lease domain.LeaseAssignment,
) error {
	var previous int64
	if prev != nil {
		previous = prev.Lease
	}

	effective := lease.Resolve(prev)
	if effective == previous {
		return nil
	}

	ref := domain.KeyRef{Namespace: namespace, Path: path}

	if previous != 0 {
		if err := s.leases.DetachKey(ctx, previous, ref); err != nil {
			return fmt.Errorf("detach previous lease: %w", err)
		}
	}

	if effective != 0 {
		if err := s.leases.AttachKey(ctx, effective, ref); err != nil {
			return fmt.Errorf("attach lease: %w", err)
		}
	}

	return nil
}

// dropLeaseClaims releases the claims of keys that were just deleted.
//
// Without this the index keeps pointing at keys that no longer exist, and a
// later revoke of that lease would try to delete them again — harmless today,
// because a missing key is skipped, but it also means the lease's key set grows
// without bound for a client that rewrites the same lease repeatedly.
func (s *Service) dropLeaseClaims(ctx context.Context, deleted []*domain.KVPair) error {
	for _, kv := range deleted {
		if kv.Lease == 0 {
			continue
		}

		ref := domain.KeyRef{Namespace: kv.Namespace, Path: kv.Path}

		if err := s.leases.DetachKey(ctx, kv.Lease, ref); err != nil {
			return fmt.Errorf("detach lease from deleted key: %w", err)
		}
	}

	return nil
}

// keyExists reports whether a single key is present, without reading its value.
func (s *Service) keyExists(ctx context.Context, namespace, path string) (bool, error) {
	kvs, _, err := s.kv.RangeQuery(ctx, namespace, path, "", "", 1, 0, true)
	if err != nil {
		return false, fmt.Errorf("check key exists: %w", err)
	}

	return len(kvs) > 0, nil
}
