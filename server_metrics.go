package dewy

import (
	"context"
	"time"

	"github.com/linyows/dewy/telemetry"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// telemetryOn reports whether telemetry is wired up and enabled. All recording
// helpers guard on it so call sites stay uncluttered.
func (d *Dewy) telemetryOn() bool {
	return d.telemetry != nil && d.telemetry.Enabled()
}

// commandAttr labels a metric with the running command (server|assets|container)
// so the deployment metrics can be told apart across modes.
func (d *Dewy) commandAttr() otelmetric.MeasurementOption {
	return otelmetric.WithAttributes(attribute.String("command", d.config.Command.String()))
}

// recordDeployment records the outcome of one deploy attempt. A non-nil err
// counts an error; success counts the deployment and its duration. Shared by
// the server/assets path (Run) and the container path (applyContainerDeployment)
// so every mode reports consistently.
func (d *Dewy) recordDeployment(ctx context.Context, dur time.Duration, deployErr error) {
	if !d.telemetryOn() {
		return
	}
	m := d.telemetry.Metrics()
	attr := d.commandAttr()
	if deployErr != nil {
		m.DeploymentErrors.Add(ctx, 1, attr)
		return
	}
	m.DeploymentsTotal.Add(ctx, 1, attr)
	m.DeploymentDuration.Record(ctx, dur.Seconds(), attr)
}

// recordServerRestart counts a managed-server restart with its cause.
func (d *Dewy) recordServerRestart(ctx context.Context, reason string) {
	if !d.telemetryOn() {
		return
	}
	d.telemetry.Metrics().ServerRestarts.Add(ctx, 1,
		otelmetric.WithAttributes(attribute.String("reason", reason)))
}

// recordServerCrash counts a managed-server crash (the process exited on its own).
func (d *Dewy) recordServerCrash(ctx context.Context) {
	if !d.telemetryOn() {
		return
	}
	d.telemetry.Metrics().ServerCrashes.Add(ctx, 1)
}

// observeServer is the telemetry.ServerObserver for server mode: it reports
// whether the supervised process is currently running.
func (d *Dewy) observeServer() telemetry.ServerSnapshot {
	d.RLock()
	up := d.isServerRunning
	d.RUnlock()
	return telemetry.ServerSnapshot{Up: up}
}
