package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

type stubCopyGetter struct {
	cfg *domain.Config
	err error
}

func (s *stubCopyGetter) Get(_ context.Context, _, _ string) (*domain.Config, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.cfg, nil
}

type stubCopyCreator struct {
	called bool
	err    error
}

func (s *stubCopyCreator) Create(_ context.Context, _ *domain.Config) error {
	s.called = true

	return s.err
}

type stubCopyNotifier struct{}

func (stubCopyNotifier) NotifyCreated(_ context.Context, _ *domain.Config) {}

type stubCopyNSChecker struct {
	ns  *domain.Namespace
	err error
}

func (s *stubCopyNSChecker) Get(_ context.Context, _ string) (*domain.Namespace, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.ns, nil
}

type stubCopyNSTimestamp struct{}

func (stubCopyNSTimestamp) UpdateTimestamp(_ context.Context, _ string) error { return nil }

func TestCopy_DestinationNamespaceLocked(t *testing.T) {
	t.Parallel()

	source := &domain.Config{
		Path: "/a.json", Content: `{}`, Format: domain.FormatJSON, Namespace: "src",
	}
	creator := &stubCopyCreator{}

	uc := config.NewCopyUseCase(
		&stubCopyGetter{cfg: source},
		creator,
		stubCopyNotifier{},
		&stubCopyNSChecker{ns: &domain.Namespace{Name: "dst", Locked: true}},
		stubCopyNSTimestamp{},
	)

	_, err := uc.Execute(context.Background(), "/a.json", "src", "/b.json", "dst")
	require.ErrorIs(t, err, domain.ErrLocked)
	assert.False(t, creator.called, "creator must not be called when destination namespace is locked")
}

func TestCopy_DestinationNamespaceNotFound(t *testing.T) {
	t.Parallel()

	uc := config.NewCopyUseCase(
		&stubCopyGetter{cfg: &domain.Config{Path: "/a.json", Namespace: "src"}},
		&stubCopyCreator{},
		stubCopyNotifier{},
		&stubCopyNSChecker{err: domain.NewNotFoundError("namespace", "dst")},
		stubCopyNSTimestamp{},
	)

	_, err := uc.Execute(context.Background(), "/a.json", "src", "/b.json", "dst")
	require.Error(t, err)

	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve, "expected validation error for missing destination namespace")
}
