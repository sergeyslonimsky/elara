package service

import (
	"context"
	"fmt"
	"path/filepath"

	coreotel "github.com/sergeyslonimsky/core/otel"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

// Infrastructure groups every external I/O endpoint owned by the service.
// A field belongs here when it represents a connection or client to a system
// outside the process: a database, a message broker, an observability backend.
//
// Examples (current and future):
//   - bbolt store (local on-disk DB — the process boundary is the file lock)
//   - OTLP tracer provider (network exporter)
//   - Prometheus metrics exporter (HTTP scrape target)
//   - Future Kafka producer / consumer host
//   - Future RabbitMQ publisher / consumer host
//   - Future Redis / S3 / object-store clients
//
// Every field implements core/lifecycle.Resource (and may also be Runner —
// e.g. a Kafka consumer host). Lifecycle is owned by app.App; each field is
// registered individually in cmd/service/main.go.
//
// What does NOT belong here:
//   - In-memory primitives that talk to no external system (pub/sub
//     publisher, in-memory registries, internal event buffers) → Components.
//   - Persistence adapters that wrap the store with domain methods →
//     Repositories.
//   - Application-level background workers with business logic → Workers.
//
// Infrastructure has no internal dependencies — it sits at the very bottom of
// the layering and is constructed first.
type Infrastructure struct {
	Store     *bbolt.Store
	Telemetry *PrometheusMetrics // nil when cfg.Metrics.Enabled = false
	Tracing   *coreotel.Provider // noop provider when cfg.Tracing.Enabled = false
}

// NewInfrastructure opens every external I/O endpoint. Failures are fatal —
// partial state (an opened store, an initialised exporter) is cleaned up
// before the error returns.
func NewInfrastructure(ctx context.Context, cfg config.Config) (*Infrastructure, error) {
	store, err := bbolt.Open(filepath.Join(cfg.DataPath, "elara.db"))
	if err != nil {
		return nil, fmt.Errorf("open bbolt store: %w", err)
	}

	var telemetry *PrometheusMetrics

	if cfg.Metrics.Enabled {
		telemetry, err = NewPrometheusMetrics(cfg.ServiceName, cfg.ServiceVersion)
		if err != nil {
			_ = store.Close()

			return nil, fmt.Errorf("init prometheus metrics: %w", err)
		}
	}

	tracing, err := coreotel.Setup(ctx, coreotel.Config{ //nolint:exhaustruct // only traces are enabled
		Disabled:       !cfg.Tracing.Enabled,
		OTelHost:       cfg.Tracing.OTLPEndpoint,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		EnableTracer:   cfg.Tracing.Enabled,
		EnableMetrics:  false,
		EnableLogger:   false,
	})
	if err != nil {
		_ = store.Close()

		return nil, fmt.Errorf("init otel tracing: %w", err)
	}

	return &Infrastructure{
		Store:     store,
		Telemetry: telemetry,
		Tracing:   tracing,
	}, nil
}
