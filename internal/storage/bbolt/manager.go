package bbolt

import (
	"context"
	"fmt"

	"go.etcd.io/bbolt"

	"github.com/sergeyslonimsky/elara/internal/storage"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

var _ storage.Manager = (*Manager)(nil)

// Manager implements storage.Manager for bbolt as a thin facade over
// pkg/bbolt.DBManager. The underlying ctx-key is shared with the pkg layer,
// so writers opened by either side see the same transaction handle and
// nested WithTx flattens correctly.
type Manager struct {
	pm *pkgbbolt.DBManager
}

// NewManager wraps a *bbolt.DB in a Manager backed by pkg/bbolt.DBManager.
func NewManager(db *bbolt.DB) *Manager {
	return &Manager{pm: pkgbbolt.NewManager(db)}
}

// WithTx delegates to pkg/bbolt.DBManager.WithTx.
func (m *Manager) WithTx(ctx context.Context, callback func(ctx context.Context) error) error {
	if err := m.pm.WithTx(ctx, callback); err != nil {
		return fmt.Errorf("bbolt: with tx: %w", err)
	}

	return nil
}
