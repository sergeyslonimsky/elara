package etcdv3

import (
	"context"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

// Txn translates an etcd transaction onto usecase/config and converts the result
// back.
//
// It orchestrates nothing. Evaluating the guard, choosing a branch, running the
// operations and ordering watch notifications against the commit all live in the
// usecase, because they are what the transaction boundary exists to sequence —
// see docs/adr/0003-responsibility-placement.md.
//
// Authorization stays here, and now covers **both** branches rather than only
// the one that runs. The branch is not known until the compares have been
// evaluated inside the transaction, and etcd itself authorizes a whole Txn
// request up front, so this both matches etcd and avoids a permission check that
// depends on data read under the lock.
func (s *KVServer) Txn(
	ctx context.Context,
	req *etcdserverpb.TxnRequest,
) (*etcdserverpb.TxnResponse, error) {
	in, err := txnInput(req)
	if err != nil {
		return nil, err
	}

	if err := s.checkTxnAccess(ctx, in); err != nil {
		return nil, err
	}

	res, err := s.usecase.Txn(ctx, in)
	if err != nil {
		if ns, ok := firstWriteNamespace(in); ok {
			s.recordRejectedWrite(ctx, "txn", ns, err)
		}

		return nil, toKVStatus(err, "txn", "")
	}

	return txnResponse(req, res)
}

func (s *KVServer) checkTxnAccess(ctx context.Context, in configuc.KVTxnInput) error {
	for _, cmp := range in.Compare {
		if err := s.checkRangeAccess(ctx, cmp.StartNS, cmp.EndNS, domain.ActionRead); err != nil {
			return err
		}
	}

	if err := s.checkOpsAccess(ctx, in.Success); err != nil {
		return err
	}

	return s.checkOpsAccess(ctx, in.Failure)
}

func (s *KVServer) checkOpsAccess(ctx context.Context, ops []configuc.KVOp) error {
	for _, op := range ops {
		var err error

		switch op.Kind {
		case configuc.KVOpRange:
			err = s.checkRangeAccess(ctx, op.Range.StartNS, op.Range.EndNS, domain.ActionRead)
		case configuc.KVOpPut:
			err = s.checkAccess(ctx, op.Put.Namespace, domain.ActionWrite)
		case configuc.KVOpDeleteRange:
			err = s.checkRangeAccess(
				ctx,
				op.DeleteRange.StartNS,
				op.DeleteRange.EndNS,
				domain.ActionWrite,
			)
		case configuc.KVOpTxn:
			if op.Txn == nil {
				return status.Errorf(codes.InvalidArgument, "nested txn is empty")
			}

			err = s.checkTxnAccess(ctx, *op.Txn)
		default:
			err = status.Errorf(codes.InvalidArgument, "unknown txn request op")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func txnInput(req *etcdserverpb.TxnRequest) (configuc.KVTxnInput, error) {
	compares := make([]configuc.KVCompare, 0, len(req.GetCompare()))

	for _, cmp := range req.GetCompare() {
		converted, err := txnCompare(cmp)
		if err != nil {
			return configuc.KVTxnInput{}, err
		}

		compares = append(compares, converted)
	}

	success, err := txnOps(req.GetSuccess())
	if err != nil {
		return configuc.KVTxnInput{}, err
	}

	failure, err := txnOps(req.GetFailure())
	if err != nil {
		return configuc.KVTxnInput{}, err
	}

	return configuc.KVTxnInput{Compare: compares, Success: success, Failure: failure}, nil
}

func txnCompare(cmp *etcdserverpb.Compare) (configuc.KVCompare, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(cmp.GetKey(), cmp.GetRangeEnd())
	if !ok {
		return configuc.KVCompare{}, status.Errorf(
			codes.InvalidArgument,
			"invalid compare key: %q",
			string(cmp.GetKey()),
		)
	}

	out := configuc.KVCompare{
		StartNS:   startNS,
		StartPath: startPath,
		EndNS:     endNS,
		EndPath:   endPath,
		Target:    compareTarget(cmp.GetTarget()),
		Result:    compareResult(cmp.GetResult()),
	}

	switch cmp.GetTarget() {
	case etcdserverpb.Compare_VERSION:
		out.WantRevision = cmp.GetVersion()
	case etcdserverpb.Compare_CREATE:
		out.WantRevision = cmp.GetCreateRevision()
	case etcdserverpb.Compare_MOD:
		out.WantRevision = cmp.GetModRevision()
	case etcdserverpb.Compare_VALUE:
		out.WantValue = cmp.GetValue()
	case etcdserverpb.Compare_LEASE:
	}

	return out, nil
}

// compareTarget and compareResult map an unrecognised enum to the zero value
// rather than an error, on purpose. The usecase treats an unknown target or
// relation as a condition that does not hold, which is what this path did before
// the orchestration moved: an odd compare selects the failure branch instead of
// rejecting the request.
func compareTarget(t etcdserverpb.Compare_CompareTarget) configuc.KVCompareTarget {
	switch t {
	case etcdserverpb.Compare_VERSION:
		return configuc.KVCompareVersion
	case etcdserverpb.Compare_CREATE:
		return configuc.KVCompareCreateRevision
	case etcdserverpb.Compare_MOD:
		return configuc.KVCompareModRevision
	case etcdserverpb.Compare_VALUE:
		return configuc.KVCompareValue
	case etcdserverpb.Compare_LEASE:
		return 0
	default:
		return 0
	}
}

func compareResult(r etcdserverpb.Compare_CompareResult) configuc.KVCompareResult {
	switch r {
	case etcdserverpb.Compare_EQUAL:
		return configuc.KVCompareEqual
	case etcdserverpb.Compare_NOT_EQUAL:
		return configuc.KVCompareNotEqual
	case etcdserverpb.Compare_GREATER:
		return configuc.KVCompareGreater
	case etcdserverpb.Compare_LESS:
		return configuc.KVCompareLess
	default:
		return 0
	}
}

func txnOps(ops []*etcdserverpb.RequestOp) ([]configuc.KVOp, error) {
	out := make([]configuc.KVOp, 0, len(ops))

	for _, op := range ops {
		converted, err := txnOp(op)
		if err != nil {
			return nil, err
		}

		out = append(out, converted)
	}

	return out, nil
}

func txnOp(op *etcdserverpb.RequestOp) (configuc.KVOp, error) {
	switch r := op.GetRequest().(type) {
	case *etcdserverpb.RequestOp_RequestRange:
		return txnRangeOp(r.RequestRange)
	case *etcdserverpb.RequestOp_RequestPut:
		return txnPutOp(r.RequestPut)
	case *etcdserverpb.RequestOp_RequestDeleteRange:
		return txnDeleteRangeOp(r.RequestDeleteRange)
	case *etcdserverpb.RequestOp_RequestTxn:
		nested, err := txnInput(r.RequestTxn)
		if err != nil {
			return configuc.KVOp{}, err
		}

		return configuc.KVOp{Kind: configuc.KVOpTxn, Txn: &nested}, nil
	default:
		return configuc.KVOp{}, status.Errorf(codes.InvalidArgument, "unknown txn request op")
	}
}

func txnRangeOp(req *etcdserverpb.RangeRequest) (configuc.KVOp, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.GetKey(), req.GetRangeEnd())
	if !ok {
		return configuc.KVOp{}, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	return configuc.KVOp{
		Kind: configuc.KVOpRange,
		Range: &configuc.KVRangeOp{
			StartNS:   startNS,
			StartPath: startPath,
			EndNS:     endNS,
			EndPath:   endPath,
			Opts: configuc.KVRangeOpts{
				Limit:    req.GetLimit(),
				Revision: req.GetRevision(),
				KeysOnly: req.GetKeysOnly(),
			},
		},
	}, nil
}

func txnPutOp(req *etcdserverpb.PutRequest) (configuc.KVOp, error) {
	namespace, path, ok := SplitKey(req.GetKey())
	if !ok {
		return configuc.KVOp{}, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	if req.GetIgnoreValue() {
		return configuc.KVOp{}, status.Errorf(codes.Unimplemented, "ignore_value is not supported")
	}

	return configuc.KVOp{
		Kind: configuc.KVOpPut,
		Put: &configuc.KVPutOp{
			Namespace: namespace,
			Path:      path,
			Value:     req.GetValue(),
			Lease:     putLease(req),
		},
	}, nil
}

func txnDeleteRangeOp(req *etcdserverpb.DeleteRangeRequest) (configuc.KVOp, error) {
	startNS, startPath, endNS, endPath, ok := SplitRange(req.GetKey(), req.GetRangeEnd())
	if !ok {
		return configuc.KVOp{}, status.Errorf(
			codes.InvalidArgument,
			"invalid key encoding: %q",
			string(req.GetKey()),
		)
	}

	return configuc.KVOp{
		Kind: configuc.KVOpDeleteRange,
		DeleteRange: &configuc.KVDeleteRangeOp{
			StartNS:    startNS,
			StartPath:  startPath,
			EndNS:      endNS,
			EndPath:    endPath,
			ReturnPrev: req.GetPrevKv(),
		},
	}, nil
}

// txnResponse zips the executed branch's requests with the usecase's results.
// The requests are needed because per-operation wire shaping — sort order,
// count-only, prev_kv — is a property of the request, not of the stored data.
func txnResponse(
	req *etcdserverpb.TxnRequest,
	res configuc.KVTxnResult,
) (*etcdserverpb.TxnResponse, error) {
	ops := req.GetSuccess()
	if !res.Succeeded {
		ops = req.GetFailure()
	}

	if len(ops) != len(res.Responses) {
		return nil, status.Errorf(
			codes.Internal,
			"txn response mismatch: %d operations, %d results",
			len(ops), len(res.Responses),
		)
	}

	responses := make([]*etcdserverpb.ResponseOp, 0, len(res.Responses))

	for i, result := range res.Responses {
		converted, err := txnOpResponse(ops[i], result)
		if err != nil {
			return nil, err
		}

		responses = append(responses, converted)
	}

	return &etcdserverpb.TxnResponse{
		Header:    newHeader(res.Revision),
		Succeeded: res.Succeeded,
		Responses: responses,
	}, nil
}

func txnOpResponse(
	op *etcdserverpb.RequestOp,
	result configuc.KVOpResult,
) (*etcdserverpb.ResponseOp, error) {
	switch r := op.GetRequest().(type) {
	case *etcdserverpb.RequestOp_RequestRange:
		resp := buildRangeResponse(
			r.RequestRange,
			result.Range.KVs,
			result.Range.Revision,
			result.Range.More,
		)

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseRange{ResponseRange: resp},
		}, nil

	case *etcdserverpb.RequestOp_RequestPut:
		resp := &etcdserverpb.PutResponse{Header: newHeader(result.Put.Revision)}
		if r.RequestPut.GetPrevKv() && result.Put.Prev != nil {
			resp.PrevKv = kvPairToProto(result.Put.Prev)
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponsePut{ResponsePut: resp},
		}, nil

	case *etcdserverpb.RequestOp_RequestDeleteRange:
		resp := buildDeleteRangeResponse(
			result.DeleteRange.Revision,
			result.DeleteRange.Deleted,
			r.RequestDeleteRange.GetPrevKv(),
		)

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseDeleteRange{ResponseDeleteRange: resp},
		}, nil

	case *etcdserverpb.RequestOp_RequestTxn:
		resp, err := txnResponse(r.RequestTxn, *result.Txn)
		if err != nil {
			return nil, err
		}

		return &etcdserverpb.ResponseOp{
			Response: &etcdserverpb.ResponseOp_ResponseTxn{ResponseTxn: resp},
		}, nil

	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown txn request op")
	}
}

// firstWriteNamespace reports the namespace of the first writing operation, for
// the rejected-write metric.
//
// Before the orchestration moved, each operation's own handler recorded that
// metric with its own namespace; a failed Txn now surfaces as one error, so the
// label is approximated by the first write. Imperfect on a cross-namespace Txn
// and deliberately so — dropping the metric entirely would be worse.
func firstWriteNamespace(in configuc.KVTxnInput) (string, bool) {
	for _, ops := range [][]configuc.KVOp{in.Success, in.Failure} {
		for _, op := range ops {
			switch op.Kind {
			case configuc.KVOpPut:
				return op.Put.Namespace, true
			case configuc.KVOpDeleteRange:
				return op.DeleteRange.StartNS, true
			case configuc.KVOpTxn:
				if op.Txn == nil {
					continue
				}

				if ns, ok := firstWriteNamespace(*op.Txn); ok {
					return ns, true
				}
			case configuc.KVOpRange:
			}
		}
	}

	return "", false
}
