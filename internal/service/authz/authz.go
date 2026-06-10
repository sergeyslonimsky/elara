package authz

//go:generate mockgen -destination=mocks/authz_mock.go -package=authz_mock -source=authz.go

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
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
	info, err := authctx.AuthInfoFromContext(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	if !a.pdp.Has(
		info.UserID,
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
		user.UserID,
		domain.Permission{Object: object, Action: action, Domain: domainStr},
	) {
		return connect.NewError(connect.CodePermissionDenied, domain.ErrForbidden)
	}

	return nil
}

// RequireNamespace gates the caller on Namespace:<action> for the given
// namespace name (or DomainAll for a global grant). Hides the
// domain.NamespaceResource prefix convention from handlers.
func (a *Authz) RequireNamespace(
	ctx context.Context,
	action domain.Action,
	name string,
) error {
	return a.Require(ctx, domain.ObjectNamespace, action, domain.NamespaceResource(name))
}

// RequireGroup gates the caller on Group:<action> for the given group id
// (or DomainAll for a global grant). Hides the domain.GroupResource prefix
// convention from handlers.
func (a *Authz) RequireGroup(
	ctx context.Context,
	action domain.Action,
	id string,
) error {
	return a.Require(ctx, domain.ObjectGroup, action, domain.GroupResource(id))
}

func (a *Authz) RequireAuthenticated(ctx context.Context) error {
	if _, err := authctx.AuthInfoFromContext(ctx); err != nil {
		return connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	return nil
}
