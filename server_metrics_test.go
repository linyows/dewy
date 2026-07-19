package dewy

import (
	"context"
	"testing"
	"time"
)

func TestObserveServerReflectsRunning(t *testing.T) {
	d := &Dewy{}

	if d.observeServer().Up {
		t.Error("expected Up=false before the server starts")
	}

	d.Lock()
	d.isServerRunning = true
	d.Unlock()

	if !d.observeServer().Up {
		t.Error("expected Up=true once the server is running")
	}
}

func TestRecordHelpersNoopWithoutTelemetry(t *testing.T) {
	// With no telemetry provider wired, the record helpers must be safe no-ops
	// (not nil-deref) so call sites can stay unconditional.
	d := &Dewy{}
	if d.telemetryOn() {
		t.Fatal("telemetry should be off on a bare Dewy")
	}
	ctx := context.Background()
	d.recordDeployment(ctx, time.Second, nil)
	d.recordDeployment(ctx, time.Second, context.Canceled)
	d.recordServerRestart(ctx, "deploy")
	d.recordServerCrash(ctx)
}
