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
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	transferuc "github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	mock_transfer "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

func TestTransferHandler_ExportNamespace(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	enforcer := mock_transfer.NewMockexportNSEnforcer(ctrl)
	configs := mock_transfer.NewMockexportNSConfigLister(ctrl)
	namespaces := mock_transfer.NewMockexportNSChecker(ctrl)

	exportUC := transferuc.NewExportNamespaceUseCase(enforcer, configs, namespaces)
	h := &TransferHandler{exportNamespace: exportUC}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})

	enforcer.EXPECT().Enforce("admin@example.com", "*", "transfer", "write").Return(true, nil)
	namespaces.EXPECT().Get(gomock.Any(), "prod").Return(&domain.Namespace{Name: "prod"}, nil)
	configs.EXPECT().ListAllByNamespace(gomock.Any(), "prod").Return([]*domain.Config{}, nil)

	req := connect.NewRequest(&transferv1.ExportNamespaceRequest{
		Namespace: "prod",
		Zip:       false,
		Encoding:  transferv1.BundleEncoding_BUNDLE_ENCODING_JSON,
	})

	resp, err := h.ExportNamespace(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.GetData())
	assert.Equal(t, "application/json", resp.Msg.GetContentType())
}
