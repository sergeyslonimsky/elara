package bbolt

import (
	"context"
	"errors"
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
		return fmt.Errorf("bbolt: with tx: %w", toStorageErr(err))
	}

	return nil
}

// WithReadTx delegates to pkg/bbolt.DBManager.WithReadTx.
func (m *Manager) WithReadTx(ctx context.Context, callback func(ctx context.Context) error) error {
	if err := m.pm.WithReadTx(ctx, callback); err != nil {
		return fmt.Errorf("bbolt: with read tx: %w", toStorageErr(err))
	}

	return nil
}

// toStorageErr adds the backend-agnostic sentinel to pkg/bbolt's read-only
// refusal, so callers can match storage.ErrReadOnlyTx whether the refused
// write came from a usecase's WithTx or from a repository deeper down.
func toStorageErr(err error) error {
	if errors.Is(err, pkgbbolt.ErrTxReadOnly) && !errors.Is(err, storage.ErrReadOnlyTx) {
		return fmt.Errorf("%w: %w", storage.ErrReadOnlyTx, err)
	}

	return err
}
