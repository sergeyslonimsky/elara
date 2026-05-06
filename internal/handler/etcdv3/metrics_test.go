package etcdv3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestKVServer_RecordRejectedWrite(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("elara/etcd")

	c, _ := meter.Int64Counter("elara_writes_rejected_total")
	m := &kvMetrics{writesRejected: c}
	s := &KVServer{metrics: m}

	ctx := context.Background()
	s.recordRejectedWrite(ctx, "Put", "prod", domain.ErrLocked)

	var rm metricdata.ResourceMetrics
	_ = reader.Collect(ctx, &rm)

	assert.NotEmpty(t, rm.ScopeMetrics)
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "elara_writes_rejected_total" {
				found = true
				break
			}
		}
	}
	assert.True(t, found)
}
