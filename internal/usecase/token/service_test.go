package token_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
	tokenmock "github.com/sergeyslonimsky/elara/internal/usecase/token/mocks"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

type mocks struct {
	txm   *storage_mock.MockManager
	pdp   *tokenmock.Mockpdp
	store *tokenmock.Mockstore
}

func setupService(t *testing.T) (*token.Service, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := mocks{
		txm:   storage_mock.NewMockManager(ctrl),
		pdp:   tokenmock.NewMockpdp(ctrl),
		store: tokenmock.NewMockstore(ctrl),
	}

	// Transactions are a passthrough here: the number of WithTx calls
	// follows the retry count, which the store expectations already pin.
	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	svc := token.New(m.txm, m.pdp, m.store)

	return svc, m
}

func authUser(email string) domain.AuthInfo {
	return domain.AuthInfo{
		UserID: testUserID,
		Email:  email,
	}
}
