package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

type stubLockStore struct {
	locked   bool
	lockErr  error
	getCfg   *domain.Config
	getErr   error
	lockCall int
	getCall  int
}

func (s *stubLockStore) LockConfig(_ context.Context, _, _ string) error {
	s.lockCall++
	if s.lockErr != nil {
		return s.lockErr
	}
	s.locked = true

	return nil
}

func (s *stubLockStore) Get(_ context.Context, _, _ string) (*domain.Config, error) {
	s.getCall++
	if s.getErr != nil {
		return nil, s.getErr
	}

	return s.getCfg, nil
}

type captureLockNotifier struct {
	lockedCfg   *domain.Config
	unlockedCfg *domain.Config
}

func (c *captureLockNotifier) NotifyConfigLocked(_ context.Context, cfg *domain.Config) {
	c.lockedCfg = cfg
}

func (c *captureLockNotifier) NotifyConfigUnlocked(_ context.Context, cfg *domain.Config) {
	c.unlockedCfg = cfg
}

func TestLockUseCase_EmitsWatchEvent(t *testing.T) {
	t.Parallel()

	store := &stubLockStore{
		getCfg: &domain.Config{Path: "/a.json", Namespace: "prod", Locked: true},
	}
	notifier := &captureLockNotifier{}

	uc := config.NewLockUseCase(store, notifier)

	require.NoError(t, uc.Execute(context.Background(), "prod", "/a.json"))

	require.NotNil(t, notifier.lockedCfg, "publisher must receive the locked config")
	assert.True(t, notifier.lockedCfg.Locked)
	assert.Equal(t, "/a.json", notifier.lockedCfg.Path)
	assert.Equal(t, "prod", notifier.lockedCfg.Namespace)
}

func TestLockUseCase_PropagatesLockError(t *testing.T) {
	t.Parallel()

	store := &stubLockStore{lockErr: errors.New("boom")}
	notifier := &captureLockNotifier{}

	uc := config.NewLockUseCase(store, notifier)

	require.Error(t, uc.Execute(context.Background(), "prod", "/a.json"))
	assert.Nil(t, notifier.lockedCfg, "publisher must not fire if store rejects")
}

func TestLockUseCase_GetFailure_StillEmits(t *testing.T) {
	t.Parallel()

	store := &stubLockStore{getErr: errors.New("read failed")}
	notifier := &captureLockNotifier{}

	uc := config.NewLockUseCase(store, notifier)

	// Lock committed → no caller error, degraded event still emitted.
	require.NoError(t, uc.Execute(context.Background(), "prod", "/a.json"))
	require.NotNil(t, notifier.lockedCfg)
	assert.True(t, notifier.lockedCfg.Locked)
}

type stubUnlockStore struct {
	unlockErr error
	getCfg    *domain.Config
	getErr    error
}

func (s *stubUnlockStore) UnlockConfig(_ context.Context, _, _ string) error {
	return s.unlockErr
}

func (s *stubUnlockStore) Get(_ context.Context, _, _ string) (*domain.Config, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}

	return s.getCfg, nil
}

func TestUnlockUseCase_EmitsWatchEvent(t *testing.T) {
	t.Parallel()

	store := &stubUnlockStore{
		getCfg: &domain.Config{Path: "/a.json", Namespace: "prod", Locked: false},
	}
	notifier := &captureLockNotifier{}

	uc := config.NewUnlockUseCase(store, notifier)

	require.NoError(t, uc.Execute(context.Background(), "prod", "/a.json"))
	require.NotNil(t, notifier.unlockedCfg)
	assert.False(t, notifier.unlockedCfg.Locked)
}
