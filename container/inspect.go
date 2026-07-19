package container

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// inspection represents the structure of `<runtime> inspect <container>` JSON
// output. The "container" qualifier is implicit (the package is container);
// the image variant is named imageInspection (in image.go).
type inspection struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Created      string `json:"Created"`
	RestartCount int    `json:"RestartCount"`
	State        struct {
		Status     string `json:"Status"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		ExitCode   int    `json:"ExitCode"`
		OOMKilled  bool   `json:"OOMKilled"`
	} `json:"State"`
	Config struct {
		Image        string              `json:"Image"`
		Labels       map[string]string   `json:"Labels"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// Status is the observed lifecycle state of a single managed container,
// including stopped ones. It carries exactly the fields the telemetry layer
// turns into crash/restart metrics; unlike Info it does not resolve host
// ports, so it can be gathered for exited containers too.
type Status struct {
	ID         string
	Name       string
	Image      string
	State      string // created, running, paused, restarting, exited, dead
	Restarts   int
	ExitCode   int
	OOMKilled  bool
	Replica    string // dewy.replica label, "" for pre-upgrade containers
	Version    string // dewy.version label, "" for pre-upgrade containers
	StartedAt  time.Time
	FinishedAt time.Time
}

// Terminated reports whether the container has stopped running. Only a
// terminated container carries a meaningful ExitCode/FinishedAt.
func (s *Status) Terminated() bool {
	switch s.State {
	case "exited", "dead":
		return true
	default:
		return false
	}
}

// findContainerIDs lists container IDs matching every label. When all is true
// stopped containers are included (ps -a); otherwise only running ones are
// returned. Deploy-path callers must keep all=false: they use the result to
// decide which containers to stop and remove, and including stopped containers
// there would try to tear down already-dead ones.
func (r *Runtime) findContainerIDs(ctx context.Context, labels map[string]string, all bool) ([]string, error) {
	args := []string{"ps", "-q"}
	if all {
		args = append(args, "-a")
	}

	for key, value := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", key, value))
	}

	output, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(output, "\n")
	containers := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

// FindContainerByLabel finds a running container by labels.
func (r *Runtime) FindContainerByLabel(ctx context.Context, labels map[string]string) (string, error) {
	containers, err := r.findContainerIDs(ctx, labels, false)
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "", ErrContainerNotFound
	}
	return containers[0], nil
}

// FindContainersByLabel finds all running containers matching the given labels.
func (r *Runtime) FindContainersByLabel(ctx context.Context, labels map[string]string) ([]string, error) {
	containers, err := r.findContainerIDs(ctx, labels, false)
	if err != nil {
		return nil, err
	}

	r.logger.Debug("Found containers by label",
		slog.Any("labels", labels),
		slog.Int("count", len(containers)))

	return containers, nil
}

// RemoveExited removes exited containers managed by this dewy instance for
// appName, returning the number removed. The rolling deploy only tears down the
// running containers it replaces; a replica that crashes on its own lingers in
// the exited state and is never otherwise reclaimed, so exited containers would
// accumulate across deploys (and keep reporting metrics). Reaping them on each
// deploy keeps that set bounded to the current cycle.
//
// Only status=exited is filtered: it is the common crash state and is
// understood by both docker and podman. Podman has no "dead" state and rejects
// status=dead outright; docker's "dead" is a rare un-removable remnant that rm
// would fail on anyway, so it is not worth a runtime-specific filter.
func (r *Runtime) RemoveExited(ctx context.Context, appName string) (int, error) {
	// Always scope by dewy.app, exactly as the deploy path (FindContainersByLabel)
	// does — even when appName is empty (the registry-derived fallback can yield
	// ""). Omitting it here would reap every managed app's exited containers on a
	// shared runtime, which is a destructive cross-app action.
	args := []string{"ps", "-aq",
		"--filter", "status=exited",
		"--filter", "label=dewy.managed=true",
		"--filter", fmt.Sprintf("label=dewy.app=%s", appName)}

	output, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to list exited containers: %w", err)
	}
	if output == "" {
		return 0, nil
	}

	removed := 0
	for _, id := range strings.Split(output, "\n") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := r.Remove(ctx, id); err != nil {
			// Best-effort: a container that vanished between the list and the
			// remove is fine to skip; log and keep going.
			r.logger.Warn("Failed to remove exited container",
				slog.String("container", id),
				slog.String("error", err.Error()))
			continue
		}
		removed++
	}
	return removed, nil
}

// InspectManaged returns the lifecycle state of every container managed by this
// dewy instance for appName, including stopped ones. A single batched inspect
// backs the whole result so the cost is one exec per call regardless of replica
// count. An empty appName lists all managed containers.
func (r *Runtime) InspectManaged(ctx context.Context, appName string) ([]*Status, error) {
	labels := map[string]string{"dewy.managed": "true"}
	if appName != "" {
		labels["dewy.app"] = appName
	}

	ids, err := r.findContainerIDs(ctx, labels, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list managed containers: %w", err)
	}
	if len(ids) == 0 {
		return []*Status{}, nil
	}

	output, err := r.execCommandOutput(ctx, append([]string{"inspect"}, ids...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect managed containers: %w", err)
	}

	var inspects []inspection
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	statuses := make([]*Status, 0, len(inspects))
	for i := range inspects {
		statuses = append(statuses, r.toStatus(&inspects[i]))
	}
	return statuses, nil
}

// toStatus converts a raw inspection into a Status, tolerating unparseable
// timestamps (a not-yet-started or malformed container yields a zero time
// rather than an error).
func (r *Runtime) toStatus(in *inspection) *Status {
	parseTime := func(v string) time.Time {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return time.Time{}
		}
		return t
	}

	return &Status{
		ID:         in.ID,
		Name:       strings.TrimPrefix(in.Name, "/"),
		Image:      in.Config.Image,
		State:      in.State.Status,
		Restarts:   in.RestartCount,
		ExitCode:   in.State.ExitCode,
		OOMKilled:  in.State.OOMKilled,
		Replica:    in.Config.Labels["dewy.replica"],
		Version:    in.Config.Labels["dewy.version"],
		StartedAt:  parseTime(in.State.StartedAt),
		FinishedAt: parseTime(in.State.FinishedAt),
	}
}

// GetMappedPort returns the host port mapped to the container port.
func (r *Runtime) GetMappedPort(ctx context.Context, containerID string, containerPort int) (int, error) {
	portSpec := fmt.Sprintf("%d/tcp", containerPort)
	output, err := r.execCommandOutput(ctx, "port", containerID, portSpec)
	if err != nil {
		return 0, fmt.Errorf("failed to get mapped port: %w", err)
	}

	if output == "" {
		return 0, fmt.Errorf("no port mapping found for container port %d", containerPort)
	}

	// Output format: "0.0.0.0:32768" or "0.0.0.0:32768\n:::32768"
	parts := strings.Split(output, "\n")
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected port output format: %s", output)
	}

	firstMapping := strings.TrimSpace(parts[0])
	colonIdx := strings.LastIndex(firstMapping, ":")
	if colonIdx == -1 {
		return 0, fmt.Errorf("unexpected port format: %s", firstMapping)
	}

	portStr := firstMapping[colonIdx+1:]
	port, err := fmt.Sscanf(portStr, "%d", new(int))
	if err != nil || port != 1 {
		return 0, fmt.Errorf("failed to parse port number from %s: %w", portStr, err)
	}

	var hostPort int
	n, err := fmt.Sscanf(portStr, "%d", &hostPort)
	if err != nil {
		return 0, fmt.Errorf("fmt.Sscanf failed: n=%d, %w", n, err)
	}

	return hostPort, nil
}

// GetRunningContainerWithImage checks if a container is running with the specified image and app name.
func (r *Runtime) GetRunningContainerWithImage(ctx context.Context, imageRef, appName string) (string, error) {
	output, err := r.execCommandOutput(ctx, "ps",
		"--filter", fmt.Sprintf("ancestor=%s", imageRef),
		"--filter", fmt.Sprintf("label=dewy.app=%s", appName),
		"--filter", "status=running",
		"--format", "{{.ID}}")

	if err != nil {
		return "", fmt.Errorf("failed to list running containers: %w", err)
	}

	if output == "" {
		r.logger.Debug("No running container found with image", slog.String("image", imageRef))
		return "", nil
	}

	containerIDs := strings.Split(output, "\n")
	containerID := strings.TrimSpace(containerIDs[0])

	r.logger.Debug("Found running container with image",
		slog.String("image", imageRef),
		slog.String("container", containerID))
	return containerID, nil
}

// GetContainerInfo returns detailed information about a container.
func (r *Runtime) GetContainerInfo(ctx context.Context, containerID string, containerPort int) (*Info, error) {
	output, err := r.execCommandOutput(ctx, "inspect", containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	var inspects []inspection
	if err := json.Unmarshal([]byte(output), &inspects); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(inspects) == 0 {
		return nil, ErrContainerNotFound
	}

	inspect := inspects[0]

	startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
	if err != nil {
		r.logger.Warn("Failed to parse StartedAt timestamp",
			slog.String("container", containerID),
			slog.String("timestamp", inspect.State.StartedAt))
		startedAt = time.Time{}
	}

	deployedAt := startedAt
	if deployedAtStr, ok := inspect.Config.Labels["dewy.deployed_at"]; ok {
		if t, err := time.Parse(time.RFC3339, deployedAtStr); err == nil {
			deployedAt = t
		}
	}

	ipPort := ""
	if containerPort > 0 {
		portSpec := fmt.Sprintf("%d/tcp", containerPort)
		if portBindings, ok := inspect.NetworkSettings.Ports[portSpec]; ok && len(portBindings) > 0 {
			ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
		}
	} else {
		for _, portBindings := range inspect.NetworkSettings.Ports {
			if len(portBindings) > 0 {
				ipPort = fmt.Sprintf("%s:%s", portBindings[0].HostIP, portBindings[0].HostPort)
				break
			}
		}
	}

	name := strings.TrimPrefix(inspect.Name, "/")

	return &Info{
		ID:         inspect.ID,
		Name:       name,
		Image:      inspect.Config.Image,
		Status:     inspect.State.Status,
		IPPort:     ipPort,
		StartedAt:  startedAt,
		DeployedAt: deployedAt,
		Labels:     inspect.Config.Labels,
	}, nil
}

// ListContainersByLabels returns detailed information about containers matching the given labels.
func (r *Runtime) ListContainersByLabels(ctx context.Context, labels map[string]string, containerPort int) ([]*Info, error) {
	containerIDs, err := r.FindContainersByLabel(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to find containers: %w", err)
	}

	infos := make([]*Info, 0, len(containerIDs))
	for _, containerID := range containerIDs {
		info, err := r.GetContainerInfo(ctx, containerID, containerPort)
		if err != nil {
			r.logger.Warn("Failed to get container info",
				slog.String("container", containerID),
				slog.String("error", err.Error()))
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}
