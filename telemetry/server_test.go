package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func otelWithAttrs(k, v string) otelmetric.MeasurementOption {
	return otelmetric.WithAttributes(attribute.String(k, v))
}

func collectSum(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.DataPoint[int64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			s, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("%s is not an int64 sum: %T", name, m.Data)
			}
			return s.DataPoints
		}
	}
	return nil
}

func TestServerObserverUp(t *testing.T) {
	p, reader := newTestProvider(t, fixedClock())

	up := true
	if err := p.RegisterServerObserver(func() ServerSnapshot { return ServerSnapshot{Up: up} }); err != nil {
		t.Fatalf("RegisterServerObserver: %v", err)
	}

	if dps := collectGauge(t, reader, "dewy.server.up"); len(dps) != 1 || dps[0].Value != 1 {
		t.Errorf("server.up = %+v, want 1", dps)
	}

	up = false
	if dps := collectGauge(t, reader, "dewy.server.up"); len(dps) != 1 || dps[0].Value != 0 {
		t.Errorf("server.up after down = %+v, want 0", dps)
	}
}

func TestServerCountersAndDeploymentCommand(t *testing.T) {
	p, reader := newTestProvider(t, fixedClock())
	m := p.Metrics()
	ctx := context.Background()

	m.ServerRestarts.Add(ctx, 1, otelWithAttrs("reason", "deploy"))
	m.ServerRestarts.Add(ctx, 1, otelWithAttrs("reason", "signal"))
	m.ServerCrashes.Add(ctx, 1)
	m.DeploymentsTotal.Add(ctx, 1, otelWithAttrs("command", "server"))

	// restarts split by reason.
	byReason := map[string]int64{}
	for _, dp := range collectSum(t, reader, "dewy.server.restarts.total") {
		byReason[attrsOf(dp)["reason"]] = dp.Value
	}
	if byReason["deploy"] != 1 || byReason["signal"] != 1 {
		t.Errorf("restarts by reason = %v, want deploy=1 signal=1", byReason)
	}

	if dps := collectSum(t, reader, "dewy.server.crashes.total"); len(dps) != 1 || dps[0].Value != 1 {
		t.Errorf("crashes = %+v, want 1", dps)
	}

	// deployment carries the command attribute.
	dps := collectSum(t, reader, "dewy.deployments.total")
	if len(dps) != 1 || attrsOf(dps[0])["command"] != "server" {
		t.Errorf("deployments.total = %+v, want a single server-command series", dps)
	}
}
