package etcdv3

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// kvMetrics groups instruments published by the etcd-compatible KV server.
type kvMetrics struct {
	writesRejected metric.Int64Counter
}

// newKVMetrics builds the instruments off the global meter provider set up by
// service/telemetry.go. Failure here is non-fatal — instrumentation is not
// allowed to break the data path, so we fall back to a no-op counter.
func newKVMetrics() *kvMetrics {
	meter := otel.GetMeterProvider().Meter("elara/etcd")

	c, err := meter.Int64Counter(
		"elara_writes_rejected_total",
		metric.WithDescription("Writes rejected by lock guards on the etcd-compatible API."),
	)
	if err != nil {
		slog.Warn("failed to register elara_writes_rejected_total counter", "err", err)

		return &kvMetrics{}
	}

	return &kvMetrics{writesRejected: c}
}

// recordRejectedWrite increments the counter when a write was rejected because
// of a lock guard. No-op for any other error.
func (s *KVServer) recordRejectedWrite(ctx context.Context, op, namespace string, err error) {
	if s.metrics == nil || s.metrics.writesRejected == nil {
		return
	}

	if !errors.Is(err, domain.ErrLocked) {
		return
	}

	reason := "config_locked"
	if errors.Is(err, domain.ErrNamespaceLocked) {
		reason = "namespace_locked"
	}

	s.metrics.writesRejected.Add(ctx, 1, metric.WithAttributes(
		attribute.String("op", op),
		attribute.String("reason", reason),
		attribute.String("namespace", namespace),
	))
}
