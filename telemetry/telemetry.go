package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds telemetry configuration.
type Config struct {
	Enabled      bool
	OTLPEndpoint string // OTLP gRPC endpoint (e.g., "localhost:4317"), empty to disable
	ServiceName  string
	Version      string
}

// Provider manages OpenTelemetry meter provider and Prometheus handler.
type Provider struct {
	meterProvider *sdkmetric.MeterProvider
	promHandler   http.Handler
	metrics       *Metrics
}

// New creates a new telemetry Provider with Prometheus and optional OTLP exporters.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return &Provider{}, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "dewy"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Prometheus exporter (always enabled when telemetry is on)
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	}

	// OTLP exporter (optional)
	if cfg.OTLPEndpoint != "" {
		otlpExporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}

		opts = append(opts, sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(otlpExporter,
				sdkmetric.WithInterval(15*time.Second),
			),
		))
	}

	mp := sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("github.com/linyows/dewy")
	metrics, err := newMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	return &Provider{
		meterProvider: mp,
		promHandler:   promhttp.Handler(),
		metrics:       metrics,
	}, nil
}

// Metrics returns the metrics instruments.
func (p *Provider) Metrics() *Metrics {
	return p.metrics
}

// PrometheusHandler returns the HTTP handler for /metrics endpoint.
func (p *Provider) PrometheusHandler() http.Handler {
	if p.promHandler == nil {
		return http.NotFoundHandler()
	}
	return p.promHandler
}

// Shutdown gracefully shuts down the meter provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.meterProvider == nil {
		return nil
	}
	return p.meterProvider.Shutdown(ctx)
}

// Enabled returns whether telemetry is enabled.
func (p *Provider) Enabled() bool {
	return p.meterProvider != nil
}

// Metrics holds all metric instruments for dewy.
type Metrics struct {
	// Proxy metrics
	ProxyConnectionsTotal   otelmetric.Int64Counter
	ProxyActiveConnections  otelmetric.Int64UpDownCounter
	ProxyConnectionDuration otelmetric.Float64Histogram
	ProxyConnectLatency     otelmetric.Float64Histogram
	ProxyBytesTransferred   otelmetric.Int64Counter
	ProxyErrorsTotal        otelmetric.Int64Counter
	ProxyBackendCount       otelmetric.Int64UpDownCounter

	// Deployment metrics
	DeploymentsTotal    otelmetric.Int64Counter
	DeploymentDuration  otelmetric.Float64Histogram
	DeploymentErrors    otelmetric.Int64Counter
	HealthChecksTotal   otelmetric.Int64Counter
	HealthCheckFailures otelmetric.Int64Counter

	// Container metrics
	ContainerReplicas otelmetric.Int64UpDownCounter
}

func newMetrics(meter otelmetric.Meter) (*Metrics, error) {
	m := &Metrics{}
	var err error

	// Proxy metrics
	if m.ProxyConnectionsTotal, err = meter.Int64Counter("dewy.proxy.connections.total",
		otelmetric.WithDescription("Total number of proxy connections accepted"),
		otelmetric.WithUnit("{connection}"),
	); err != nil {
		return nil, err
	}

	if m.ProxyActiveConnections, err = meter.Int64UpDownCounter("dewy.proxy.connections.active",
		otelmetric.WithDescription("Number of currently active proxy connections"),
		otelmetric.WithUnit("{connection}"),
	); err != nil {
		return nil, err
	}

	if m.ProxyConnectionDuration, err = meter.Float64Histogram("dewy.proxy.connection.duration",
		otelmetric.WithDescription("Duration of proxy connections"),
		otelmetric.WithUnit("s"),
	); err != nil {
		return nil, err
	}

	if m.ProxyConnectLatency, err = meter.Float64Histogram("dewy.proxy.connect.latency",
		otelmetric.WithDescription("Latency to establish connection to backend"),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5),
	); err != nil {
		return nil, err
	}

	if m.ProxyBytesTransferred, err = meter.Int64Counter("dewy.proxy.bytes.transferred",
		otelmetric.WithDescription("Total bytes transferred through proxy"),
		otelmetric.WithUnit("By"),
	); err != nil {
		return nil, err
	}

	if m.ProxyErrorsTotal, err = meter.Int64Counter("dewy.proxy.errors.total",
		otelmetric.WithDescription("Total number of proxy errors"),
		otelmetric.WithUnit("{error}"),
	); err != nil {
		return nil, err
	}

	if m.ProxyBackendCount, err = meter.Int64UpDownCounter("dewy.proxy.backends",
		otelmetric.WithDescription("Number of active proxy backends"),
		otelmetric.WithUnit("{backend}"),
	); err != nil {
		return nil, err
	}

	// Deployment metrics
	if m.DeploymentsTotal, err = meter.Int64Counter("dewy.deployments.total",
		otelmetric.WithDescription("Total number of deployments"),
		otelmetric.WithUnit("{deployment}"),
	); err != nil {
		return nil, err
	}

	if m.DeploymentDuration, err = meter.Float64Histogram("dewy.deployment.duration",
		otelmetric.WithDescription("Duration of deployments"),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300, 600),
	); err != nil {
		return nil, err
	}

	if m.DeploymentErrors, err = meter.Int64Counter("dewy.deployment.errors.total",
		otelmetric.WithDescription("Total number of deployment errors"),
		otelmetric.WithUnit("{error}"),
	); err != nil {
		return nil, err
	}

	if m.HealthChecksTotal, err = meter.Int64Counter("dewy.healthchecks.total",
		otelmetric.WithDescription("Total number of health checks performed"),
		otelmetric.WithUnit("{check}"),
	); err != nil {
		return nil, err
	}

	if m.HealthCheckFailures, err = meter.Int64Counter("dewy.healthchecks.failures.total",
		otelmetric.WithDescription("Total number of failed health checks"),
		otelmetric.WithUnit("{check}"),
	); err != nil {
		return nil, err
	}

	// Container metrics
	if m.ContainerReplicas, err = meter.Int64UpDownCounter("dewy.container.replicas",
		otelmetric.WithDescription("Number of running container replicas"),
		otelmetric.WithUnit("{replica}"),
	); err != nil {
		return nil, err
	}

	return m, nil
}
