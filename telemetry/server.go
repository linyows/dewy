package telemetry

import (
	"context"

	otelmetric "go.opentelemetry.io/otel/metric"
)

// serverMetrics holds the asynchronous instrument for the managed server's
// up-state. Restarts and crashes are synchronous counters on Metrics; only the
// current up/down state is observed.
type serverMetrics struct {
	up otelmetric.Int64ObservableGauge
}

func (s *serverMetrics) init(meter otelmetric.Meter) error {
	var err error
	// No unit: a dimensionless 0/1 state flag.
	s.up, err = meter.Int64ObservableGauge("dewy.server.up",
		otelmetric.WithDescription("Whether the managed server process is currently running (1) or not (0)"),
	)
	return err
}

// ServerSnapshot is the observed state of the managed server process.
type ServerSnapshot struct {
	Up bool
}

// ServerObserver returns the current server state. Unlike the container
// observer it takes no context and returns no error: the implementation reads
// an in-memory flag, so there is nothing to time out or fail.
type ServerObserver func() ServerSnapshot

// RegisterServerObserver wires an observer for the server up-state gauge. It is
// a no-op when telemetry is disabled or fn is nil.
func (p *Provider) RegisterServerObserver(fn ServerObserver) error {
	if p.meter == nil || p.metrics == nil || fn == nil {
		return nil
	}
	sm := &p.metrics.server
	_, err := p.meter.RegisterCallback(func(_ context.Context, o otelmetric.Observer) error {
		v := int64(0)
		if fn().Up {
			v = 1
		}
		o.ObserveInt64(sm.up, v)
		return nil
	}, sm.up)
	return err
}
