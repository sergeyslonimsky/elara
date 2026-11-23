package etcdv3_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/etcdv3"
	etcdv3_mock "github.com/sergeyslonimsky/elara/internal/handler/etcdv3/mocks"
)

func TestAuthServer_Authenticate(t *testing.T) {
	t.Parallel()

	const (
		username = "ci-abcdef"
		password = "elara_secret"
	)

	tests := []struct {
		name     string
		req      *etcdserverpb.AuthenticateRequest
		mockFunc func(*gomock.Controller) *etcdv3.AuthServer
		errIs    error
		wantCode codes.Code
		want     string
	}{
		{
			name: "success returns the raw token as auth token",
			req:  &etcdserverpb.AuthenticateRequest{Name: username, Password: password},
			mockFunc: func(ctrl *gomock.Controller) *etcdv3.AuthServer {
				tokens := etcdv3_mock.NewMocktokenAuthenticator(ctrl)
				tokens.EXPECT().
					Authenticate(gomock.Any(), username, password).
					Return(&domain.Token{ID: "t1", Username: username}, nil)

				return etcdv3.NewAuthServer(tokens, true)
			},
			want: password,
		},
		{
			name: "auth disabled",
			req:  &etcdserverpb.AuthenticateRequest{Name: username, Password: password},
			mockFunc: func(_ *gomock.Controller) *etcdv3.AuthServer {
				return etcdv3.NewAuthServer(nil, false)
			},
			errIs: rpctypes.ErrGRPCAuthNotEnabled,
		},
		{
			name: "empty user name",
			req:  &etcdserverpb.AuthenticateRequest{Password: password},
			mockFunc: func(_ *gomock.Controller) *etcdv3.AuthServer {
				return etcdv3.NewAuthServer(nil, true)
			},
			errIs: rpctypes.ErrGRPCUserEmpty,
		},
		{
			name: "invalid credentials map to auth failed",
			req:  &etcdserverpb.AuthenticateRequest{Name: username, Password: "elara_wrong"},
			mockFunc: func(ctrl *gomock.Controller) *etcdv3.AuthServer {
				tokens := etcdv3_mock.NewMocktokenAuthenticator(ctrl)
				tokens.EXPECT().
					Authenticate(gomock.Any(), username, "elara_wrong").
					Return(nil, fmt.Errorf("token t1: secret mismatch: %w", domain.ErrInvalidToken))

				return etcdv3.NewAuthServer(tokens, true)
			},
			errIs: rpctypes.ErrGRPCAuthFailed,
		},
		{
			name: "unexpected usecase error maps to internal",
			req:  &etcdserverpb.AuthenticateRequest{Name: username, Password: password},
			mockFunc: func(ctrl *gomock.Controller) *etcdv3.AuthServer {
				tokens := etcdv3_mock.NewMocktokenAuthenticator(ctrl)
				tokens.EXPECT().
					Authenticate(gomock.Any(), username, password).
					Return(nil, errors.New("db down"))

				return etcdv3.NewAuthServer(tokens, true)
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, err := sut.Authenticate(t.Context(), tt.req)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, status.Code(err))
				assert.NotContains(t, err.Error(), "db down", "internal detail must not leak")

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.GetToken())
			assert.NotNil(t, got.GetHeader())
		})
	}
}
