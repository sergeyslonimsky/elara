package webhook_test

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func webhookTestCtx(ctx context.Context) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: "test@example.com"})
}
