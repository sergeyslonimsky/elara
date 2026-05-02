package casbin

//go:generate mockgen -destination=mocks/mock_adapter.go -package=casbin_mock github.com/sergeyslonimsky/elara/internal/auth/casbin ContextPolicyRepo

import (
	"context"
	"fmt"
)

// ContextPolicyRepo is the context-aware interface implemented by the bbolt policy repository.
type ContextPolicyRepo interface {
	Load(ctx context.Context) ([][]string, error)
	Save(ctx context.Context, rules [][]string) error
}

type contextPolicyAdapter struct {
	repo ContextPolicyRepo
	ctx  context.Context //nolint:containedctx // intentional adapter pattern for context bridging
}

// NewContextPolicyLoader wraps a ContextPolicyRepo as a context-free PolicyLoader.
// Pass context.Background() for startup use; pass the request ctx for request-scoped use.
//
//nolint:ireturn // intentional factory: hides contextPolicyAdapter implementation behind PolicyLoader
func NewContextPolicyLoader(
	repo ContextPolicyRepo,
	ctx context.Context, //nolint:revive // ctx after repo is intentional
) PolicyLoader {
	return &contextPolicyAdapter{repo: repo, ctx: ctx}
}

func (a *contextPolicyAdapter) Load() ([][]string, error) {
	rules, err := a.repo.Load(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	return rules, nil
}

func (a *contextPolicyAdapter) Save(rules [][]string) error {
	if err := a.repo.Save(a.ctx, rules); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}
