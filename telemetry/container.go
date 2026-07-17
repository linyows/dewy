package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	// observeMinInterval is the minimum spacing between container inspections.
	// Prometheus and OTLP readers can both trigger a collection; caching within
	// this window collapses concurrent scrapes into one container-runtime call.
	observeMinInterval = 5 * time.Second

	// observeTimeout caps a single inspection so a wedged container daemon
	// cannot hang a metrics scrape indefinitely.
	observeTimeout = 2 * time.Second
)

// containerStates is the fixed set of lifecycle states reported by
// dewy.container.status. Reporting every state (0 for all but the current one)
// lets queries use absent()/max without guessing which labels exist.
var containerStates = []string{"created", "running", "paused", "restarting", "exited", "dead"}

// ContainerStatus is the observed state of a single managed container. The
// telemetry package deliberately does not import the container package; the
// dewy layer translates container.Status into this shape.
type ContainerStatus struct {
	Name       string
	Image      string
	Version    string
	Replica    string
	State      string
	Restarts   int64
	ExitCode   int64
	OOMKilled  bool
	Terminated bool
	StartedAt  time.Time
}

// ContainerSnapshot is one collection cycle's view of an app's containers.
type ContainerSnapshot struct {
	App             string
	DesiredReplicas int64
	Containers      []ContainerStatus
}

// ContainerObserver returns the current container snapshot. Returning an error
// means "report nothing this cycle" — callers must not fabricate a snapshot,
// so a failed inspection leaves the series stale rather than reporting a
// misleading value.
type ContainerObserver func(ctx context.Context) (ContainerSnapshot, error)

// containerMetrics holds the asynchronous instruments backing the container
// lifecycle metrics.
type containerMetrics struct {
	restarts        otelmetric.Int64ObservableGauge
	status          otelmetric.Int64ObservableGauge
	lastExitCode    otelmetric.Int64ObservableGauge
	oomKilled       otelmetric.Int64ObservableGauge
	startedAt       otelmetric.Int64ObservableGauge
	replicas        otelmetric.Int64ObservableGauge
	desiredReplicas otelmetric.Int64ObservableGauge
	info            otelmetric.Int64ObservableGauge
}

func (c *containerMetrics) init(meter otelmetric.Meter) error {
	var err error
	if c.restarts, err = meter.Int64ObservableGauge("dewy.container.restarts",
		otelmetric.WithDescription("Container restart count as reported by the runtime (resets when the container is replaced)"),
		otelmetric.WithUnit("{restart}"),
	); err != nil {
		return err
	}
	// No unit: this is a dimensionless 0/1 state flag, not a container count.
	if c.status, err = meter.Int64ObservableGauge("dewy.container.status",
		otelmetric.WithDescription("Container lifecycle state (1 for the current state, 0 otherwise), keyed by the state attribute"),
	); err != nil {
		return err
	}
	if c.lastExitCode, err = meter.Int64ObservableGauge("dewy.container.last_terminated.exit_code",
		otelmetric.WithDescription("Exit code of a terminated container (reported only while it lingers)"),
	); err != nil {
		return err
	}
	if c.oomKilled, err = meter.Int64ObservableGauge("dewy.container.oom_killed",
		otelmetric.WithDescription("Whether a terminated container was OOM-killed (1) or not (0)"),
	); err != nil {
		return err
	}
	if c.startedAt, err = meter.Int64ObservableGauge("dewy.container.started.timestamp",
		otelmetric.WithDescription("Start time of the container as a Unix timestamp"),
		otelmetric.WithUnit("s"),
	); err != nil {
		return err
	}
	if c.replicas, err = meter.Int64ObservableGauge("dewy.container.replicas",
		otelmetric.WithDescription("Number of running container replicas"),
		otelmetric.WithUnit("{replica}"),
	); err != nil {
		return err
	}
	if c.desiredReplicas, err = meter.Int64ObservableGauge("dewy.container.desired_replicas",
		otelmetric.WithDescription("Number of container replicas dewy is configured to run"),
		otelmetric.WithUnit("{replica}"),
	); err != nil {
		return err
	}
	if c.info, err = meter.Int64ObservableGauge("dewy.container.info",
		otelmetric.WithDescription("Static per-container info series (always 1), labeled with image and version"),
	); err != nil {
		return err
	}
	return nil
}

// instruments lists the observables for RegisterCallback.
func (c *containerMetrics) instruments() []otelmetric.Observable {
	return []otelmetric.Observable{
		c.restarts, c.status, c.lastExitCode, c.oomKilled,
		c.startedAt, c.replicas, c.desiredReplicas, c.info,
	}
}

// observe emits every container series for one snapshot.
func (c *containerMetrics) observe(o otelmetric.Observer, snap ContainerSnapshot) {
	running := int64(0)
	for _, cs := range snap.Containers {
		if cs.State == "running" {
			running++
		}

		base := []attribute.KeyValue{
			attribute.String("app", snap.App),
			attribute.String("container", cs.Name),
			attribute.String("replica", cs.Replica),
		}

		o.ObserveInt64(c.restarts, cs.Restarts, otelmetric.WithAttributes(base...))

		for _, state := range containerStates {
			v := int64(0)
			if cs.State == state {
				v = 1
			}
			attrs := append(base[:len(base):len(base)], attribute.String("state", state))
			o.ObserveInt64(c.status, v, otelmetric.WithAttributes(attrs...))
		}

		if !cs.StartedAt.IsZero() {
			o.ObserveInt64(c.startedAt, cs.StartedAt.Unix(), otelmetric.WithAttributes(base...))
		}

		// Exit code and OOM state are only meaningful for a container that
		// has actually stopped; reporting them for a running one would be a
		// stale zero.
		if cs.Terminated {
			o.ObserveInt64(c.lastExitCode, cs.ExitCode, otelmetric.WithAttributes(base...))
			oom := int64(0)
			if cs.OOMKilled {
				oom = 1
			}
			o.ObserveInt64(c.oomKilled, oom, otelmetric.WithAttributes(base...))
		}

		o.ObserveInt64(c.info, 1, otelmetric.WithAttributes(
			attribute.String("app", snap.App),
			attribute.String("container", cs.Name),
			attribute.String("replica", cs.Replica),
			attribute.String("image", cs.Image),
			attribute.String("version", cs.Version),
		))
	}

	appAttr := otelmetric.WithAttributes(attribute.String("app", snap.App))
	o.ObserveInt64(c.replicas, running, appAttr)
	o.ObserveInt64(c.desiredReplicas, snap.DesiredReplicas, appAttr)
}

// RegisterContainerObserver wires an observer into the container metrics. It is
// a no-op when telemetry is disabled or fn is nil. The observer is called at
// most once per observeMinInterval, and each call receives a ctx deadline of
// observeTimeout; the deadline only bounds observers that honor ctx
// cancellation (dewy's does, via exec.CommandContext). A failed or timed-out
// collection reports nothing for that cycle.
func (p *Provider) RegisterContainerObserver(fn ContainerObserver) error {
	if p.meter == nil || p.metrics == nil || fn == nil {
		return nil
	}

	state := &containerObserverState{
		fn:      fn,
		clock:   p.clock,
		ttl:     observeMinInterval,
		timeout: observeTimeout,
	}
	cm := &p.metrics.container

	_, err := p.meter.RegisterCallback(func(ctx context.Context, o otelmetric.Observer) error {
		snap, ok := state.snapshot(ctx)
		if !ok {
			return nil
		}
		cm.observe(o, snap)
		return nil
	}, cm.instruments()...)
	return err
}

// containerObserverState adds caching and a ctx deadline around a
// ContainerObserver. The whole collection is serialized so simultaneous
// Prometheus and OTLP scrapes collapse into a single runtime call, and a cached
// result (success or failure) is reused within ttl.
//
// Because fn is called while holding mu, the deadline is only a real bound if
// fn honors ctx cancellation: an observer that blocks while ignoring ctx would
// stall every concurrent collection until it returns.
type containerObserverState struct {
	fn      ContainerObserver
	clock   sysdeps.Clock
	ttl     time.Duration
	timeout time.Duration

	mu       sync.Mutex
	cached   ContainerSnapshot
	cachedOK bool
	cachedAt time.Time
	primed   bool
}

// snapshot returns the current snapshot and whether it should be reported.
// (zero, false) means "report nothing".
func (s *containerObserverState) snapshot(ctx context.Context) (ContainerSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	if s.primed && now.Sub(s.cachedAt) < s.ttl {
		return s.cached, s.cachedOK
	}

	cctx, cancel := context.WithTimeout(ctx, s.timeout)
	snap, err := s.fn(cctx)
	cancel()

	s.primed = true
	s.cachedAt = now
	if err != nil {
		s.cached = ContainerSnapshot{}
		s.cachedOK = false
	} else {
		s.cached = snap
		s.cachedOK = true
	}
	return s.cached, s.cachedOK
}
