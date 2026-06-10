package main

import (
	"context"
	"fmt"

	coreotel "github.com/sergeyslonimsky/core/otel"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
)

// setupMetrics initialises a Prometheus pull-based MeterProvider and the
// /metrics HTTP handler if cfg.Metrics.Enabled is true. Returns nil (no
// error) when metrics are disabled — callers must check the returned
// value before using it.
func setupMetrics(cfg config.Config) (*service.PrometheusMetrics, error) {
	if !cfg.Metrics.Enabled {
		return nil, nil //nolint:nilnil // "disabled" is a valid non-error outcome
	}

	pm, err := service.NewPrometheusMetrics(cfg.ServiceName, cfg.ServiceVersion)
	if err != nil {
		return nil, fmt.Errorf("init prometheus metrics: %w", err)
	}

	return pm, nil
}

// setupTracing initialises the core/otel tracer with an OTLP HTTP trace
// exporter when cfg.Tracing.Enabled is true. Metrics and logs are left
// off: metrics go via Prometheus pull (see setupMetrics), logs go to
// stdout as JSON and are picked up by the cluster log collector.
//
// When tracing is disabled, returns a noop Provider so the lifecycle
// registration path stays uniform.
func setupTracing(ctx context.Context, cfg config.Config) (*coreotel.Provider, error) {
	otelCfg := coreotel.Config{ //nolint:exhaustruct // we intentionally only enable traces
		Disabled:       !cfg.Tracing.Enabled,
		OTelHost:       cfg.Tracing.OTLPEndpoint,
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		EnableTracer:   cfg.Tracing.Enabled,
		EnableMetrics:  false,
		EnableLogger:   false,
	}

	provider, err := coreotel.Setup(ctx, otelCfg)
	if err != nil {
		return nil, fmt.Errorf("otel setup: %w", err)
	}

	return provider, nil
}
