package token_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
	tokenmock "github.com/sergeyslonimsky/elara/internal/usecase/token/mocks"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

type mocks struct {
	pdp   *tokenmock.Mockpdp
	store *tokenmock.Mockstore
}

func setupService(t *testing.T) (*token.Service, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := mocks{
		pdp:   tokenmock.NewMockpdp(ctrl),
		store: tokenmock.NewMockstore(ctrl),
	}

	svc := token.New(m.pdp, m.store)

	return svc, m
}

func authUser(email string) domain.AuthInfo {
	return domain.AuthInfo{
		UserID: testUserID,
		Email:  email,
	}
}
