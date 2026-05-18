package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/usecase/profile"
	profilemock "github.com/sergeyslonimsky/elara/internal/usecase/profile/mocks"
)

type mocks struct {
	enforcer *profilemock.Mockenforcer
	ns       *profilemock.MocknsLister
	users    *profilemock.MockuserGetter
	pass     *profilemock.MockpassWriter
	session  *profilemock.MocksessionCreator
}

func setupService(ctrl *gomock.Controller) (*profile.Service, mocks) {
	m := mocks{
		enforcer: profilemock.NewMockenforcer(ctrl),
		ns:       profilemock.NewMocknsLister(ctrl),
		users:    profilemock.NewMockuserGetter(ctrl),
		pass:     profilemock.NewMockpassWriter(ctrl),
		session:  profilemock.NewMocksessionCreator(ctrl),
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
