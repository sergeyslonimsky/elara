package config

import (
	"bytes"
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/txevents"
)

// KVCompareTarget names the stored field a compare inspects.
type KVCompareTarget int

const (
	KVCompareVersion KVCompareTarget = iota + 1
	KVCompareCreateRevision
	KVCompareModRevision
	KVCompareValue
)

// KVCompareResult names the relation asserted between the stored field and the
// wanted value.
type KVCompareResult int

const (
	KVCompareEqual KVCompareResult = iota + 1
	KVCompareNotEqual
	KVCompareGreater
	KVCompareLess
)

// KVOpKind names which operation a KVOp carries.
type KVOpKind int

const (
	KVOpRange KVOpKind = iota + 1
	KVOpPut
	KVOpDeleteRange
	KVOpTxn
)

// KVCompare is one condition of a transaction's guard, over the key range
// [StartNS/StartPath, EndNS/EndPath). An empty end means a single key.
//
// Bounds arrive already split into namespace and path: key encoding is the
// handler's business (see etcdv3.SplitRange), not this layer's.
type KVCompare struct {
	StartNS, StartPath string
	EndNS, EndPath     string
	Target             KVCompareTarget
	Result             KVCompareResult
	WantRevision       int64  // Version, CreateRevision and ModRevision targets.
	WantValue          []byte // Value target.
}

// KVOp is one operation in a transaction branch. Exactly one pointer is set,
// selected by Kind.
type KVOp struct {
	Kind        KVOpKind
	Range       *KVRangeOp
	Put         *KVPutOp
	DeleteRange *KVDeleteRangeOp
	Txn         *KVTxnInput // Nested transaction; etcd permits these.
}

type KVRangeOp struct {
	StartNS, StartPath string
	EndNS, EndPath     string
	Opts               KVRangeOpts
}

type KVPutOp struct {
	Namespace, Path string
	Value           []byte
	// Lease follows PutKey's three-valued convention: nil is ignore_lease, a
	// pointer to zero detaches, non-zero attaches.
	Lease domain.LeaseAssignment
}

type KVDeleteRangeOp struct {
	StartNS, StartPath string
	EndNS, EndPath     string
	ReturnPrev         bool
}

// KVTxnInput is a transaction: evaluate every Compare, then run Success if all
// held or Failure otherwise.
type KVTxnInput struct {
	Compare []KVCompare
	Success []KVOp
	Failure []KVOp
}

// KVOpResult mirrors KVOp: exactly one pointer is set, selected by Kind.
type KVOpResult struct {
	Kind        KVOpKind
	Range       *KVRangeResult
	Put         *KVPutResult
	DeleteRange *KVDeleteRangeResult
	Txn         *KVTxnResult
}

type KVRangeResult struct {
	KVs      []*domain.KVPair
	Revision int64
	More     bool
}

type KVPutResult struct {
	Prev     *domain.KVPair
	Revision int64
}

type KVDeleteRangeResult struct {
	Deleted  []*domain.KVPair
	Revision int64
}

type KVTxnResult struct {
	Responses []KVOpResult
	Revision  int64
	Succeeded bool
}

// Txn evaluates in.Compare and runs exactly one branch, atomically.
//
// Every compare and every operation share one transaction, which is what makes
// compare-and-swap safe under concurrency: the guard cannot be evaluated against
// a state that another writer changes before this transaction's write lands.
// Nested WithTx flattens (docs/adr/0001-usecase-owned-transactions.md), so the
// per-operation boundaries inside PutKey and DeleteRangeKeys join this one
// instead of opening their own.
//
// Watch notifications are buffered and published after the commit — see
// kv_events.go. Publishing per operation, as this path used to, would let an
// observer see a write that a later failing operation rolled back.
//
// A transaction whose branches only read does NOT open a writable transaction:
// bbolt permits one writer at a time, so doing that would serialise read-only
// transactions behind every writer for no gain. Known limitation, unchanged from
// before this method existed: those reads are therefore not a single snapshot —
// each joins its own short read transaction. Fixing that needs WithReadTx on
// storage.Manager, which is deliberately out of scope here.
func (s *Service) Txn(ctx context.Context, in KVTxnInput) (KVTxnResult, error) {
	if !in.hasWrites() {
		res, err := s.runTxn(ctx, in)
		if err != nil {
			return KVTxnResult{}, err
		}

		return res, nil
	}

	outer, pending, owner := txevents.Install(ctx)

	var res KVTxnResult

	err := s.txm.WithTx(outer, func(ctx context.Context) error {
		r, err := s.runTxn(ctx, in)
		if err != nil {
			return err
		}

		res = r

		return nil
	})
	if err != nil {
		return KVTxnResult{}, fmt.Errorf("txn tx: %w", err)
	}

	if owner {
		pending.Flush(ctx)
	}

	return res, nil
}

// hasWrites reports whether either branch would write.
//
// Deliberately conservative: the branch is not known until the compares have
// been evaluated, and evaluating them is itself part of the transaction. A
// transaction that writes only in the branch it does not take therefore still
// takes the writable path — the alternative would be evaluating the guard
// outside the boundary that is supposed to protect it.
func (in KVTxnInput) hasWrites() bool {
	return opsWrite(in.Success) || opsWrite(in.Failure)
}

func opsWrite(ops []KVOp) bool {
	for _, op := range ops {
		switch op.Kind {
		case KVOpPut, KVOpDeleteRange:
			return true
		case KVOpTxn:
			if op.Txn != nil && op.Txn.hasWrites() {
				return true
			}
		case KVOpRange:
		}
	}

	return false
}

func (s *Service) runTxn(ctx context.Context, in KVTxnInput) (KVTxnResult, error) {
	succeeded, err := s.evalCompares(ctx, in.Compare)
	if err != nil {
		return KVTxnResult{}, err
	}

	ops := in.Success
	if !succeeded {
		ops = in.Failure
	}

	responses, lastRev, err := s.runOps(ctx, ops)
	if err != nil {
		return KVTxnResult{}, err
	}

	// No operation advanced the revision, but the header still needs one. A
	// range already read it, so reuse that before paying for another read
	// transaction; only a branch that read nothing has to ask the store.
	if lastRev == 0 {
		lastRev = revisionFromResponses(responses)
	}

	if lastRev == 0 {
		lastRev, err = s.kv.CurrentRevisionValue(ctx)
		if err != nil {
			return KVTxnResult{}, fmt.Errorf("current revision: %w", err)
		}
	}

	return KVTxnResult{Succeeded: succeeded, Revision: lastRev, Responses: responses}, nil
}

// revisionFromResponses returns a revision already observed by a read in this
// branch, or 0 if none was.
//
// Reads do not advance the revision, so they deliberately contribute 0 to the
// header candidate in runOps. They do however observe the current one, and
// discarding that just to re-read it is a second bbolt transaction for a value
// already in hand.
func revisionFromResponses(responses []KVOpResult) int64 {
	for _, res := range responses {
		switch res.Kind {
		case KVOpRange:
			if res.Range != nil && res.Range.Revision > 0 {
				return res.Range.Revision
			}
		case KVOpTxn:
			if res.Txn != nil && res.Txn.Revision > 0 {
				return res.Txn.Revision
			}
		case KVOpPut, KVOpDeleteRange:
		}
	}

	return 0
}

func (s *Service) evalCompares(ctx context.Context, compares []KVCompare) (bool, error) {
	for _, cmp := range compares {
		ok, err := s.evalCompare(ctx, cmp)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}

func (s *Service) evalCompare(ctx context.Context, cmp KVCompare) (bool, error) {
	// RangeKVs rather than RangeQuery: the current revision is not needed here,
	// and fetching it would be a second read on the hottest path in Txn.
	kvs, _, err := s.RangeKVs(ctx, cmp.StartNS, cmp.StartPath, cmp.EndNS, cmp.EndPath, KVRangeOpts{})
	if err != nil {
		return false, fmt.Errorf("compare range: %w", err)
	}

	// Nothing matched: every revision target compares against 0 and the value
	// target against nil, which is what makes "create if absent" expressible.
	if len(kvs) == 0 {
		return compareKV(cmp, nil), nil
	}

	for _, kv := range kvs {
		if !compareKV(cmp, kv) {
			return false, nil
		}
	}

	return true, nil
}

func (s *Service) runOps(ctx context.Context, ops []KVOp) ([]KVOpResult, int64, error) {
	results := make([]KVOpResult, 0, len(ops))

	var lastRev int64

	for _, op := range ops {
		res, rev, err := s.runOp(ctx, op)
		if err != nil {
			return nil, 0, err
		}

		if rev > lastRev {
			lastRev = rev
		}

		results = append(results, res)
	}

	return results, lastRev, nil
}

func (s *Service) runOp(ctx context.Context, op KVOp) (KVOpResult, int64, error) {
	switch op.Kind {
	case KVOpRange:
		return s.runRangeOp(ctx, op.Range)
	case KVOpPut:
		return s.runPutOp(ctx, op.Put)
	case KVOpDeleteRange:
		return s.runDeleteRangeOp(ctx, op.DeleteRange)
	case KVOpTxn:
		return s.runNestedTxnOp(ctx, op.Txn)
	default:
		return KVOpResult{}, 0, fmt.Errorf(
			"run op: %w",
			domain.NewValidationError("op", "unknown transaction operation"),
		)
	}
}

func (s *Service) runRangeOp(ctx context.Context, op *KVRangeOp) (KVOpResult, int64, error) {
	kvs, rev, more, err := s.RangeQuery(ctx, op.StartNS, op.StartPath, op.EndNS, op.EndPath, op.Opts)
	if err != nil {
		return KVOpResult{}, 0, fmt.Errorf("txn range: %w", err)
	}

	// A range does not advance the revision, so it does not contribute to the
	// response header — hence 0 rather than rev.
	return KVOpResult{
		Kind:  KVOpRange,
		Range: &KVRangeResult{KVs: kvs, Revision: rev, More: more},
	}, 0, nil
}

func (s *Service) runPutOp(ctx context.Context, op *KVPutOp) (KVOpResult, int64, error) {
	prev, rev, err := s.PutKey(ctx, op.Namespace, op.Path, op.Value, op.Lease)
	if err != nil {
		return KVOpResult{}, 0, fmt.Errorf("txn put: %w", err)
	}

	return KVOpResult{Kind: KVOpPut, Put: &KVPutResult{Prev: prev, Revision: rev}}, rev, nil
}

func (s *Service) runDeleteRangeOp(ctx context.Context, op *KVDeleteRangeOp) (KVOpResult, int64, error) {
	deleted, rev, err := s.DeleteRangeKeys(ctx, op.StartNS, op.StartPath, op.EndNS, op.EndPath, op.ReturnPrev)
	if err != nil {
		return KVOpResult{}, 0, fmt.Errorf("txn delete range: %w", err)
	}

	return KVOpResult{
		Kind:        KVOpDeleteRange,
		DeleteRange: &KVDeleteRangeResult{Deleted: deleted, Revision: rev},
	}, rev, nil
}

func (s *Service) runNestedTxnOp(ctx context.Context, in *KVTxnInput) (KVOpResult, int64, error) {
	if in == nil {
		return KVOpResult{}, 0, fmt.Errorf(
			"run op: %w",
			domain.NewValidationError("txn", "nested transaction is empty"),
		)
	}

	// Calls Txn rather than runTxn so a nested read-only transaction keeps its
	// own boundary decision. Inside an outer writable transaction its WithTx
	// flattens; the collector is likewise already installed, so its events join
	// the outer flush.
	res, err := s.Txn(ctx, *in)
	if err != nil {
		return KVOpResult{}, 0, fmt.Errorf("txn nested: %w", err)
	}

	return KVOpResult{Kind: KVOpTxn, Txn: &res}, res.Revision, nil
}

// compareKV evaluates one condition against one pair, or against absence when kv
// is nil. Semantics are those of etcd's Compare and are asserted by the
// conformance suite; an unknown target or relation is false rather than an error,
// matching what the handler did before this moved.
func compareKV(cmp KVCompare, kv *domain.KVPair) bool {
	var (
		version, createRev, modRev int64
		value                      []byte
	)

	if kv != nil {
		version = kv.Version
		createRev = kv.CreateRevision
		modRev = kv.ModRevision
		value = kv.Value
	}

	switch cmp.Target {
	case KVCompareVersion:
		return compareInt64(cmp.Result, version, cmp.WantRevision)
	case KVCompareCreateRevision:
		return compareInt64(cmp.Result, createRev, cmp.WantRevision)
	case KVCompareModRevision:
		return compareInt64(cmp.Result, modRev, cmp.WantRevision)
	case KVCompareValue:
		return compareBytes(cmp.Result, value, cmp.WantValue)
	default:
		return false
	}
}

func compareInt64(op KVCompareResult, got, want int64) bool {
	switch op {
	case KVCompareEqual:
		return got == want
	case KVCompareNotEqual:
		return got != want
	case KVCompareGreater:
		return got > want
	case KVCompareLess:
		return got < want
	default:
		return false
	}
}

func compareBytes(op KVCompareResult, got, want []byte) bool {
	cmp := bytes.Compare(got, want)

	switch op {
	case KVCompareEqual:
		return cmp == 0
	case KVCompareNotEqual:
		return cmp != 0
	case KVCompareGreater:
		return cmp > 0
	case KVCompareLess:
		return cmp < 0
	default:
		return false
	}
}
