package etcdv3

//go:generate mockgen -destination=mocks/auth_server_mock.go -package=etcdv3_mock -source=auth_server.go

import (
	"context"
	"errors"
	"log/slog"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type tokenAuthenticator interface {
	Authenticate(ctx context.Context, username, password string) (*domain.Token, error)
}

// AuthServer implements the one piece of etcd's Auth service a client needs
// to log in: Authenticate. etcd's own user/role management (UserAdd,
// RoleGrantPermission, …) stays Unimplemented — Elara's RBAC is managed
// through the UI and ConnectRPC API.
//
// The user name is a token's Username, the password is the raw token. The
// returned auth token is that same raw token: the client already holds it,
// and the interceptor re-validates it on every request, so a revoked token
// stops working immediately without any server-side session state.
type AuthServer struct {
	etcdserverpb.UnimplementedAuthServer

	tokens  tokenAuthenticator
	enabled bool
}

// NewAuthServer returns an AuthServer. With enabled=false Authenticate
// answers ErrGRPCAuthNotEnabled, which clientv3 treats as "proceed without a
// token" — a client configured with credentials still connects.
func NewAuthServer(tokens tokenAuthenticator, enabled bool) *AuthServer {
	return &AuthServer{tokens: tokens, enabled: enabled}
}

//nolint:wrapcheck // gRPC status errors are terminal; wrapping corrupts the status code
func (s *AuthServer) Authenticate(
	ctx context.Context,
	req *etcdserverpb.AuthenticateRequest,
) (*etcdserverpb.AuthenticateResponse, error) {
	if !s.enabled {
		return nil, rpctypes.ErrGRPCAuthNotEnabled
	}

	if req.GetName() == "" {
		return nil, rpctypes.ErrGRPCUserEmpty
	}

	if _, err := s.tokens.Authenticate(ctx, req.GetName(), req.GetPassword()); err != nil {
		if errors.Is(err, domain.ErrInvalidToken) {
			slog.InfoContext(ctx, "etcd authenticate rejected", "user", req.GetName(), "reason", err)

			return nil, rpctypes.ErrGRPCAuthFailed
		}

		return nil, status.Error(codes.Internal, "authenticate failed")
	}

	return &etcdserverpb.AuthenticateResponse{
		Header: newHeader(0),
		Token:  req.GetPassword(),
	}, nil
}
