package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewDisabled(t *testing.T) {
	p, err := New(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Enabled() {
		t.Error("expected telemetry to be disabled")
	}
	if p.Metrics() != nil {
		t.Error("expected nil metrics when disabled")
	}
}

func TestNewEnabled(t *testing.T) {
	p, err := New(context.Background(), Config{
		Enabled:     true,
		ServiceName: "test-dewy",
		Version:     "0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = p.Shutdown(context.Background()) }()

	if !p.Enabled() {
		t.Error("expected telemetry to be enabled")
	}
	if p.Metrics() == nil {
		t.Error("expected non-nil metrics")
	}
}

func TestPrometheusHandler(t *testing.T) {
	p, err := New(context.Background(), Config{
		Enabled:     true,
		ServiceName: "test-dewy",
		Version:     "0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = p.Shutdown(context.Background()) }()

	// Record a metric
	ctx := context.Background()
	p.Metrics().ProxyConnectionsTotal.Add(ctx, 1)

	// Verify /metrics endpoint responds
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	p.PrometheusHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty metrics response")
	}

	// Verify dewy-specific metrics are present (Prometheus format uses _ instead of .)
	expectedMetrics := []string{
		"dewy_proxy_connections_total",
	}
	for _, name := range expectedMetrics {
		if !strings.Contains(body, name) {
			t.Errorf("expected metrics response to contain %q", name)
		}
	}
}

func TestPrometheusHandlerDisabled(t *testing.T) {
	p, err := New(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	p.PrometheusHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when disabled, got %d", rec.Code)
	}
}

func TestMetricsInstruments(t *testing.T) {
	p, err := New(context.Background(), Config{
		Enabled:     true,
		ServiceName: "test-dewy",
		Version:     "0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = p.Shutdown(context.Background()) }()

	m := p.Metrics()
	ctx := context.Background()

	// Verify all metric instruments can be used without panic
	m.ProxyConnectionsTotal.Add(ctx, 1)
	m.ProxyActiveConnections.Add(ctx, 1)
	m.ProxyActiveConnections.Add(ctx, -1)
	m.ProxyConnectionDuration.Record(ctx, 1.5)
	m.ProxyConnectLatency.Record(ctx, 0.01)
	m.ProxyBytesTransferred.Add(ctx, 1024)
	m.ProxyErrorsTotal.Add(ctx, 1)
	m.ProxyBackendCount.Add(ctx, 1)
	m.DeploymentsTotal.Add(ctx, 1)
	m.DeploymentDuration.Record(ctx, 30.0)
	m.DeploymentErrors.Add(ctx, 1)
	m.HealthChecksTotal.Add(ctx, 1)
	m.HealthCheckFailures.Add(ctx, 1)
	m.ContainerReplicas.Add(ctx, 2)
}

func TestShutdown(t *testing.T) {
	p, err := New(context.Background(), Config{
		Enabled:     true,
		ServiceName: "test-dewy",
		Version:     "0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestShutdownDisabled(t *testing.T) {
	p, err := New(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}
