package interceptor

import (
	"context"
	"fmt"
	"maps"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

//go:generate mockgen -destination=mocks/mock_rbac.go -package=mock_interceptor -source=rbac.go

// rbacEnforcer is the minimal Casbin subset the RBAC interceptor needs.
type rbacEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

// Permission names the resource and action required to call a global RPC.
// The domain is always domain.DomainAll for interceptor-enforced procedures.
type Permission struct {
	Object string
	Action string
}

// RBACInterceptor enforces a per-procedure RBAC table. Every procedure is
// classified as either:
//   - globally enforced: registry holds a Permission; interceptor calls
//     Enforce(claims.Email, DomainAll, perm.Object, perm.Action).
//   - auth-only: the upstream AuthInterceptor has already established claims;
//     the usecase enforces any per-namespace check.
//
// A procedure in neither set is rejected with CodePermissionDenied at
// runtime. This is intentional: a new RPC mounted without a corresponding
// classification fails closed instead of silently bypassing RBAC.
type RBACInterceptor struct {
	enforcer rbacEnforcer
	registry map[string]Permission
	authOnly map[string]struct{}
}

var _ connect.Interceptor = (*RBACInterceptor)(nil)

// NewRBACInterceptor returns an interceptor that consults its own registry +
// auth-only whitelist on every call. The maps are copied internally.
func NewRBACInterceptor(
	enforcer rbacEnforcer,
	registry map[string]Permission,
	authOnly map[string]struct{},
) *RBACInterceptor {
	r := make(map[string]Permission, len(registry))
	maps.Copy(r, registry)

	w := make(map[string]struct{}, len(authOnly))
	for k := range authOnly {
		w[k] = struct{}{}
	}

	// Catching the overlap at construction time is cheaper than debugging
	// "permission appears to be ignored" later.
	for k := range r {
		if _, ok := w[k]; ok {
			panic(fmt.Sprintf("rbac: procedure %q listed both in registry and auth-only whitelist", k))
		}
	}

	return &RBACInterceptor{enforcer: enforcer, registry: r, authOnly: w}
}

func (i *RBACInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.Authorize(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *RBACInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RBACInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.Authorize(ctx, conn.Spec().Procedure); err != nil {
			return err
		}

		return next(ctx, conn)
	}
}

// Authorize runs the RBAC decision for the given procedure. It is exported so
// tests can exercise it directly; WrapUnary and WrapStreamingHandler are thin
// wrappers around this.
func (i *RBACInterceptor) Authorize(ctx context.Context, procedure string) error {
	if _, ok := i.authOnly[procedure]; ok {
		return nil
	}

	perm, registered := i.registry[procedure]
	if !registered {
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("%w: no RBAC policy for %q", domain.ErrForbidden, procedure))
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized)
	}

	allowed, err := i.enforcer.Enforce(claims.Email, domain.DomainAll, perm.Object, perm.Action)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("rbac enforce: %w", err))
	}

	if !allowed {
		return connect.NewError(connect.CodePermissionDenied, domain.ErrForbidden)
	}

	return nil
}
