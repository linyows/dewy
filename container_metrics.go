package dewy

import (
	"context"
	"time"

	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/telemetry"
)

// terminatedRetention bounds how long a stopped container keeps reporting
// metrics. dewy does not currently reap exited containers, and `ps -a` would
// otherwise let their series accumulate without limit. A crash is actionable
// for a while after it happens; past this window the series is dropped.
const terminatedRetention = 1 * time.Hour

// observeContainers is the telemetry.ContainerObserver for container mode. It
// inspects the managed containers and shapes them into a snapshot. Before the
// first deploy tick the runtime does not exist yet; that is reported as an
// empty snapshot (not an error) so startup scrapes stay quiet instead of
// logging failures.
func (d *Dewy) observeContainers(ctx context.Context) (telemetry.ContainerSnapshot, error) {
	d.RLock()
	rt := d.containerRuntime
	d.RUnlock()

	app := d.appName()
	if rt == nil {
		return telemetry.ContainerSnapshot{App: app, DesiredReplicas: d.desiredReplicas()}, nil
	}

	statuses, err := rt.InspectManaged(ctx, app)
	if err != nil {
		return telemetry.ContainerSnapshot{}, err
	}

	return buildContainerSnapshot(app, d.desiredReplicas(), statuses, time.Now(), terminatedRetention), nil
}

// desiredReplicas mirrors the deploy-time default: a non-positive configured
// count means one replica.
func (d *Dewy) desiredReplicas() int64 {
	if d.config.Container == nil || d.config.Container.Replicas <= 0 {
		return 1
	}
	return int64(d.config.Container.Replicas)
}

// buildContainerSnapshot converts runtime statuses into a telemetry snapshot,
// dropping containers that terminated longer than retention ago. It is a pure
// function of its inputs so the filtering and mapping can be tested without a
// container runtime.
func buildContainerSnapshot(app string, desired int64, statuses []*container.Status, now time.Time, retention time.Duration) telemetry.ContainerSnapshot {
	snap := telemetry.ContainerSnapshot{
		App:             app,
		DesiredReplicas: desired,
		Containers:      make([]telemetry.ContainerStatus, 0, len(statuses)),
	}

	for _, s := range statuses {
		if s == nil {
			continue
		}
		if s.Terminated() && !s.FinishedAt.IsZero() && now.Sub(s.FinishedAt) > retention {
			continue
		}
		snap.Containers = append(snap.Containers, telemetry.ContainerStatus{
			Name:       s.Name,
			Image:      s.Image,
			Version:    s.Version,
			Replica:    s.Replica,
			State:      s.State,
			Restarts:   int64(s.Restarts),
			ExitCode:   int64(s.ExitCode),
			OOMKilled:  s.OOMKilled,
			Terminated: s.Terminated(),
			StartedAt:  s.StartedAt,
		})
	}

	return snap
}
