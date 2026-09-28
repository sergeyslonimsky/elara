package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/txevents"
)

// KVRangeOpts configures RangeQuery — the etcd-compatible gRPC API's Range
// RPC, translated into namespace/path bounds by the handler
// (see etcdv3.SplitRange).
type KVRangeOpts struct {
	Limit    int64
	Revision int64 // > 0 selects a point-in-time read from the history bucket.
	KeysOnly bool
}

// CurrentRevisionValue returns the current global etcd-compatible-API
// revision counter.
func (s *Service) CurrentRevisionValue(ctx context.Context) (int64, error) {
	rev, err := s.kv.CurrentRevisionValue(ctx)
	if err != nil {
		return 0, fmt.Errorf("current revision: %w", err)
	}

	return rev, nil
}

// RangeKVs returns the KV pairs in [startNS/startPath, endNS/endPath)
// without fetching the current revision — for callers that only need the
// matched pairs (e.g. Txn's compare evaluation). Use RangeQuery instead when
// the current revision is actually needed (e.g. the Range RPC's response
// header) — fetching it is a second bbolt read transaction, wasted on the
// high-contention Txn-compare path otherwise.
//
// Deliberately NOT wrapped in txm.WithTx: doing so would force every call
// onto bbolt's single writable transaction, serializing reads that today
// run concurrently through the repo's own internal read-only transaction
// (plans/etcd-usecase-decoupling/plan.md Ш1's "оценить сериализацию" risk).
// Called from inside a future outer WithTx (e.g. an atomic Txn
// orchestration), the repo's own WithReadTx still joins that transaction
// correctly instead of opening a second one.
func (s *Service) RangeKVs(
	ctx context.Context,
	startNS, startPath, endNS, endPath string,
	opts KVRangeOpts,
) ([]*domain.KVPair, bool, error) {
	kvs, more, err := s.kv.RangeQuery(
		ctx,
		startNS, startPath, endNS, endPath,
		opts.Limit, opts.Revision, opts.KeysOnly,
	)
	if err != nil {
		return nil, false, fmt.Errorf("range query: %w", err)
	}

	return kvs, more, nil
}

// RangeQuery is RangeKVs plus the current revision, for the etcd-compatible
// gRPC API's Range RPC (whose response header needs it).
func (s *Service) RangeQuery(
	ctx context.Context,
	startNS, startPath, endNS, endPath string,
	opts KVRangeOpts,
) ([]*domain.KVPair, int64, bool, error) {
	kvs, more, err := s.RangeKVs(ctx, startNS, startPath, endNS, endPath, opts)
	if err != nil {
		return nil, 0, false, err
	}

	rev, err := s.kv.CurrentRevisionValue(ctx)
	if err != nil {
		return nil, 0, false, fmt.Errorf("current revision: %w", err)
	}

	return kvs, rev, more, nil
}

// PutKey validates value against the namespace's attached JSON Schema (if
// any) and writes it at namespace/path — the etcd-compatible gRPC API's Put
// RPC, translated into namespace/path by the handler (see
// etcdv3.SplitKey).
//
// Deliberately does NOT run content.Normalize or domain.ValidatePath: etcd
// values are opaque bytes (locks, leader-election payloads), not
// necessarily structured config content — running the ConnectRPC path's
// content pipeline here would silently reformat or reject values the etcd
// wire protocol has always accepted byte-for-byte (see plan's byte-fidelity
// risk). Schema validation is the one piece of ConnectRPC-path business
// logic this method deliberately brings over — see plan finding #1.
//
// lease is three-valued (domain.EffectiveLease): a non-nil zero detaches the
// key, a non-nil non-zero attaches it, and nil means ignore_lease — keep the
// existing attachment, which requires the key to already exist. Callers on the
// ordinary write path pass a pointer to zero, not nil.
func (s *Service) PutKey(
	ctx context.Context,
	namespace, path string,
	value []byte,
	lease domain.LeaseAssignment,
) (*domain.KVPair, int64, error) {
	format := domain.DetectFormatFromPath(path)

	if err := s.schemaValidator.Validate(ctx, namespace, path, string(value), format); err != nil {
		return nil, 0, fmt.Errorf("schema validation: %w", err)
	}

	outer, pending, owner := txevents.Install(ctx)

	var (
		prev   *domain.KVPair
		newRev int64
	)

	err := s.txm.WithTx(outer, func(ctx context.Context) error {
		if err := s.ensureLeaseAssignable(ctx, namespace, path, lease); err != nil {
			return err
		}

		p, rev, err := s.kv.PutKey(ctx, namespace, path, value, lease)
		prev = p
		newRev = rev

		if err != nil {
			return fmt.Errorf("put key: %w", err)
		}

		if err := s.moveLeaseClaim(ctx, namespace, path, prev, lease); err != nil {
			return err
		}

		cfg := kvPutConfig(namespace, path, value, format, p, rev)
		if p != nil {
			pending.Add(func(ctx context.Context) { s.watcher.NotifyUpdated(ctx, cfg) })
		} else {
			pending.Add(func(ctx context.Context) { s.watcher.NotifyCreated(ctx, cfg) })
		}

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("put key tx: %w", err)
	}

	if owner {
		pending.Flush(ctx)
	}

	return prev, newRev, nil
}

// DeleteRangeKeys deletes the KV pairs in [startNS/startPath,
// endNS/endPath) — the etcd-compatible gRPC API's DeleteRange RPC.
func (s *Service) DeleteRangeKeys(
	ctx context.Context,
	startNS, startPath, endNS, endPath string,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	outer, pending, owner := txevents.Install(ctx)

	var (
		deleted []*domain.KVPair
		newRev  int64
	)

	err := s.txm.WithTx(outer, func(ctx context.Context) error {
		d, rev, err := s.kv.DeleteRangeKeys(ctx, startNS, startPath, endNS, endPath, returnPrev)
		deleted = d
		newRev = rev

		if err != nil {
			return fmt.Errorf("delete range keys: %w", err)
		}

		if err := s.dropLeaseClaims(ctx, d); err != nil {
			return err
		}

		for _, kv := range d {
			deletedPath, deletedNS := kv.Path, kv.Namespace
			pending.Add(func(ctx context.Context) {
				s.watcher.NotifyDeleted(ctx, deletedPath, deletedNS, rev)
			})
		}

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("delete range keys tx: %w", err)
	}

	if owner {
		pending.Flush(ctx)
	}

	return deleted, newRev, nil
}

// DeleteKeys deletes an explicit set of keys in one revision and one watch
// batch, releasing each key's lease claim as it goes.
//
// This is the method a lease revoke calls, and the reason it exists on the
// config service rather than being assembled by the caller: the pending-events
// collector may only be flushed by whoever installed it (see kv_events.go), so
// an outside caller looping over DeleteRangeKeys would publish each deletion
// before its own transaction had committed.
func (s *Service) DeleteKeys(
	ctx context.Context,
	refs []domain.KeyRef,
	returnPrev bool,
) ([]*domain.KVPair, int64, error) {
	outer, pending, owner := txevents.Install(ctx)

	var (
		deleted []*domain.KVPair
		newRev  int64
	)

	err := s.txm.WithTx(outer, func(ctx context.Context) error {
		d, rev, err := s.kv.DeleteKeys(ctx, refs, returnPrev)
		deleted = d
		newRev = rev

		if err != nil {
			return fmt.Errorf("delete keys: %w", err)
		}

		if err := s.dropLeaseClaims(ctx, d); err != nil {
			return err
		}

		for _, kv := range d {
			deletedPath, deletedNS := kv.Path, kv.Namespace
			pending.Add(func(ctx context.Context) {
				s.watcher.NotifyDeleted(ctx, deletedPath, deletedNS, rev)
			})
		}

		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("delete keys tx: %w", err)
	}

	if owner {
		pending.Flush(ctx)
	}

	return deleted, newRev, nil
}

// kvPutConfig builds the domain.Config that a KV write notifies with.
//
// Intentionally incomplete: ContentHash, Metadata and Locked are not populated.
// The etcd wire protocol's own response (mvccpb.KeyValue) never needs them, and
// the only consumer of this value is watch/webhook notification, which does not
// read those fields either.
func kvPutConfig(
	namespace, path string,
	value []byte,
	format domain.Format,
	prev *domain.KVPair,
	newRev int64,
) *domain.Config {
	cfg := &domain.Config{
		Path:      path,
		Namespace: namespace,
		Content:   string(value),
		Format:    format,
		Revision:  newRev,
		UpdatedAt: time.Now(),
	}

	if prev != nil {
		cfg.Version = prev.Version + 1
		cfg.CreateRevision = prev.CreateRevision

		return cfg
	}

	cfg.Version = 1
	cfg.CreateRevision = newRev
	cfg.CreatedAt = cfg.UpdatedAt

	return cfg
}
