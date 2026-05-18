package config_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/config"
	configmock "github.com/sergeyslonimsky/elara/internal/handler/v2/config/mocks"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
)

func TestSchemaHandler_AttachSchema_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := configmock.NewMockschemaUsecase(ctrl)

	uc.EXPECT().
		Attach(gomock.Any(), schemauc.AttachInput{
			Namespace:   "prod",
			PathPattern: "/*.json",
			JSONSchema:  "{}",
		}).
		Return(&domain.SchemaAttachment{
			Namespace:   "prod",
			PathPattern: "/*.json",
			JSONSchema:  "{}",
		}, nil)

	h := config.NewSchemaHandler(uc)

	resp, err := h.AttachSchema(t.Context(), connect.NewRequest(&configv1.AttachSchemaRequest{
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

	ctrl := gomock.NewController(t)
	uc := configmock.NewMockschemaUsecase(ctrl)
	uc.EXPECT().
		Attach(gomock.Any(), gomock.Any()).
		Return(nil, domain.ErrUnauthorized)

	h := config.NewSchemaHandler(uc)

	_, err := h.AttachSchema(t.Context(), connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:  "prod",
		JsonSchema: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSchemaHandler_AttachSchema_NamespaceNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := configmock.NewMockschemaUsecase(ctrl)
	uc.EXPECT().
		Attach(gomock.Any(), gomock.Any()).
		Return(nil, domain.ErrNotFound)

	h := config.NewSchemaHandler(uc)

	_, err := h.AttachSchema(t.Context(), connect.NewRequest(&configv1.AttachSchemaRequest{
		Namespace:  "prod",
		JsonSchema: "{}",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
