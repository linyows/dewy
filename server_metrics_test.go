package dewy

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/linyows/dewy/logging"
)

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

func TestSupervisorConfigReportsCrashes(t *testing.T) {
	d := &Dewy{logger: logging.SetupLogger("error", "text", io.Discard), config: Config{Server: &ServerConfig{Command: "true"}}}
	if d.supervisorConfig().OnCrash == nil {
		t.Fatal("supervisorConfig().OnCrash = nil, want crashes reported to telemetry")
	}
}
