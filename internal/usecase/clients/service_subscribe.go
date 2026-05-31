package clients

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// SubscribeChanges returns a channel of client-change events filtered to
// clients the caller is allowed to see (see scopeChecker for rules).
// Admins receive the unfiltered upstream channel directly.
func (s *Service) SubscribeChanges(
	ctx context.Context,
) (<-chan domain.ClientChange, func(), error) {
	claims, ok := authctx.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	upstream, cancel := s.active.Subscribe()
	scope := newScopeChecker(s.pdp, claims.Email)

	if scope.admin {
		return upstream, cancel, nil
	}

	out := make(chan domain.ClientChange, cap(upstream))

	go func() {
		defer close(out)

		for change := range upstream {
			if change.Client == nil || !scope.visible(change.Client) {
				continue
			}

			select {
			case out <- change:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, cancel, nil
}

// SubscribeClient returns a per-client change channel. If the caller cannot
// see the client (no overlapping watch namespace), the response is
// ErrNotFound — same wording as Get to avoid leaking existence.
func (s *Service) SubscribeClient(
	ctx context.Context,
	connID string,
) (<-chan domain.ClientChange, func(), error) {
	claims, ok := authctx.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	scope := newScopeChecker(s.pdp, claims.Email)

	if !scope.admin {
		c := s.active.Get(connID)
		if c == nil || !scope.visible(c) {
			return nil, nil, domain.ErrNotFound
		}
	}

	ch, cancel := s.active.SubscribeClient(connID)

	return ch, cancel, nil
}
