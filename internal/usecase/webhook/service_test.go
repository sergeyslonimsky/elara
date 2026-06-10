package webhook_test

import (
	"context"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

func webhookTestCtx(ctx context.Context) context.Context {
	user := &domain.User{
		ID:     uuid.MustParse(testUserID),
		Email:  "test@example.com",
		Status: domain.UserStatusActive,
	}
	sess := &domain.Session{
		ID:     "test-sess",
		UserID: testUserID,
	}

	return authctx.WithSession(ctx, sess, user)
}
