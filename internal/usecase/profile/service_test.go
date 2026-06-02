package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	storage_mock "github.com/sergeyslonimsky/elara/internal/storage/mocks"
	"github.com/sergeyslonimsky/elara/internal/usecase/profile"
	profilemock "github.com/sergeyslonimsky/elara/internal/usecase/profile/mocks"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

type mocks struct {
	txm      *storage_mock.MockManager
	pdp      *profilemock.Mockpdp
	users    *profilemock.MockuserGetter
	pass     *profilemock.MockpassWriter
	sessions *profilemock.MocksessionsService
}

func setupService(ctrl *gomock.Controller) (*profile.Service, mocks) {
	m := mocks{
		txm:      storage_mock.NewMockManager(ctrl),
		pdp:      profilemock.NewMockpdp(ctrl),
		users:    profilemock.NewMockuserGetter(ctrl),
		pass:     profilemock.NewMockpassWriter(ctrl),
		sessions: profilemock.NewMocksessionsService(ctrl),
	}

	svc := profile.New(m.txm, m.pdp, m.users, m.pass, m.sessions)

	return svc, m
}

func TestService_Logout(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc, m := setupService(ctrl)

	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)
	m.sessions.EXPECT().Revoke(gomock.Any(), "s1", testUserID, gomock.Any(), gomock.Any()).Return(nil)

	err := svc.Logout(t.Context(), "s1", testUserID)

	require.NoError(t, err)
}
