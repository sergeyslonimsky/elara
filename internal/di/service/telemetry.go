package service

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sergeyslonimsky/core/lifecycle"
	"go.opentelemetry.io/otel"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// PrometheusMetrics wires a pull-based Prometheus meter provider and
// exposes an http.Handler ready to be mounted on /metrics via
// http2.WithMetricsHandler.
//
// This is the infra-service counterpart to core/otel.Setup's OTLP push
// metrics: Prometheus Operator scrapes /metrics on the service, no
// collector required. Tracing (if enabled) still goes through core/otel
// as OTLP push — metrics and traces are independent signals here.
//
// Implements lifecycle.Resource.
type PrometheusMetrics struct {
	handler http.Handler

	provider     *sdkmetric.MeterProvider
	shutdownOnce sync.Once
	shutdownErr  error
}

// NewPrometheusMetrics creates a MeterProvider backed by a Prometheus
// exporter, sets it as the global provider (so library instrumentation
// picks it up), and returns the wrapper.
//
// serviceName and serviceVersion populate resource.service.name /
// service.version so scraped metrics carry consistent labels.
func NewPrometheusMetrics(serviceName, serviceVersion string) (*PrometheusMetrics, error) {
	exp, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		)),
	)

	otel.SetMeterProvider(provider)

	return &PrometheusMetrics{
		handler:  promhttp.Handler(),
		provider: provider,
	}, nil
}

// Handler returns the http.Handler that serves Prometheus-format metrics.
// Typical use: pass to http2.WithMetricsHandler so it's mounted at /metrics.
func (p *PrometheusMetrics) Handler() http.Handler {
	return p.handler
}

// Shutdown flushes pending metrics and releases the provider. Idempotent
// and concurrent-safe. Implements lifecycle.Resource.
func (p *PrometheusMetrics) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		if err := p.provider.Shutdown(ctx); err != nil {
			p.shutdownErr = fmt.Errorf("shutdown prometheus meter provider: %w", err)
		}
	})

	return p.shutdownErr
}

var _ lifecycle.Resource = (*PrometheusMetrics)(nil)
