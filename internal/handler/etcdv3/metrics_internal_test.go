package etcdv3

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func newMetricsTestServer(t *testing.T) (*KVServer, metric.Reader) {
	t.Helper()

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	c, err := provider.Meter("elara/etcd").Int64Counter("elara_writes_rejected_total")
	require.NoError(t, err)

	return &KVServer{metrics: &kvMetrics{writesRejected: c}}, reader
}

func collectRejectedCounter(t *testing.T, ctx context.Context, reader metric.Reader) *metricdata.Sum[int64] {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elara_writes_rejected_total" {
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok, "expected Sum[int64] data type")

				return &sum
			}
		}
	}

	return nil
}

func TestKVServer_RecordRejectedWrite_ConfigLocked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, reader := newMetricsTestServer(t)

	s.recordRejectedWrite(ctx, "Put", "prod", domain.ErrLocked)

	sum := collectRejectedCounter(t, ctx, reader)
	require.NotNil(t, sum, "counter metric not found")
	require.Len(t, sum.DataPoints, 1)
	assert.Equal(t, int64(1), sum.DataPoints[0].Value)

	attrs := sum.DataPoints[0].Attributes
	op, ok := attrs.Value(attribute.Key("op"))
	require.True(t, ok)
	assert.Equal(t, "Put", op.AsString())

	reason, ok := attrs.Value(attribute.Key("reason"))
	require.True(t, ok)
	assert.Equal(t, "config_locked", reason.AsString())

	ns, ok := attrs.Value(attribute.Key("namespace"))
	require.True(t, ok)
	assert.Equal(t, "prod", ns.AsString())
}

func TestKVServer_RecordRejectedWrite_NamespaceLocked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, reader := newMetricsTestServer(t)

	s.recordRejectedWrite(ctx, "DeleteRange", "staging", domain.ErrNamespaceLocked)

	sum := collectRejectedCounter(t, ctx, reader)
	require.NotNil(t, sum)
	require.Len(t, sum.DataPoints, 1)

	reason, ok := sum.DataPoints[0].Attributes.Value(attribute.Key("reason"))
	require.True(t, ok)
	assert.Equal(t, "namespace_locked", reason.AsString())
}

func TestKVServer_RecordRejectedWrite_OtherError_NoIncrement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s, reader := newMetricsTestServer(t)

	s.recordRejectedWrite(ctx, "Put", "prod", errors.New("some db error"))

	sum := collectRejectedCounter(t, ctx, reader)
	assert.Nil(t, sum, "counter must not be incremented for non-locked errors")
}

func TestKVServer_RecordRejectedWrite_NilMetrics_NoPanic(t *testing.T) {
	t.Parallel()

	s := &KVServer{metrics: nil}
	assert.NotPanics(t, func() {
		s.recordRejectedWrite(context.Background(), "Put", "prod", domain.ErrLocked)
	})
}

func TestKVServer_RecordRejectedWrite_NilCounter_NoPanic(t *testing.T) {
	t.Parallel()

	s := &KVServer{metrics: &kvMetrics{}}
	assert.NotPanics(t, func() {
		s.recordRejectedWrite(context.Background(), "Put", "prod", domain.ErrLocked)
	})
}
