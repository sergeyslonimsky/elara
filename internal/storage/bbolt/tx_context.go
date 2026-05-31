package bbolt

import (
	"context"

	"go.etcd.io/bbolt"
)

type txKey struct{}

// withTx adds a bbolt transaction to the context.
func withTx(ctx context.Context, tx *bbolt.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// txFromCtx retrieves a bbolt transaction from the context.
func txFromCtx(ctx context.Context) (*bbolt.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*bbolt.Tx)

	return tx, ok
}
