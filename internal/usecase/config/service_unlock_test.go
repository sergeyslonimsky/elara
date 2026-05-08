package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Unlock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		svc, m, _ := setupService(t)

		ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})
		m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
		m.storage.EXPECT().UnlockConfig(ctx, "prod", "/a.json").Return(nil)

		cfg := &domain.Config{Path: "/a.json", Namespace: "prod"}
		m.storage.EXPECT().Get(ctx, "/a.json", "prod").Return(cfg, nil)
		m.watcher.EXPECT().NotifyConfigUnlocked(ctx, cfg)

		err := svc.Unlock(ctx, config.UnlockInput{Namespace: "prod", Path: "/a.json"})
		require.NoError(t, err)
	})
}
