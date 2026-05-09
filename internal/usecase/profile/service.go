package profile

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=profile_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	nsLister interface {
		List(ctx context.Context) ([]*domain.Namespace, error)
	}

	userGetter interface {
		Get(ctx context.Context, email string) (*domain.User, error)
	}

	passWriter interface {
		SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
	}

	sessionCreator interface {
		Create(user *domain.User) (string, error)
	}
)

type Service struct {
	enforcer enforcer
	ns       nsLister
	users    userGetter
	pass     passWriter
	session  sessionCreator
}

func New(
	enforcer enforcer,
	ns nsLister,
	users userGetter,
	pass passWriter,
	session sessionCreator,
) *Service {
	return &Service{
		enforcer: enforcer,
		ns:       ns,
		users:    users,
		pass:     pass,
		session:  session,
	}
}

// Logout is a no-op; cookie clearing is performed by the handler.
func (s *Service) Logout(_ context.Context) error {
	return nil
}
