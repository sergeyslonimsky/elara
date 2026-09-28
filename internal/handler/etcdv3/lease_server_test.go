package etcdv3_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
)

// fakeLeaseUsecase is a hand-written stub, matching how the rest of this package
// fakes its usecases (see fakeKVUsecase).
type fakeLeaseUsecase struct {
	lease *domain.Lease
	keys  []domain.KeyRef

	grantErr     error
	keepAliveErr error
	revokeErr    error
	ttlErr       error

	revoked   []int64
	grantedID int64
	grantTTL  time.Duration
}

func (f *fakeLeaseUsecase) Grant(_ context.Context, id int64, ttl time.Duration) (*domain.Lease, error) {
	if f.grantErr != nil {
		return nil, f.grantErr
	}

	f.grantedID = id
	f.grantTTL = ttl

	return f.lease, nil
}

func (f *fakeLeaseUsecase) KeepAlive(_ context.Context, _ int64) (*domain.Lease, error) {
	if f.keepAliveErr != nil {
		return nil, f.keepAliveErr
	}

	return f.lease, nil
}

func (f *fakeLeaseUsecase) Revoke(_ context.Context, id int64) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}

	f.revoked = append(f.revoked, id)

	return nil
}

func (f *fakeLeaseUsecase) TimeToLive(
	_ context.Context,
	_ int64,
	withKeys bool,
) (*domain.Lease, []domain.KeyRef, error) {
	if f.ttlErr != nil {
		return nil, nil, f.ttlErr
	}

	if !withKeys {
		return f.lease, nil, nil
	}

	return f.lease, f.keys, nil
}

func (f *fakeLeaseUsecase) List(_ context.Context) ([]*domain.Lease, error) {
	if f.lease == nil {
		return nil, nil
	}

	return []*domain.Lease{f.lease}, nil
}

// fakeRevisionRepo satisfies the revision source a response header needs.
type fakeRevisionRepo struct{ rev int64 }

func (f fakeRevisionRepo) CurrentRevisionValue(_ context.Context) (int64, error) {
	return f.rev, nil
}

// fakeKeepAliveStream replays a fixed set of requests and then ends the stream,
// which is how a client half-closing looks to the server.
type fakeKeepAliveStream struct {
	ctx  context.Context //nolint:containedctx // mirrors grpc.ServerStream, same as fakeWatchStream
	reqs []*etcdserverpb.LeaseKeepAliveRequest
	sent []*etcdserverpb.LeaseKeepAliveResponse
}

func (s *fakeKeepAliveStream) Context() context.Context { return s.ctx }

func (s *fakeKeepAliveStream) Send(resp *etcdserverpb.LeaseKeepAliveResponse) error {
	s.sent = append(s.sent, resp)

	return nil
}

func (s *fakeKeepAliveStream) Recv() (*etcdserverpb.LeaseKeepAliveRequest, error) {
	if len(s.reqs) == 0 {
		return nil, io.EOF
	}

	req := s.reqs[0]
	s.reqs = s.reqs[1:]

	return req, nil
}

func (s *fakeKeepAliveStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeKeepAliveStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeKeepAliveStream) SetTrailer(metadata.MD)       {}
func (s *fakeKeepAliveStream) SendMsg(any) error            { return nil }
func (s *fakeKeepAliveStream) RecvMsg(any) error            { return nil }

func liveLease(id int64, ttl, remaining time.Duration) *domain.Lease {
	now := time.Now()

	return &domain.Lease{
		ID:        id,
		TTL:       ttl,
		GrantedAt: now.Add(remaining - ttl),
		ExpiresAt: now.Add(remaining),
	}
}

func newLeaseServer(uc *fakeLeaseUsecase) *etcdv3.LeaseServer {
	return etcdv3.NewLeaseServer(uc, fakeRevisionRepo{rev: 42})
}

func TestLeaseServer_LeaseGrant(t *testing.T) {
	t.Parallel()

	t.Run("answers with the granted id and ttl in seconds", func(t *testing.T) {
		t.Parallel()

		uc := &fakeLeaseUsecase{lease: liveLease(7, 30*time.Second, 30*time.Second)}
		srv := newLeaseServer(uc)

		resp, err := srv.LeaseGrant(t.Context(), &etcdserverpb.LeaseGrantRequest{ID: 7, TTL: 30})
		require.NoError(t, err)
		assert.Equal(t, int64(7), resp.GetID())
		assert.Equal(t, int64(30), resp.GetTTL())
		assert.Equal(t, int64(42), resp.GetHeader().GetRevision())
		assert.Equal(t, 30*time.Second, uc.grantTTL, "seconds on the wire become a Duration inland")
	})

	t.Run("a taken id is FailedPrecondition, as in etcd", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{grantErr: domain.ErrLeaseExists})

		_, err := srv.LeaseGrant(t.Context(), &etcdserverpb.LeaseGrantRequest{ID: 7, TTL: 30})
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("an invalid ttl is OutOfRange", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{grantErr: domain.ErrLeaseTTLInvalid})

		_, err := srv.LeaseGrant(t.Context(), &etcdserverpb.LeaseGrantRequest{TTL: 0})
		assert.Equal(t, codes.OutOfRange, status.Code(err))
	})

	t.Run("a reader token may not grant", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{lease: liveLease(7, time.Minute, time.Minute)})
		ctx := authctx.WithClaims(t.Context(), &authctx.Claims{
			Namespaces: []string{"*"},
			Role:       "reader",
		})

		_, err := srv.LeaseGrant(ctx, &etcdserverpb.LeaseGrantRequest{TTL: 30})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

func TestLeaseServer_LeaseKeepAlive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		usecase *fakeLeaseUsecase
		wantTTL int64
	}{
		{
			name:    "a renewed lease reports its full ttl",
			usecase: &fakeLeaseUsecase{lease: liveLease(7, 30*time.Second, 30*time.Second)},
			wantTTL: 30,
		},
		{
			// Not a stream error: clientv3 turns a non-positive TTL into
			// ErrLeaseNotFound and closes the keep-alive channel, which is how
			// concurrency.Session learns its lease is gone. Failing the stream
			// would take down every lease multiplexed onto it.
			name:    "an unknown lease is answered with ttl 0",
			usecase: &fakeLeaseUsecase{keepAliveErr: domain.ErrLeaseNotFound},
			wantTTL: 0,
		},
		{
			name:    "an expired lease is answered with ttl 0",
			usecase: &fakeLeaseUsecase{keepAliveErr: domain.ErrLeaseExpired},
			wantTTL: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newLeaseServer(tt.usecase)
			stream := &fakeKeepAliveStream{
				ctx:  t.Context(),
				reqs: []*etcdserverpb.LeaseKeepAliveRequest{{ID: 7}},
			}

			err := srv.LeaseKeepAlive(stream)
			require.ErrorIs(t, err, io.EOF, "the client half-closing ends the stream")
			require.Len(t, stream.sent, 1)
			assert.Equal(t, int64(7), stream.sent[0].GetID())
			assert.Equal(t, tt.wantTTL, stream.sent[0].GetTTL())
		})
	}

	t.Run("renews repeatedly on one stream", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{lease: liveLease(7, 10*time.Second, 10*time.Second)})
		stream := &fakeKeepAliveStream{
			ctx: t.Context(),
			reqs: []*etcdserverpb.LeaseKeepAliveRequest{
				{ID: 7}, {ID: 7}, {ID: 7},
			},
		}

		err := srv.LeaseKeepAlive(stream)
		require.ErrorIs(t, err, io.EOF)
		assert.Len(t, stream.sent, 3)
	})
}

func TestLeaseServer_LeaseRevoke(t *testing.T) {
	t.Parallel()

	t.Run("revokes the lease", func(t *testing.T) {
		t.Parallel()

		uc := &fakeLeaseUsecase{lease: liveLease(7, time.Minute, time.Minute)}
		srv := newLeaseServer(uc)

		_, err := srv.LeaseRevoke(t.Context(), &etcdserverpb.LeaseRevokeRequest{ID: 7})
		require.NoError(t, err)
		assert.Equal(t, []int64{7}, uc.revoked)
	})

	t.Run("an unknown lease is NotFound", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{
			lease:     liveLease(7, time.Minute, time.Minute),
			revokeErr: domain.ErrLeaseNotFound,
		})

		_, err := srv.LeaseRevoke(t.Context(), &etcdserverpb.LeaseRevokeRequest{ID: 7})
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("a token cannot revoke a lease holding another namespace's keys", func(t *testing.T) {
		t.Parallel()

		// Without this check, knowing nothing but a lease ID would be enough to
		// delete keys the token has no write access to.
		uc := &fakeLeaseUsecase{
			lease: liveLease(7, time.Minute, time.Minute),
			keys:  []domain.KeyRef{{Namespace: "other", Path: "/lock"}},
		}
		srv := newLeaseServer(uc)
		ctx := authctx.WithClaims(t.Context(), &authctx.Claims{
			Namespaces: []string{"allowed"},
			Role:       "writer",
		})

		_, err := srv.LeaseRevoke(ctx, &etcdserverpb.LeaseRevokeRequest{ID: 7})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.Empty(t, uc.revoked, "the revoke must not reach the usecase")
	})

	t.Run("a token may revoke a lease holding only its own keys", func(t *testing.T) {
		t.Parallel()

		uc := &fakeLeaseUsecase{
			lease: liveLease(7, time.Minute, time.Minute),
			keys:  []domain.KeyRef{{Namespace: "allowed", Path: "/lock"}},
		}
		srv := newLeaseServer(uc)
		ctx := authctx.WithClaims(t.Context(), &authctx.Claims{
			Namespaces: []string{"allowed"},
			Role:       "writer",
		})

		_, err := srv.LeaseRevoke(ctx, &etcdserverpb.LeaseRevokeRequest{ID: 7})
		require.NoError(t, err)
		assert.Equal(t, []int64{7}, uc.revoked)
	})
}

func TestLeaseServer_LeaseTimeToLive(t *testing.T) {
	t.Parallel()

	t.Run("reports the remaining and granted ttl", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{lease: liveLease(7, time.Minute, 30*time.Second)})

		resp, err := srv.LeaseTimeToLive(t.Context(), &etcdserverpb.LeaseTimeToLiveRequest{ID: 7})
		require.NoError(t, err)
		assert.Equal(t, int64(7), resp.GetID())
		assert.InDelta(t, 30, resp.GetTTL(), 1)
		assert.Equal(t, int64(60), resp.GetGrantedTTL())
		assert.Empty(t, resp.GetKeys())
	})

	t.Run("returns the attached keys when asked", func(t *testing.T) {
		t.Parallel()

		srv := newLeaseServer(&fakeLeaseUsecase{
			lease: liveLease(7, time.Minute, time.Minute),
			keys:  []domain.KeyRef{{Namespace: "prod", Path: "/lock/leader"}},
		})

		resp, err := srv.LeaseTimeToLive(t.Context(), &etcdserverpb.LeaseTimeToLiveRequest{
			ID:   7,
			Keys: true,
		})
		require.NoError(t, err)
		require.Len(t, resp.GetKeys(), 1)
		assert.Equal(t, "/prod/lock/leader", string(resp.GetKeys()[0]))
	})

	t.Run("an unknown lease reports ttl -1, not an error", func(t *testing.T) {
		t.Parallel()

		// -1 is the wire protocol's "no such lease" for this RPC, distinct from
		// 0, which a live lease can legitimately report.
		srv := newLeaseServer(&fakeLeaseUsecase{ttlErr: domain.ErrLeaseNotFound})

		resp, err := srv.LeaseTimeToLive(t.Context(), &etcdserverpb.LeaseTimeToLiveRequest{ID: 404})
		require.NoError(t, err)
		assert.Equal(t, int64(-1), resp.GetTTL())
		assert.Equal(t, int64(404), resp.GetID())
	})
}

func TestLeaseServer_LeaseLeases(t *testing.T) {
	t.Parallel()

	srv := newLeaseServer(&fakeLeaseUsecase{lease: liveLease(7, time.Minute, time.Minute)})

	resp, err := srv.LeaseLeases(t.Context(), &etcdserverpb.LeaseLeasesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetLeases(), 1)
	assert.Equal(t, int64(7), resp.GetLeases()[0].GetID())
}
