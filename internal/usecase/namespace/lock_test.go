package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
)

type stubNSLocker struct {
	locked bool
	err    error
}

func (s *stubNSLocker) LockNamespace(_ context.Context, _ string) error {
	if s.err != nil {
		return s.err
	}
	s.locked = true

	return nil
}

type stubNSUnlocker struct {
	unlocked bool
	err      error
}

func (s *stubNSUnlocker) UnlockNamespace(_ context.Context, _ string) error {
	if s.err != nil {
		return s.err
	}
	s.unlocked = true

	return nil
}

type captureNSNotifier struct {
	locked   string
	unlocked string
}

func (c *captureNSNotifier) NotifyNamespaceLocked(_ context.Context, ns string)   { c.locked = ns }
func (c *captureNSNotifier) NotifyNamespaceUnlocked(_ context.Context, ns string) { c.unlocked = ns }

func TestNamespaceLockUseCase_EmitsWatchEvent(t *testing.T) {
	t.Parallel()

	store := &stubNSLocker{}
	notifier := &captureNSNotifier{}

	uc := namespace.NewLockUseCase(store, notifier)

	require.NoError(t, uc.Execute(context.Background(), "prod"))
	assert.True(t, store.locked)
	assert.Equal(t, "prod", notifier.locked, "publisher must be notified with the namespace name")
}

func TestNamespaceLockUseCase_SkipsNotifyOnError(t *testing.T) {
	t.Parallel()

	store := &stubNSLocker{err: errors.New("boom")}
	notifier := &captureNSNotifier{}

	uc := namespace.NewLockUseCase(store, notifier)

	require.Error(t, uc.Execute(context.Background(), "prod"))
	assert.Empty(t, notifier.locked)
}

func TestNamespaceUnlockUseCase_EmitsWatchEvent(t *testing.T) {
	t.Parallel()

	store := &stubNSUnlocker{}
	notifier := &captureNSNotifier{}

	uc := namespace.NewUnlockUseCase(store, notifier)

	require.NoError(t, uc.Execute(context.Background(), "prod"))
	assert.Equal(t, "prod", notifier.unlocked)
}
