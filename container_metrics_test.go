package dewy

import (
	"testing"
	"time"

	"github.com/linyows/dewy/container"
)

func TestBuildContainerSnapshot(t *testing.T) {
	started := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)

	statuses := []*container.Status{
		{Name: "app-0", State: "running", Replica: "0", Version: "v1", Restarts: 1, StartedAt: started},
		{Name: "app-1", State: "exited", Replica: "1", ExitCode: 137, OOMKilled: true, FinishedAt: started.Add(time.Minute)},
		nil, // defensive: skipped
	}

	snap := buildContainerSnapshot("app", 2, statuses)

	if snap.App != "app" || snap.DesiredReplicas != 2 {
		t.Fatalf("snapshot header wrong: %+v", snap)
	}
	// Both real containers are mapped; exited ones are not dropped (reaping
	// bounds them, so there is no time-based retention filter).
	if len(snap.Containers) != 2 {
		t.Fatalf("got %d containers, want 2: %+v", len(snap.Containers), snap.Containers)
	}

	byName := map[string]telemetryStatus{}
	for _, c := range snap.Containers {
		byName[c.Name] = telemetryStatus{terminated: c.Terminated, exitCode: c.ExitCode, restarts: c.Restarts}
	}

	// Terminated flag is derived from state, not passed through.
	if !byName["app-1"].terminated {
		t.Error("exited container should be marked Terminated")
	}
	if byName["app-0"].terminated {
		t.Error("running container should not be marked Terminated")
	}
	if byName["app-1"].exitCode != 137 || byName["app-0"].restarts != 1 {
		t.Errorf("fields mapped wrong: %+v", byName)
	}
}

type telemetryStatus struct {
	terminated bool
	exitCode   int64
	restarts   int64
}
