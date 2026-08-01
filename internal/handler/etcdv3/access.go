package etcdv3

import (
	"slices"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// namespaceAllowed reports whether claims grants action-level access to namespace.
// Shared by KVServer and WatchServer so both RPC families enforce the same
// service-token scoping instead of maintaining independent copies.
//
// nil claims means auth is disabled — always allowed.
func namespaceAllowed(claims *authctx.Claims, namespace string, action domain.Action) bool {
	if claims == nil {
		return true
	}

	allowedNS := false

	for _, ns := range claims.Namespaces {
		if ns == namespace || ns == "*" {
			allowedNS = true

			break
		}
	}

	if !allowedNS {
		return false
	}

	if action == domain.ActionWrite && domain.Role(claims.Role) != domain.RoleWriter {
		return false
	}

	return true
}

// namespaceAllScoped reports whether claims grants access to every namespace
// (a wildcard service token). Required for scanAll watches, which cannot be
// bounded to a per-request namespace check.
//
// nil claims means auth is disabled — always allowed.
func namespaceAllScoped(claims *authctx.Claims) bool {
	if claims == nil {
		return true
	}

	return slices.Contains(claims.Namespaces, "*")
}

// checkWatchAccess validates that claims permits creating a watch over the
// given key range. A scanAll watch (unbounded, RangeEnd == "\x00") isn't
// tied to any namespace, so it requires an unrestricted (wildcard) token.
// Bounded and cross-namespace ranges are checked against their namespace
// boundaries, mirroring KVServer.checkRangeAccess for Range.
func checkWatchAccess(claims *authctx.Claims, scanAll bool, startNS, endNS string) error {
	if scanAll {
		if !namespaceAllScoped(claims) {
			return status.Errorf(codes.PermissionDenied, "scanAll watch requires an unrestricted token")
		}

		return nil
	}

	if !namespaceAllowed(claims, startNS, domain.ActionRead) {
		return status.Errorf(codes.PermissionDenied, "permission denied for namespace %q", startNS)
	}

	if endNS != "" && endNS != startNS {
		if !namespaceAllowed(claims, endNS, domain.ActionRead) {
			return status.Errorf(codes.PermissionDenied, "permission denied for namespace %q", endNS)
		}
	}

	return nil
}
