package etcdv3

import (
	"context"
	"errors"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_lease.go -package=etcdv3_mock . LeaseUsecase

// LeaseUsecase is the surface the Lease RPCs translate onto.
type LeaseUsecase interface {
	Grant(ctx context.Context, id int64, ttl time.Duration) (*domain.Lease, error)
	KeepAlive(ctx context.Context, id int64) (*domain.Lease, error)
	Revoke(ctx context.Context, id int64) error
	TimeToLive(ctx context.Context, id int64, withKeys bool) (*domain.Lease, []domain.KeyRef, error)
	List(ctx context.Context) ([]*domain.Lease, error)
}

// LeaseServer implements etcdserverpb.LeaseServer.
//
// Leases are what etcd clients build distributed locks and leader election on
// (clientv3/concurrency): a session holds a lease, the lock key is attached to
// it, and losing the lease is what releases the lock.
type LeaseServer struct {
	etcdserverpb.UnimplementedLeaseServer

	usecase  LeaseUsecase
	revision MaintenanceRepo
}

func NewLeaseServer(usecase LeaseUsecase, revision MaintenanceRepo) *LeaseServer {
	return &LeaseServer{usecase: usecase, revision: revision}
}

func (s *LeaseServer) LeaseGrant(
	ctx context.Context,
	req *etcdserverpb.LeaseGrantRequest,
) (*etcdserverpb.LeaseGrantResponse, error) {
	if err := checkLeaseWriteAccess(ctx); err != nil {
		return nil, err
	}

	l, err := s.usecase.Grant(ctx, req.GetID(), time.Duration(req.GetTTL())*time.Second)
	if err != nil {
		return nil, toLeaseStatus(err, req.GetID())
	}

	return &etcdserverpb.LeaseGrantResponse{
		Header: newHeader(s.currentRevision(ctx)),
		ID:     l.ID,
		TTL:    int64(l.TTL / time.Second),
	}, nil
}

func (s *LeaseServer) LeaseRevoke(
	ctx context.Context,
	req *etcdserverpb.LeaseRevokeRequest,
) (*etcdserverpb.LeaseRevokeResponse, error) {
	if err := s.checkLeaseKeyAccess(ctx, req.GetID(), domain.ActionWrite); err != nil {
		return nil, err
	}

	if err := s.usecase.Revoke(ctx, req.GetID()); err != nil {
		return nil, toLeaseStatus(err, req.GetID())
	}

	return &etcdserverpb.LeaseRevokeResponse{
		Header: newHeader(s.currentRevision(ctx)),
	}, nil
}

// LeaseKeepAlive renews leases for as long as the client keeps asking.
//
// A lease the server will not renew — unknown, or already expired — is answered
// with TTL 0 rather than a stream error. That is the wire contract clientv3
// relies on: KeepAliveOnce turns a non-positive TTL into ErrLeaseNotFound
// (client/v3/lease.go), and the keep-alive loop closes the channel on it, which
// is how concurrency.Session learns its lease is gone. Failing the stream
// instead would take down every lease multiplexed onto it.
func (s *LeaseServer) LeaseKeepAlive(stream etcdserverpb.Lease_LeaseKeepAliveServer) error {
	ctx := stream.Context()

	if err := checkLeaseWriteAccess(ctx); err != nil {
		return err
	}

	for {
		req, err := stream.Recv()
		if err != nil {
			// io.EOF is the client half-closing; anything else ends the stream
			// the same way, and gRPC reports it to the caller.
			return err //nolint:wrapcheck // stream errors are terminal, wrapping hides the status
		}

		resp := &etcdserverpb.LeaseKeepAliveResponse{
			Header: newHeader(s.currentRevision(ctx)),
			ID:     req.GetID(),
		}

		l, err := s.usecase.KeepAlive(ctx, req.GetID())
		switch {
		case err == nil:
			resp.TTL = int64(l.TTL / time.Second)
		case errors.Is(err, domain.ErrLeaseNotFound), errors.Is(err, domain.ErrLeaseExpired):
			resp.TTL = 0
		default:
			return toLeaseStatus(err, req.GetID())
		}

		if err := stream.Send(resp); err != nil {
			return err //nolint:wrapcheck // same as Recv above
		}
	}
}

func (s *LeaseServer) LeaseTimeToLive(
	ctx context.Context,
	req *etcdserverpb.LeaseTimeToLiveRequest,
) (*etcdserverpb.LeaseTimeToLiveResponse, error) {
	if req.GetKeys() {
		if err := s.checkLeaseKeyAccess(ctx, req.GetID(), domain.ActionRead); err != nil {
			return nil, err
		}
	}

	l, refs, err := s.usecase.TimeToLive(ctx, req.GetID(), req.GetKeys())
	if err != nil {
		if errors.Is(err, domain.ErrLeaseNotFound) {
			// -1 is how the wire protocol says "no such lease" for this RPC,
			// distinct from 0, which a live lease can legitimately report.
			return &etcdserverpb.LeaseTimeToLiveResponse{
				Header: newHeader(s.currentRevision(ctx)),
				ID:     req.GetID(),
				TTL:    -1,
			}, nil
		}

		return nil, toLeaseStatus(err, req.GetID())
	}

	return &etcdserverpb.LeaseTimeToLiveResponse{
		Header:     newHeader(s.currentRevision(ctx)),
		ID:         l.ID,
		TTL:        int64(l.RemainingTTL(time.Now()) / time.Second),
		GrantedTTL: int64(l.TTL / time.Second),
		Keys:       keyRefsToWire(refs),
	}, nil
}

// LeaseLeases lists lease IDs.
//
// Not filtered by namespace scope, deliberately: an ID carries no key material,
// and every operation that could act on one — revoke, or reading its key set —
// checks the scope of the keys it actually holds. Filtering here would cost a
// key lookup per lease to hide a number that is not secret.
func (s *LeaseServer) LeaseLeases(
	ctx context.Context,
	_ *etcdserverpb.LeaseLeasesRequest,
) (*etcdserverpb.LeaseLeasesResponse, error) {
	leases, err := s.usecase.List(ctx)
	if err != nil {
		return nil, toLeaseStatus(err, 0)
	}

	out := make([]*etcdserverpb.LeaseStatus, 0, len(leases))
	for _, l := range leases {
		out = append(out, &etcdserverpb.LeaseStatus{ID: l.ID})
	}

	return &etcdserverpb.LeaseLeasesResponse{
		Header: newHeader(s.currentRevision(ctx)),
		Leases: out,
	}, nil
}

// checkLeaseKeyAccess gates an operation on a lease by the namespaces of the
// keys it holds.
//
// A lease is not itself scoped to a namespace, but its keys are, and revoking it
// deletes them. Without this check a service token scoped to one namespace could
// delete another's keys knowing nothing but a lease ID.
//
// A lease holding no keys is allowed through on the write-role check alone —
// there is nothing to scope it by, and refusing would make a client unable to
// revoke the lease it just created.
func (s *LeaseServer) checkLeaseKeyAccess(ctx context.Context, id int64, action domain.Action) error {
	claims, _ := authctx.ClaimsFromContext(ctx)
	if claims == nil {
		return nil
	}

	if action == domain.ActionWrite {
		if err := checkLeaseWriteAccess(ctx); err != nil {
			return err
		}
	}

	_, refs, err := s.usecase.TimeToLive(ctx, id, true)
	if err != nil {
		if errors.Is(err, domain.ErrLeaseNotFound) {
			// Let the operation itself report the miss, so that "no such lease"
			// does not arrive as a permission error.
			return nil
		}

		return toLeaseStatus(err, id)
	}

	for _, ref := range refs {
		if !namespaceAllowed(claims, ref.Namespace, action) {
			return status.Errorf(codes.PermissionDenied, "permission denied for namespace %q", ref.Namespace)
		}
	}

	return nil
}

// currentRevision reads the revision for a response header. A failure here is
// not worth failing the RPC over: the lease operation itself succeeded, and the
// header's revision is informational for these RPCs.
func (s *LeaseServer) currentRevision(ctx context.Context) int64 {
	rev, _ := s.revision.CurrentRevisionValue(ctx)

	return rev
}

// checkLeaseWriteAccess requires a token that may write somewhere. Granting and
// renewing are not scoped to a namespace — the keys are, and they are checked
// when they are written.
func checkLeaseWriteAccess(ctx context.Context) error {
	claims, _ := authctx.ClaimsFromContext(ctx)
	if claims == nil {
		return nil
	}

	if domain.Role(claims.Role) != domain.RoleWriter {
		return status.Errorf(codes.PermissionDenied, "lease operations require a writer token")
	}

	return nil
}

func keyRefsToWire(refs []domain.KeyRef) [][]byte {
	if len(refs) == 0 {
		return nil
	}

	keys := make([][]byte, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, JoinKey(ref.Namespace, ref.Path))
	}

	return keys
}

// toLeaseStatus maps domain errors onto the status codes etcd uses for the Lease
// RPCs (see api/v3/v3rpc/rpctypes: NotFound for a missing lease,
// FailedPrecondition for a taken ID, OutOfRange for a rejected TTL).
func toLeaseStatus(err error, id int64) error {
	switch {
	case errors.Is(err, domain.ErrLeaseNotFound), errors.Is(err, domain.ErrLeaseExpired):
		return status.Errorf(codes.NotFound, "etcdserver: requested lease not found: %d", id)
	case errors.Is(err, domain.ErrLeaseExists):
		return status.Errorf(codes.FailedPrecondition, "etcdserver: lease already exists: %d", id)
	case errors.Is(err, domain.ErrLeaseTTLInvalid):
		return status.Errorf(codes.OutOfRange, "etcdserver: invalid lease TTL")
	default:
		return status.Errorf(codes.Internal, "lease operation failed: %v", err)
	}
}
