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

func TestSchemaHandler_AttachSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_schema.NewMockattachEnforcer(ctrl)
	store := mock_schema.NewMockschemaAttacher(ctrl)
	nsChecker := mock_schema.NewMockattachNSChecker(ctrl)

	attachUC := schemauc.NewAttachUseCase(enforcer, store, nsChecker)
	h := &SchemaHandler{attach: attachUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	enforcer.EXPECT().Enforce("admin@example.com", "prod", "schema", "write").Return(true, nil)
	nsChecker.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	store.EXPECT().Attach(gomock.Any(), gomock.Any()).Return(nil)

	req := connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:   "prod",
		PathPattern: "/*.json",
		JsonSchema:  "{}",
	})

	resp, err := h.AttachSchema(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "prod", resp.Msg.GetSchema().GetNamespace())
	assert.Equal(t, "/*.json", resp.Msg.GetSchema().GetPathPattern())
}
