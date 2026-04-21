package etcdv3

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestToKVStatus_NamespaceLocked_NormalizedToConfigMessage(t *testing.T) {
	t.Parallel()

	// Simulate the wrap produced by validateNamespaceUnlocked when a config
	// inside a locked namespace is mutated.
	wrapped := fmt.Errorf("put: %w", fmt.Errorf("namespace %q: %w", "prod", domain.ErrNamespaceLocked))

	got := toKVStatus(wrapped, "put", "/foo.json")

	st, ok := status.FromError(got)
	require.True(t, ok, "result must be a gRPC status")

	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Equal(t, `put: config "/foo.json" is locked`, st.Message(),
		"etcd clients must see config-only wording even when the cause is namespace-level")
}

func TestToKVStatus_ConfigLocked_UsesSamePathBasedMessage(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("delete: %w", domain.NewLockedError("/foo.json"))

	got := toKVStatus(wrapped, "delete range", "/foo.json")

	st, ok := status.FromError(got)
	require.True(t, ok)

	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Equal(t, `delete range: config "/foo.json" is locked`, st.Message())
}

func TestToKVStatus_OtherError_StaysInternal(t *testing.T) {
	t.Parallel()

	got := toKVStatus(errors.New("disk full"), "put", "/foo.json")

	st, ok := status.FromError(got)
	require.True(t, ok)

	assert.Equal(t, codes.Internal, st.Code())
}
