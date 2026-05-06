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
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
	mock_schema "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestSchemaHandler_AttachSchema_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_schema.NewMockattachEnforcer(ctrl)
	store := mock_schema.NewMockschemaAttacher(ctrl)
	nsChecker := mock_schema.NewMockattachNSChecker(ctrl)

	enforcer.EXPECT().Enforce("admin@example.com", "prod", "schema", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	store.EXPECT().Attach(gomock.Any(), gomock.Any()).Return(nil)

	h := &SchemaHandler{attach: schemauc.NewAttachUseCase(enforcer, store, nsChecker)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	resp, err := h.AttachSchema(ctx, connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:   "prod",
		PathPattern: "/*.json",
		JsonSchema:  "{}",
	}))
	require.NoError(t, err)
	assert.Equal(t, "prod", resp.Msg.GetSchema().GetNamespace())
	assert.Equal(t, "/*.json", resp.Msg.GetSchema().GetPathPattern())
}

func TestSchemaHandler_AttachSchema_Unauthorized(t *testing.T) {
	t.Parallel()

	h := &SchemaHandler{attach: schemauc.NewAttachUseCase(nil, nil, nil)}

	_, err := h.AttachSchema(context.Background(), connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:  "prod",
		JsonSchema: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSchemaHandler_AttachSchema_NamespaceNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_schema.NewMockattachEnforcer(ctrl)
	nsChecker := mock_schema.NewMockattachNSChecker(ctrl)

	enforcer.EXPECT().Enforce("admin@example.com", "prod", "schema", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(nil, domain.ErrNotFound)

	h := &SchemaHandler{attach: schemauc.NewAttachUseCase(enforcer, nil, nsChecker)}
	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	_, err := h.AttachSchema(ctx, connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:  "prod",
		JsonSchema: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
