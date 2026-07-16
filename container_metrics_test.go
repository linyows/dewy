package dewy

import (
	"testing"
	"time"

	"github.com/linyows/dewy/container"
)

func TestBuildContainerSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	started := now.Add(-2 * time.Hour)

	statuses := []*container.Status{
		{Name: "app-0", State: "running", Replica: "0", Version: "v1", Restarts: 1, StartedAt: started},
		// Terminated recently: kept.
		{Name: "app-1", State: "exited", Replica: "1", ExitCode: 137, OOMKilled: true, FinishedAt: now.Add(-10 * time.Minute)},
		// Terminated long ago: dropped to bound series growth.
		{Name: "app-old", State: "exited", Replica: "2", FinishedAt: now.Add(-3 * time.Hour)},
		nil, // defensive: skipped
	}

	snap := buildContainerSnapshot("app", 2, statuses, now, terminatedRetention)

	if snap.App != "app" || snap.DesiredReplicas != 2 {
		t.Fatalf("snapshot header wrong: %+v", snap)
	}
	if len(snap.Containers) != 2 {
		t.Fatalf("got %d containers, want 2 (stale exited one dropped): %+v", len(snap.Containers), snap.Containers)
	}

	byName := map[string]bool{}
	for _, c := range snap.Containers {
		byName[c.Name] = true
	}
	if byName["app-old"] {
		t.Error("container terminated beyond retention should be dropped")
	}
	if !byName["app-0"] || !byName["app-1"] {
		t.Errorf("expected app-0 and app-1 retained, got %v", byName)
	}

	// Terminated flag is derived from state, not passed through.
	for _, c := range snap.Containers {
		if c.Name == "app-1" && !c.Terminated {
			t.Error("exited container should be marked Terminated")
		}
		if c.Name == "app-0" && c.Terminated {
			t.Error("running container should not be marked Terminated")
		}
	}
}

func TestBuildContainerSnapshotKeepsZeroFinishedAt(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	// Terminated but FinishedAt unknown (zero): age is indeterminate, so it
	// is kept rather than silently dropped.
	statuses := []*container.Status{
		{Name: "app-x", State: "dead", Replica: "0"},
	}
	snap := buildContainerSnapshot("app", 1, statuses, now, terminatedRetention)
	if len(snap.Containers) != 1 {
		t.Fatalf("terminated container with zero FinishedAt should be kept, got %d", len(snap.Containers))
	}
}
