package token_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
	tokenmock "github.com/sergeyslonimsky/elara/internal/usecase/token/mocks"
)

type mocks struct {
	enforcer *tokenmock.Mockenforcer
	store    *tokenmock.Mockstore
}

func setupService(t *testing.T) (*token.Service, mocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := mocks{
		enforcer: tokenmock.NewMockenforcer(ctrl),
		store:    tokenmock.NewMockstore(ctrl),
	}

	svc := token.New(m.enforcer, m.store)

	return svc, m
}

func ctxWithClaims(ctx context.Context, email string) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: email})
}
