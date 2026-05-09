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
	transfer_mock "github.com/sergeyslonimsky/elara/internal/usecase/transfer/mocks"
)

type mocks struct {
	enforcer   *transfer_mock.Mockenforcer
	configs    *transfer_mock.Mockconfigs
	namespaces *transfer_mock.Mocknamespaces
}

func setupService(t *testing.T, ctrl *gomock.Controller) (*transfer.Service, mocks) {
	t.Helper()

	m := mocks{
		enforcer:   transfer_mock.NewMockenforcer(ctrl),
		configs:    transfer_mock.NewMockconfigs(ctrl),
		namespaces: transfer_mock.NewMocknamespaces(ctrl),
	}

	svc := transfer.New(m.enforcer, m.configs, m.namespaces)

	return svc, m
}

func transferTestCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{Email: "test@example.com"})
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
