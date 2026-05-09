package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/profile"
	profile_mock "github.com/sergeyslonimsky/elara/internal/usecase/profile/mocks"
)

type mocks struct {
	enforcer *profile_mock.Mockenforcer
	ns       *profile_mock.MocknsLister
	users    *profile_mock.MockuserGetter
	pass     *profile_mock.MockpassWriter
	session  *profile_mock.MocksessionCreator
}

func setupService(ctrl *gomock.Controller) (*profile.Service, mocks) {
	m := mocks{
		enforcer: profile_mock.NewMockenforcer(ctrl),
		ns:       profile_mock.NewMocknsLister(ctrl),
		users:    profile_mock.NewMockuserGetter(ctrl),
		pass:     profile_mock.NewMockpassWriter(ctrl),
		session:  profile_mock.NewMocksessionCreator(ctrl),
	}

	svc := profile.New(m.enforcer, m.ns, m.users, m.pass, m.session)

	return svc, m
}

func TestService_Logout(t *testing.T) {
	t.Parallel()

	svc := profile.New(nil, nil, nil, nil, nil)
	err := svc.Logout(t.Context())

	require.NoError(t, err)
}
