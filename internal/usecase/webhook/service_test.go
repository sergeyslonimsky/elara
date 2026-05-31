package webhook_test

import (
	"context"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
)

func webhookTestCtx(ctx context.Context) context.Context {
	return auth2.WithClaims(ctx, &auth2.Claims{Email: "test@example.com"})
}
