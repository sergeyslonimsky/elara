package token_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
	tokenmock "github.com/sergeyslonimsky/elara/internal/usecase/token/mocks"
)

// txCtxKey marks the context a WithTx callback receives, so the store
// expectations can prove they ran inside the transaction — not merely that
// some WithTx call happened.
type txCtxKey struct{}

// inTx matches a context that came out of the fake transaction.
func inTx() gomock.Matcher {
	return gomock.Cond(func(x any) bool {
		ctx, ok := x.(context.Context)

		return ok && ctx.Value(txCtxKey{}) != nil
	})
}

// notInTx matches a context that did not come out of the fake transaction.
func notInTx() gomock.Matcher {
	return gomock.Cond(func(x any) bool {
		ctx, ok := x.(context.Context)

		return ok && ctx.Value(txCtxKey{}) == nil
	})
}

// setupStrictTx builds the service with a WithTx that must be called an exact
// number of times and hands its callback a distinct, marked context.
func setupStrictTx(t *testing.T, times int) (*token.Service, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := mocks{
		txm:   storage_mock.NewMockManager(ctrl),
		pdp:   tokenmock.NewMockpdp(ctrl),
		store: tokenmock.NewMockstore(ctrl),
	}

	m.txm.EXPECT().WithTx(notInTx(), gomock.Any()).Times(times).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(context.WithValue(ctx, txCtxKey{}, true))
		},
	)

	return token.New(m.txm, m.pdp, m.store), m
}

func TestService_TransactionBoundary(t *testing.T) {
	t.Parallel()

	createInput := token.CreateInput{
		Name:       "ci",
		Namespaces: []string{"ns1"},
		Role:       domain.RoleWriter,
	}

	tests := []struct {
		name    string
		txCalls int
		setup   func(m mocks)
		run     func(t *testing.T, svc *token.Service) error
	}{
		{
			name:    "create writes inside one transaction",
			txCalls: 1,
			setup: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionWrite).Return(true)
				m.store.EXPECT().Create(inTx(), gomock.Any()).Return(nil)
			},
			run: func(t *testing.T, svc *token.Service) error {
				t.Helper()

				_, _, err := svc.Create(t.Context(), authUser("alice@example.com"), createInput)

				return err
			},
		},
		{
			name:    "each create retry gets its own transaction",
			txCalls: 2,
			setup: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionWrite).Return(true)
				gomock.InOrder(
					m.store.EXPECT().Create(inTx(), gomock.Any()).Return(domain.ErrTokenUsernameTaken),
					m.store.EXPECT().Create(inTx(), gomock.Any()).Return(nil),
				)
			},
			run: func(t *testing.T, svc *token.Service) error {
				t.Helper()

				_, _, err := svc.Create(t.Context(), authUser("alice@example.com"), createInput)

				return err
			},
		},
		{
			name:    "backfill assigns each username inside a transaction",
			txCalls: 2,
			setup: func(m mocks) {
				m.store.EXPECT().ListAll(notInTx()).Return([]*domain.Token{
					{ID: "t1", Name: "ci"},
					{ID: "t2", Name: "deploy"},
					{ID: "t3", Name: "done", Username: "done-aaaaaa"},
				}, nil)
				m.store.EXPECT().SetUsername(inTx(), "t1", gomock.Any()).Return(nil)
				m.store.EXPECT().SetUsername(inTx(), "t2", gomock.Any()).Return(nil)
			},
			run: func(t *testing.T, svc *token.Service) error {
				t.Helper()

				_, err := svc.EnsureUsernames(t.Context())

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupStrictTx(t, tt.txCalls)
			tt.setup(m)

			require.NoError(t, tt.run(t, svc))
		})
	}
}
