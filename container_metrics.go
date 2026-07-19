package dewy

import (
	"context"

	"github.com/linyows/dewy/container"
	"github.com/linyows/dewy/telemetry"
)

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

	return buildContainerSnapshot(app, d.desiredReplicas(), statuses), nil
}

// desiredReplicas mirrors the deploy-time default: a non-positive configured
// count means one replica.
func (d *Dewy) desiredReplicas() int64 {
	if d.config.Container == nil || d.config.Container.Replicas <= 0 {
		return 1
	}
	return int64(d.config.Container.Replicas)
}

// buildContainerSnapshot converts runtime statuses into a telemetry snapshot.
// It is a pure function of its inputs so the mapping can be tested without a
// container runtime. Exited containers are included as-is; they are bounded
// because each deploy reaps them (see Runtime.RemoveExited), so no time-based
// retention filter is needed to cap series growth.
//
// This relies on reaping actually succeeding. Reaping is best-effort, so if the
// runtime persistently fails to remove exited containers while crashes keep
// happening across deploys, terminated-container series can grow unbounded. That
// is an accepted trade-off: such a state also emits loud WARN/ERROR reap-failure
// logs, so it is observable, and a secondary time/count cap here would
// reintroduce the silent-drop behavior this change set out to remove.
func buildContainerSnapshot(app string, desired int64, statuses []*container.Status) telemetry.ContainerSnapshot {
	snap := telemetry.ContainerSnapshot{
		App:             app,
		DesiredReplicas: desired,
		Containers:      make([]telemetry.ContainerStatus, 0, len(statuses)),
	}

	for _, s := range statuses {
		if s == nil {
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
