package namespace

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	namespacev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	mock_namespace "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

// -----------------------------------------------------------------------------
// GetNamespace
// -----------------------------------------------------------------------------

func TestNamespaceHandler_GetNamespace_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsGetter(ctrl)
	counter := mock_namespace.NewMockgetConfigCounter(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)
	namespaces.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	counter.EXPECT().CountConfigs(gomock.Any(), "prod").Return(5, nil)

	h := &Handler{get: nsuc.NewGetUseCase(enforcer, namespaces, counter)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	resp, err := h.GetNamespace(ctx, connect.NewRequest(&namespacev1.GetNamespaceRequest{Name: "prod"}))
	require.NoError(t, err)
	assert.Equal(t, "prod", resp.Msg.GetNamespace().GetName())
	assert.Equal(t, int32(5), resp.Msg.GetNamespace().GetConfigCount())
}

func TestNamespaceHandler_GetNamespace_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &Handler{get: nsuc.NewGetUseCase(nil, nil, nil)}

	_, err := h.GetNamespace(context.Background(), connect.NewRequest(&namespacev1.GetNamespaceRequest{Name: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestNamespaceHandler_GetNamespace_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsGetter(ctrl)

	enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)
	namespaces.EXPECT().Get(gomock.Any(), "prod").Return(nil, domain.ErrNotFound)

	h := &Handler{get: nsuc.NewGetUseCase(enforcer, namespaces, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	_, err := h.GetNamespace(ctx, connect.NewRequest(&namespacev1.GetNamespaceRequest{Name: "prod"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// -----------------------------------------------------------------------------
// CreateNamespace
// -----------------------------------------------------------------------------

func TestNamespaceHandler_CreateNamespace_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsCreator(ctrl)
	getter := mock_namespace.NewMocknsGetterForCreate(ctrl)

	enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
	namespaces.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "new-ns").Return(&domain.Namespace{Name: "new-ns"}, nil)

	h := &Handler{create: nsuc.NewCreateUseCase(enforcer, namespaces, getter)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	resp, err := h.CreateNamespace(ctx, connect.NewRequest(&namespacev1.CreateNamespaceRequest{Name: "new-ns"}))
	require.NoError(t, err)
	assert.Equal(t, "new-ns", resp.Msg.GetNamespace().GetName())
}

func TestNamespaceHandler_CreateNamespace_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &Handler{create: nsuc.NewCreateUseCase(nil, nil, nil)}

	_, err := h.CreateNamespace(
		context.Background(),
		connect.NewRequest(&namespacev1.CreateNamespaceRequest{Name: "new-ns"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestNamespaceHandler_CreateNamespace_AlreadyExists(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsCreator(ctrl)

	enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
	namespaces.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrAlreadyExists)

	h := &Handler{create: nsuc.NewCreateUseCase(enforcer, namespaces, nil)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	_, err := h.CreateNamespace(ctx, connect.NewRequest(&namespacev1.CreateNamespaceRequest{Name: "existing"}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}
