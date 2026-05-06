package v2

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

func TestNamespaceHandler_GetNamespace(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsGetter(ctrl)
	counter := mock_namespace.NewMockgetConfigCounter(ctrl)

	getUC := nsuc.NewGetUseCase(enforcer, namespaces, counter)
	h := &NamespaceHandler{get: getUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})

	enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)
	namespaces.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	counter.EXPECT().CountConfigs(gomock.Any(), "prod").Return(5, nil)

	req := connect.NewRequest(&namespacev1.GetNamespaceRequest{Name: "prod"})

	resp, err := h.GetNamespace(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "prod", resp.Msg.GetNamespace().GetName())
	assert.Equal(t, int32(5), resp.Msg.GetNamespace().GetConfigCount())
}

func TestNamespaceHandler_CreateNamespace(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
	namespaces := mock_namespace.NewMocknsCreator(ctrl)
	getter := mock_namespace.NewMocknsGetterForCreate(ctrl)

	createUC := nsuc.NewCreateUseCase(enforcer, namespaces, getter)
	h := &NamespaceHandler{create: createUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
	namespaces.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	getter.EXPECT().Get(gomock.Any(), "new-ns").Return(&domain.Namespace{Name: "new-ns"}, nil)

	req := connect.NewRequest(&namespacev1.CreateNamespaceRequest{Name: "new-ns"})

	resp, err := h.CreateNamespace(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "new-ns", resp.Msg.GetNamespace().GetName())
}
