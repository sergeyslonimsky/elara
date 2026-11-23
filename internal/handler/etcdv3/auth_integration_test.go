//go:build integration

package etcdv3_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

// seedProdToken stores a writer token scoped to "prod" and returns its
// Username and raw secret.
func seedProdToken(t *testing.T, s *itest.Suite) (string, string) {
	t.Helper()

	const raw = "elara_integration-secret"

	tok := &domain.Token{
		ID:         uuid.New().String(),
		IssuedBy:   "alice@example.com",
		Name:       "ci",
		Username:   "ci-abcdef",
		TokenHash:  token.HashRawToken(raw),
		Namespaces: []string{"prod"},
		Role:       domain.RoleWriter,
		CreatedAt:  time.Now().UTC(),
	}
	require.NoError(t, tok.Validate())
	require.NoError(t, s.Adapters.AuthTokens.Create(t.Context(), tok))

	return tok.Username, raw
}

func newAuthClient(t *testing.T, cfg clientv3.Config) (*clientv3.Client, error) {
	t.Helper()

	cfg.DialTimeout = 5 * time.Second

	cli, err := clientv3.New(cfg)
	if cli != nil {
		t.Cleanup(func() { _ = cli.Close() })
	}

	return cli, err
}

func TestIntegration_EtcdAuth_UsernamePasswordLogin(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	addr := s.StartEtcdWithAuth(t)
	username, raw := seedProdToken(t, s)
	ctx := t.Context()

	cli, err := newAuthClient(t, clientv3.Config{
		Endpoints: []string{addr},
		Username:  username,
		Password:  raw,
	})
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/login.json", `{"ok":true}`)
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/login.json")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.JSONEq(t, `{"ok":true}`, string(resp.Kvs[0].GetValue()))

	// Scope still applies after login: the token only covers "prod".
	_, err = cli.Get(ctx, "/staging/api.json")
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestIntegration_EtcdAuth_RawTokenConfig(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	addr := s.StartEtcdWithAuth(t)
	_, raw := seedProdToken(t, s)
	ctx := t.Context()

	cli, err := newAuthClient(t, clientv3.Config{
		Endpoints: []string{addr},
		Token:     raw,
	})
	require.NoError(t, err)

	_, err = cli.Put(ctx, "/prod/raw.json", `{"raw":true}`)
	require.NoError(t, err)

	resp, err := cli.Get(ctx, "/prod/raw.json")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	assert.JSONEq(t, `{"raw":true}`, string(resp.Kvs[0].GetValue()))
}

func TestIntegration_EtcdAuth_LoginFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password func(raw string) string
		username func(username string) string
	}{
		{
			name:     "wrong password",
			password: func(string) string { return "elara_wrong-secret" },
			username: func(u string) string { return u },
		},
		{
			name:     "unknown username",
			password: func(raw string) string { return raw },
			username: func(string) string { return "nobody-aaaaaa" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := itest.New(t)
			addr := s.StartEtcdWithAuth(t)
			username, raw := seedProdToken(t, s)

			cli, err := newAuthClient(t, clientv3.Config{
				Endpoints: []string{addr},
				Username:  tt.username(username),
				Password:  tt.password(raw),
			})
			if err == nil {
				// Depending on the clientv3 version, login happens on the
				// first RPC rather than in New.
				_, err = cli.Get(t.Context(), "/prod/api.json")
			}

			require.ErrorIs(t, err, rpctypes.ErrAuthFailed)
		})
	}
}

func TestIntegration_EtcdAuth_NoCredentialsRejected(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	addr := s.StartEtcdWithAuth(t)

	cli, err := newAuthClient(t, clientv3.Config{Endpoints: []string{addr}})
	require.NoError(t, err)

	_, err = cli.Get(t.Context(), "/prod/api.json")
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
