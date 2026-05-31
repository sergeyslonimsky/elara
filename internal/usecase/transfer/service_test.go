package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	transfermock "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

type mocks struct {
	pdp        *transfermock.Mockpdp
	configs    *transfermock.Mockconfigs
	namespaces *transfermock.Mocknamespaces
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*transfer.Service, mocks) {
	t.Helper()

	m := mocks{
		pdp:        transfermock.NewMockpdp(ctrl),
		configs:    transfermock.NewMockconfigs(ctrl),
		namespaces: transfermock.NewMocknamespaces(ctrl),
	}

	svc := transfer.New(m.pdp, m.configs, m.namespaces)

	return svc, m
}

func transferTestCtx(ctx context.Context) context.Context {
	return auth2.WithClaims(ctx, &auth2.Claims{Email: "test@example.com"})
}

// readZipEntries reads a ZIP archive and returns a map of filename -> content.
func readZipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	entries := make(map[string][]byte, len(zr.File))

	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)

		content, err := io.ReadAll(rc)

		require.NoError(t, rc.Close())
		require.NoError(t, err)

		entries[f.Name] = content
	}

	return entries
}
