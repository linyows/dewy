---
title: Architecture
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy provides two distinct deployment modes for different use cases:

1. **Supervisor Mode** (`server` / `assets` commands): Dewy deploys onto the host directly. The `server` command runs the application as a child process and restarts it without downtime by passing the listening sockets to the new process. The `assets` command replaces files and runs no process.
2. **Proxy Mode** (`container` command): Dewy runs a built-in TCP reverse proxy and manages Docker/Podman containers with rolling update deployments.

## Interfaces

Dewy is composed of four interfaces as pluggable abstractions: Registry, Artifact, Cache, and Notifier.

- Registry: Version management (GitHub Releases, S3, GCS, OCI Registry, gRPC)
- Artifact: Binary/Image acquisition (corresponding Registry formats)
- Cache: Downloaded file management (File, S3, GCS)
- Notifier: Deployment notifications (Slack, Mail)

### Deployment Process

The following diagram illustrates Dewy's deployment process and configuration.

![Hi-level Architecture](/images/dewy-architecture.svg)

1. Registry polls the specified registry to detect the latest version of the application
2. Artifact downloads and extracts artifacts from the specified artifact store
3. Cache stores the latest version and artifacts
4. Dewy creates child processes for new version applications and begins processing requests with the new application
5. Registry saves information about when, where, and what was deployed as files
6. Notifier sends notifications to specified notification destinations

## Supervisor Mode (server / assets)

In Supervisor Mode, Dewy acts as the main process and launches applications as child processes. This mode is ideal for deploying binary applications directly on VMs or bare-metal servers.

![Server Command Architecture](/images/server-architecture.svg)

{% caption %}
The `server` command replaces the application in this order: 1. Start the new process on the same sockets. 2. Wait for `--startup-wait` and, with `--health-before-switch`, the health check. 3. Send SIGTERM to the previous process, and SIGKILL after `--drain-time`. The sockets never close, so no connection is refused during the replacement.
{% /caption %}

### Zero-Downtime Restart with Socket Inheritance

For the `server` command, Dewy binds the listening sockets for `--port` once when the server starts and keeps them open until Dewy shuts down. The application runs as a worker process that inherits these sockets, so it serves on them instead of binding the ports itself; see [Application Integration for the Server Command](/application) for what the application needs to do. When a new version is detected:

1. Dewy starts a new worker process beside the current one; it inherits the same listening sockets
2. Once the new worker has stayed up for the startup wait (`--startup-wait`, 1 second by default), and has passed the health check on its own socket when `--health-before-switch` is set, Dewy sends SIGTERM to the previous worker
3. The previous worker finishes handling existing requests and exits
4. The sockets stay open throughout, so connections that arrive during the switch are queued instead of refused

If the new worker exits within the startup wait or fails that health check, the previous worker keeps serving and the deployment is treated as failed. See [Signal Handling](/signal-handling) for the restart and shutdown behavior.

## Proxy Mode (container)

In Proxy Mode, Dewy manages containerized applications using Docker or Podman. It runs a built-in TCP reverse proxy that routes traffic to container backends, enabling zero-downtime rolling updates.

![Proxy Architecture](/images/proxy-architecture.svg)

{% caption %}
The `container` command replaces the replicas in this order: 1. Start a new container, published on a random `127.0.0.1` port. 2. Once it is running, and has passed the health check when `--health-path` is set, add it to the proxy, and repeat until `--replicas` new containers run. 3. Remove each old container from the proxy, then stop it with SIGTERM and SIGKILL after `--drain-time`. Old and new containers receive traffic together until step 3, so the proxy always has a running container to forward to.
{% /caption %}

### Components

- **TCP Reverse Proxy**: Listens on configured ports and load-balances traffic across container backends using round-robin
- **Container Runtime**: Manages Docker or Podman containers (pull, run, stop, remove)
- **Health Checker**: Verifies container health before adding to proxy backends
- **Admin API**: Provides HTTP endpoints for monitoring and management (e.g., `dewy container list`)

### Deployment Process

1. **Registry Check**: Polls OCI registry (e.g., GitHub Container Registry, Docker Hub) to detect new image versions
2. **Image Pull**: Downloads the new container image
3. **Rolling Update**: For each replica:
   - Start new container with random localhost port mapping
   - Wait for container to pass health checks (if configured)
   - Add new container to proxy backends
4. **Traffic Switch**: New containers now receive traffic via the proxy
5. **Old Container Cleanup**: For each old container:
   - Remove from proxy backends
   - Wait for drain time to allow in-flight requests to complete
   - Stop and remove the old container
6. **Image Cleanup**: Remove old images, keeping only the most recent versions

### Zero-Downtime Updates

The rolling update strategy ensures zero-downtime deployments:

- New containers are fully healthy before receiving traffic
- Old containers continue serving requests until drained
- The TCP proxy handles the traffic switching seamlessly
- Rollback occurs automatically if new containers fail health checks

### Port Mapping

Containers are started with localhost-only port bindings (`127.0.0.1::containerPort`), which assigns random ephemeral ports. This approach:

- Avoids port conflicts between multiple replicas
- Isolates container traffic to localhost (only accessible via proxy)
- Allows the proxy to manage all external access

## Comparison

| Feature | Supervisor Mode | Proxy Mode |
|---------|-----------------|------------|
| Target | Binary applications | Container images |
| Deployment unit | Archive (tar.gz, zip) | OCI image |
| Process management | Child process | Container runtime |
| Load balancing | External (or single process) | Built-in TCP proxy |
| Zero-downtime update | Worker replacement on Dewy-owned sockets | Rolling update |
| Replicas | Single process | Multiple containers |
| Health check | HTTP endpoint (`--health-path`) | HTTP endpoint |

In this way, Dewy communicates with a central registry to achieve pull-based deployment for both binary applications and containerized workloads.
