package authz

//go:generate mockgen -destination=mocks/authz_mock.go -package=authz_mock -source=authz.go

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type pdp interface {
	Has(principal string, perm domain.Permission) bool
}

type Authz struct {
	pdp pdp
}

func NewAuthz(p pdp) *Authz {
	return &Authz{pdp: p}
}

func (a *Authz) Require(
	ctx context.Context,
	object domain.Object,
	action domain.Action,
	domainStr string,
) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	if !a.pdp.Has(
		claims.Email,
		domain.Permission{Object: object, Action: action, Domain: domainStr},
	) {
		return connect.NewError(connect.CodePermissionDenied, domain.ErrForbidden)
	}

	return nil
}

func (a *Authz) RequireUser(
	user domain.AuthInfo,
	object domain.Object,
	action domain.Action,
	domainStr string,
) error {
	if !a.pdp.Has(
		user.Email,
		domain.Permission{Object: object, Action: action, Domain: domainStr},
	) {
		return connect.NewError(connect.CodePermissionDenied, domain.ErrForbidden)
	}

	return nil
}

func (a *Authz) RequireAuthenticated(ctx context.Context) error {
	_, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	return nil
}
