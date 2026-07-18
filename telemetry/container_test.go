package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linyows/dewy/internal/sysdeps/fake"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func fixedClock() *fake.Clock {
	return fake.NewClock(time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
}

func TestContainerObserverCachesWithinTTL(t *testing.T) {
	clock := fixedClock()
	calls := 0
	state := &containerObserverState{
		fn: func(context.Context) (ContainerSnapshot, error) {
			calls++
			return ContainerSnapshot{App: "app"}, nil
		},
		clock:   clock,
		ttl:     5 * time.Second,
		timeout: time.Second,
	}

	if _, ok := state.snapshot(context.Background()); !ok || calls != 1 {
		t.Fatalf("first snapshot: ok=%v calls=%d", ok, calls)
	}
	// Within TTL: cached, no new call.
	clock.Advance(4 * time.Second)
	if _, ok := state.snapshot(context.Background()); !ok || calls != 1 {
		t.Fatalf("cached snapshot: ok=%v calls=%d, want cached without a new call", ok, calls)
	}
	// Past TTL: refreshes.
	clock.Advance(2 * time.Second)
	if _, ok := state.snapshot(context.Background()); !ok || calls != 2 {
		t.Fatalf("refreshed snapshot: ok=%v calls=%d, want a second call", ok, calls)
	}
}

func TestContainerObserverReportsNothingOnError(t *testing.T) {
	clock := fixedClock()
	calls := 0
	state := &containerObserverState{
		fn: func(context.Context) (ContainerSnapshot, error) {
			calls++
			return ContainerSnapshot{App: "app"}, errors.New("daemon down")
		},
		clock:   clock,
		ttl:     5 * time.Second,
		timeout: time.Second,
	}

	if _, ok := state.snapshot(context.Background()); ok {
		t.Fatal("error should yield ok=false (report nothing)")
	}
	// The failure is negatively cached: no hammering the daemon each scrape.
	clock.Advance(time.Second)
	if _, ok := state.snapshot(context.Background()); ok || calls != 1 {
		t.Fatalf("within TTL after error: ok=%v calls=%d, want cached failure without a new call", ok, calls)
	}
}

func TestContainerObserverTimesOut(t *testing.T) {
	clock := fixedClock()
	state := &containerObserverState{
		fn: func(ctx context.Context) (ContainerSnapshot, error) {
			<-ctx.Done() // simulate a wedged runtime
			return ContainerSnapshot{}, ctx.Err()
		},
		clock:   clock,
		ttl:     5 * time.Second,
		timeout: 10 * time.Millisecond,
	}

	if _, ok := state.snapshot(context.Background()); ok {
		t.Fatal("timed-out collection should yield ok=false")
	}
}

// newTestProvider builds a Provider backed by a ManualReader so tests can drive
// collection deterministically.
func newTestProvider(t *testing.T, clock *fake.Clock) (*Provider, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")
	metrics, err := newMetrics(meter)
	if err != nil {
		t.Fatalf("newMetrics: %v", err)
	}
	return &Provider{meterProvider: mp, meter: meter, metrics: metrics, clock: clock}, reader
}

func collectGauge(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.DataPoint[int64] {
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
			g, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("%s is not an int64 gauge: %T", name, m.Data)
			}
			return g.DataPoints
		}
	}
	return nil
}

func attrsOf(dp metricdata.DataPoint[int64]) map[string]string {
	out := map[string]string{}
	for _, kv := range dp.Attributes.ToSlice() {
		out[string(kv.Key)] = kv.Value.Emit()
	}
	return out
}

func TestContainerMetricsObserveEndToEnd(t *testing.T) {
	clock := fixedClock()
	p, reader := newTestProvider(t, clock)

	started := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	snap := ContainerSnapshot{
		App:             "app",
		DesiredReplicas: 2,
		Containers: []ContainerStatus{
			{Name: "app-1-0", Image: "img:v1", Version: "v1", Replica: "0", State: "running", Restarts: 1, StartedAt: started},
			{Name: "app-1-1", Image: "img:v1", Version: "v1", Replica: "1", State: "exited", Restarts: 4, ExitCode: 137, OOMKilled: true, Terminated: true, StartedAt: started},
		},
	}
	if err := p.RegisterContainerObserver(func(context.Context) (ContainerSnapshot, error) {
		return snap, nil
	}); err != nil {
		t.Fatalf("RegisterContainerObserver: %v", err)
	}

	// replicas: 1 running out of desired 2.
	if dps := collectGauge(t, reader, "dewy.container.replicas"); len(dps) != 1 || dps[0].Value != 1 {
		t.Errorf("replicas = %+v, want single value 1", dps)
	}
	if dps := collectGauge(t, reader, "dewy.container.desired_replicas"); len(dps) != 1 || dps[0].Value != 2 {
		t.Errorf("desired_replicas = %+v, want single value 2", dps)
	}

	// restarts: one series per container.
	restarts := map[string]int64{}
	for _, dp := range collectGauge(t, reader, "dewy.container.restarts") {
		restarts[attrsOf(dp)["container"]] = dp.Value
	}
	if restarts["app-1-0"] != 1 || restarts["app-1-1"] != 4 {
		t.Errorf("restarts = %v, want {app-1-0:1, app-1-1:4}", restarts)
	}

	// exit code and oom only for the terminated container.
	exit := collectGauge(t, reader, "dewy.container.last_terminated.exit_code")
	if len(exit) != 1 || attrsOf(exit[0])["container"] != "app-1-1" || exit[0].Value != 137 {
		t.Errorf("exit_code = %+v, want only app-1-1=137", exit)
	}
	oom := collectGauge(t, reader, "dewy.container.oom_killed")
	if len(oom) != 1 || oom[0].Value != 1 {
		t.Errorf("oom_killed = %+v, want only terminated container = 1", oom)
	}

	// status: running container has state=running -> 1, others 0.
	var runningVal, exitedForRunning int64 = -1, -1
	for _, dp := range collectGauge(t, reader, "dewy.container.status") {
		a := attrsOf(dp)
		if a["container"] == "app-1-0" && a["state"] == "running" {
			runningVal = dp.Value
		}
		if a["container"] == "app-1-0" && a["state"] == "exited" {
			exitedForRunning = dp.Value
		}
	}
	if runningVal != 1 || exitedForRunning != 0 {
		t.Errorf("status for running container: running=%d exited=%d, want 1 and 0", runningVal, exitedForRunning)
	}
}

func TestRegisterContainerObserverNoopWhenDisabled(t *testing.T) {
	p := &Provider{} // disabled provider: nil meter/metrics
	if err := p.RegisterContainerObserver(func(context.Context) (ContainerSnapshot, error) {
		return ContainerSnapshot{}, nil
	}); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}
