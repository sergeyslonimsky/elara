package profile

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=profile_mock -source=service.go

type (
	pdp interface {
		ListPermissions(principal string) ([]domain.Permission, error)
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
	pdp     pdp
	users   userGetter
	pass    passWriter
	session sessionCreator
}

func New(
	pdp pdp,
	users userGetter,
	pass passWriter,
	session sessionCreator,
) *Service {
	return &Service{
		pdp:     pdp,
		users:   users,
		pass:    pass,
		session: session,
	}
}

// Logout is a no-op; cookie clearing is performed by the handler.
func (s *Service) Logout(_ context.Context) error {
	return nil
}
