package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/transfer"
	transfermock "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

type mocks struct {
	enforcer   *transfermock.Mockenforcer
	configs    *transfermock.Mockconfigs
	namespaces *transfermock.Mocknamespaces
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*transfer.Service, mocks) {
	t.Helper()

	m := mocks{
		enforcer:   transfermock.NewMockenforcer(ctrl),
		configs:    transfermock.NewMockconfigs(ctrl),
		namespaces: transfermock.NewMocknamespaces(ctrl),
	}

	svc := transfer.New(m.enforcer, m.configs, m.namespaces)

	return svc, m
}

func transferTestCtx(ctx context.Context) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
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
