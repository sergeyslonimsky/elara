package policy

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=policy_mock -source=service.go

// groupFinder lets us reject AssignRole/RevokeRole for groups that don't
// exist. Without this check it would be possible to grant a role to a
// typo'd group name — the g-rule would sit in Casbin orphaned, with no UI
// surface to discover or revoke it.
type groupFinder interface {
	FindByName(ctx context.Context, name string) (*domain.Group, error)
}

// Service grants and revokes group→role assignments through the
// authorization administration point. Casbin specifics (subject string
// shape, in-tx rule writes) live entirely behind PAP.
type Service struct {
	pap    *authz.PAP
	groups groupFinder
}

func New(pap *authz.PAP, groups groupFinder) *Service {
	return &Service{pap: pap, groups: groups}
}
