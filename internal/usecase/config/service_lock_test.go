package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Lock(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		svc, m, _ := setupService(t)

		ctx := auth.WithClaims(t.Context(), &auth.Claims{Email: "user@example.com"})
		m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "write").Return(true, nil)
		m.storage.EXPECT().LockConfig(ctx, "prod", "/a.json").Return(nil)

		cfg := &domain.Config{Path: "/a.json", Namespace: "prod"}
		m.storage.EXPECT().Get(ctx, "/a.json", "prod").Return(cfg, nil)
		m.watcher.EXPECT().NotifyConfigLocked(ctx, cfg)

		err := svc.Lock(ctx, config.LockInput{Namespace: "prod", Path: "/a.json"})
		require.NoError(t, err)
	})
}
