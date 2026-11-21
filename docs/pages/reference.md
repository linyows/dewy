---
title: Dewy CLI Reference
description: Complete reference guide for Dewy CLI commands and options
---

# Dewy CLI Reference

This page provides detailed information about Dewy CLI commands, options, and usage examples.

## Basic Commands

Dewy provides three main commands: `server`, `assets`, and `image`. These commands are used to deploy and manage applications in different environments.

### server Command

The `dewy server` command starts the main Dewy process and handles binary application deployment and monitoring. This command provides the core functionality for non-container deployments.

```bash
dewy server [options] -- [application command]
```

### assets Command

The `dewy assets` command displays detailed information about the current artifacts. This is useful for checking deployment status.

```bash
dewy assets [options]
```

### container Command

The `dewy container` command handles container image deployment with zero-downtime rolling update deployment strategy. It monitors OCI registries for new image versions and automatically deploys them.

```bash
dewy container [options]
```

#### container list Subcommand

The `dewy container list` subcommand displays information about currently running containers managed by dewy.

```bash
dewy container list
```

**Output:**
```
UPSTREAM           DEPLOY TIME            NAME
127.0.0.1:8080     2025-01-15 10:30:00    myapp-0
127.0.0.1:8081     2025-01-15 10:30:05    myapp-1
127.0.0.1:8082     2025-01-15 10:30:10    myapp-2
```

Shows:
- **UPSTREAM**: Proxy backend address (IP:port)
- **DEPLOY TIME**: When the container was deployed
- **NAME**: Container name (sorted alphabetically)

**Note:** The command connects to the dewy admin API on TCP localhost port 17539 (default). If multiple dewy instances are running with port conflicts, the command automatically scans ports 17539-17548. Can be run from any directory.

## Command Line Options

Use the following options to customize Dewy's behavior.

### --registry (-r)

Specifies the registry URL to retrieve application version information. Supports various registries including GitHub Releases, DockerHub, and ECR.

```bash
dewy server --registry ghr://owner/repo -- /opt/app/current/app
```

### --artifact (-a)

Specifies the location of artifacts to download. Supports multiple protocols including S3, GitHub, and HTTP/HTTPS.

```bash
dewy server --artifact s3://bucket/path/to/artifact -- /opt/app/current/app
```

### --cache (-c)

Selects the cache backend by URL. Supported schemes: `file://` (default local filesystem), `s3://<region>/<bucket>/<prefix>` (Amazon S3 or S3-compatible storage), `gs://<bucket>/<prefix>` (Google Cloud Storage). When using S3 or GCS, multiple Dewy instances pointed at the same bucket share artifact downloads, dramatically reducing upstream registry traffic.

Add `?registry-ttl=<duration>` (e.g. `30s`, `2m`) to also share the upstream registry response across instances. Only one instance per TTL window calls the upstream registry; the rest read the cached response from the shared cache. The option is ignored on the file backend.

```bash
# Local filesystem (default)
dewy server --cache file:///tmp/dewy-cache -- /opt/app/current/app

# Amazon S3
dewy server --cache s3://ap-northeast-1/dewy-cache/myapp -- /opt/app/current/app

# Google Cloud Storage
dewy server --cache gs://dewy-cache/myapp -- /opt/app/current/app

# Shared cache with registry-result caching
dewy server --cache 's3://ap-northeast-1/dewy-cache/myapp?registry-ttl=30s' -- /opt/app/current/app
```

### --notifier (-n)

Configures deployment status notifications. Notification channels such as Slack and email can be set up.

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

Specifies the port(s) used by the application server. This option is optional for the `server` command.

When specified, Dewy binds the given port(s) when the server starts, keeps them open until Dewy shuts down, and passes them to the application as inherited file descriptors. When omitted, the application runs without port listening (useful for job workers, message queue consumers, etc.).

A port can be a single number (`8080`), a comma-separated list (`8080,8081`), or a range (`8080-8085`), and the option can be repeated. A port without a suffix is TCP. For the `server` command, append `/udp` to a port or a range to open a UDP socket (`53/udp`, `6000-6002/udp`); `/tcp` is accepted and means the same as no suffix. Duplicates are removed, and the ports are sorted by number, with the TCP port before the UDP port of the same number.

Without `--host`, each port listens on all interfaces. On a host with IPv6, the socket accepts both IPv4 and IPv6 connections.

The application finds the sockets in the `DEWY_PORT` environment variable. The variable lists one `<address>=<fd>` entry per port, joined by `;`. `<address>` is the port as configured, `host:port` when `--host` is set to an address other than `0.0.0.0` or `::`, with an IPv6 address in brackets, and a UDP entry ends in `/udp`. `<fd>` is the file descriptor number, starting from 3. Dewy also sets the same value in `SERVER_STARTER_PORT` for applications written for server-starter.

| `--port` and `--host` | `DEWY_PORT` |
|---|---|
| `--port 8080` | `8080=3` |
| `--port 8080 --host 127.0.0.1` | `127.0.0.1:8080=3` |
| `--port 8080 --host ::1` | `[::1]:8080=3` |
| `--port 53,53/udp` | `53=3;53/udp=4` |
| `--port 53/udp --host ::1` | `[::1]:53/udp=3` |

An application opens a TCP entry with `net.FileListener` and a UDP entry with `net.FilePacketConn`. See [Application Integration](/application#inherited-sockets) for sample code.

```bash
# With port (HTTP server)
dewy server --port 9090 -- /opt/app/current/app

# TCP and UDP (DNS server)
dewy server --registry ghr://owner/repo --port 53,53/udp -- /opt/dns/current/dns

# Without port (job worker, etc.)
dewy server --registry ghr://owner/repo -- /opt/worker/current/worker
```

### --host

Specifies the IP address the `server` command listens on. The address applies to every `--port`, TCP and UDP. When omitted, the server listens on all interfaces, accepting both IPv4 and IPv6 connections on a host with IPv6. `0.0.0.0` listens on every IPv4 interface only, and `::` on every IPv6 interface only.

The option requires `--port`. An IPv4 or IPv6 unicast address is accepted. Write an IPv6 address without brackets (`::1`); a link-local address may include a zone (`fe80::1%eth0`). A specific IPv6 address listens on IPv6 only. A hostname such as `localhost` and a multicast or broadcast address are rejected. The `assets` and `container` commands reject this option.

With an address other than `0.0.0.0` or `::`, the `DEWY_PORT` environment variable passed to the application lists each listener as `host:port=fd` (for example, `127.0.0.1:8000=3` or `[::1]:8000=3`) instead of `port=fd`. An application that parses the variable itself must accept this form. With `0.0.0.0` or `::`, the variable keeps the `port=fd` form.

When `--health-path` is set without `--health-before-switch`, the health check probes the given address. With `0.0.0.0` or no `--host`, it probes `127.0.0.1`; with `::`, it probes `::1`.

```bash
# Listen on the loopback interface only
dewy server --registry ghr://owner/repo --host 127.0.0.1 --port 8000 -- /opt/app/current/app

# Listen on a specific interface
dewy server --registry ghr://owner/repo --host 192.168.10.11 --port 8000-8001 -- /opt/app/current/app

# Listen on the IPv6 loopback address only
dewy server --registry ghr://owner/repo --host ::1 --port 8000 -- /opt/app/current/app
```

### --startup-wait

Specifies, in seconds, how long a new process must stay up before it can replace the previous one. Applies to the `server` command only. Default is 1 second; `0` also selects the default, and a negative value is rejected.

On a restart (a deployment, a rollback, or `SIGUSR1`), Dewy starts the new process beside the previous one and waits this long. If the new process exits within this time, the restart fails and the previous process keeps serving. A deployment whose restart fails follows the rollback flow described under `--health-path`. The first process started by `dewy server` is also required to stay up for this time.

Raise the value for an application that can fail after a few seconds of initialization, such as one that loads its configuration or connects to a database after it starts.

```bash
dewy server --registry ghr://owner/repo --port 8000 --startup-wait 5 -- /opt/app/current/app
```

### --health-before-switch

Probes `--health-path` on each new process before it replaces the previous one, instead of through `--port` after the switch. Applies to the `server` command only and requires `--health-path`.

With this option, each new process receives one extra TCP socket that no other process shares. The socket listens on a free port on the loopback interface (`127.0.0.1`, or `::1` when IPv4 is unavailable). It is passed as the next inherited file descriptor after the `DEWY_PORT` sockets, and the `DEWY_HEALTH_PORT` environment variable announces it as a single `<address>=<fd>` entry, for example `127.0.0.1:41234=5`. With no `--port`, the descriptor is 3.

After the `--startup-wait` time, Dewy sends requests to `--health-path` on that socket until one succeeds or `--health-timeout` runs out. Only then does the new process replace the previous one.

- On success, the previous process is sent SIGTERM as on any restart. No further probe is made after the switch.
- On failure (no answer, for example because the process is deadlocked, a status other than 2xx or 3xx, or the process exits), Dewy kills the new process. The previous process never stops serving. During a deployment, the version is recorded as blocked and the `current` symlink is restored to the previous release without a restart, because the previous process is still running. The notification states that the previous release's process never stopped serving. A restart requested with `SIGUSR1` that fails the probe leaves the previous process running and changes nothing else.

The first process that `dewy server` starts is probed on its health socket in the same way. If it fails, the start fails.

The application must serve its health endpoint on the `DEWY_HEALTH_PORT` socket. Dewy cannot distinguish an application that does not serve the socket from one that is hung, so with this option every deployment of such an application fails. For that reason the option is not enabled by default. See [Application Integration](/application#health-checks) for sample code.

Because the probe does not use `--port`, `--health-path` with this option does not require a TCP `--port`. An application that listens only on UDP ports, or a job worker without `--port`, can be health checked as long as it serves the health socket.

```bash
# HTTP server probed before the switch
dewy server --registry ghr://owner/repo --port 8000 \
  --health-path /health --health-before-switch -- /opt/app/current/app

# Job worker without --port, probed on its health socket
dewy server --registry ghr://owner/repo \
  --health-path /health --health-before-switch -- /opt/worker/current/worker
```

### --verify-checksum

Selects how a downloaded artifact is checked against the SHA-256 checksum file published next to it. Accepts `off`, `auto` or `required`. Default is `auto`.

- `auto`: verify when the registry finds a checksum file, and deploy without verification when it does not.
- `required`: additionally fail the deployment when no checksum file is published.
- `off`: never fetch a checksum file.

Verification happens before the artifact is written to the cache, so an artifact that does not match is neither extracted nor shared with other instances using the same cache backend. See [Artifact Verification](/artifact#artifact-verification) for the file layouts that are recognized.

Container images are unaffected: an image is addressed by its digest, which the container runtime verifies on pull.

```bash
# Refuse to deploy a release that publishes no checksum file
dewy server --registry ghr://owner/repo --verify-checksum required -- /opt/app/current/app
```

### --calver

Specifies the CalVer (Calendar Versioning) format for version identification. When set, Dewy uses calendar versioning instead of semantic versioning to detect the latest version.

**Format specifiers:** `YYYY`, `YY`, `0Y`, `MM`, `0M`, `WW`, `0W`, `DD`, `0D`, `MICRO`

```bash
# Four-segment CalVer format
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO -- /opt/app/current/app

# Three-segment CalVer format
dewy server --registry ghr://owner/repo --calver YYYY.0M.MICRO -- /opt/app/current/app
```

See [Versioning - Calendar Versioning](/versioning#calver) for full details.

### --interval (-i)

Specifies the interval in seconds to check the registry. Default is 10 seconds.

```bash
dewy server --interval 300 -- /opt/app/current/app
```

### --max-backoff-interval

Stretches the polling interval exponentially while checks keep failing, so that
a registry outage is not met with an unchanged request rate. The value is the
upper bound in seconds. Default is `0`, which disables the backoff and keeps the
fixed interval Dewy has always used.

```bash
dewy server --interval 10 --max-backoff-interval 300 -- /opt/app/current/app
```

A single failure changes nothing. From the second consecutive failure the wait
doubles each time (`interval`, `2x`, `4x`, ...), jittered, and never grows past
the bound given here. The first successful check restores the configured
interval immediately.

With `--interval 10 --max-backoff-interval 300`, a registry that stays down is
polled at most 10s, 20s, 40s, 80s and 160s apart, then at most 300s apart,
instead of every 10s. Jitter only ever shortens those waits, and each one lands
on a multiple of `--interval` because the scheduler keeps firing at that
cadence. The trade-off is that a release landing right after recovery can go
unnoticed for up to the bound.

A bound shorter than `--interval` has no effect, since a delay is never shorter
than one interval.

### --verbose (-v)

Enables verbose log output. Useful for debugging and troubleshooting.

```bash
dewy server --verbose -- /opt/app/current/app
```

### --version

Displays Dewy version information.

```bash
dewy --version
```

### --help (-h)

Displays help for available commands and options.

```bash
dewy --help
dewy server --help
dewy container --help
```

## Image Command Options

The `dewy container` command has specific options for container deployment management.

### --port

Specifies port mapping between the proxy and container. Can be specified multiple times for multi-port applications.

**Format:**
- `--port proxy`: Auto-detect container port from Docker image EXPOSE directive
- `--port proxy:container`: Explicit port mapping

**Auto-Detection Behavior:**
- If container port not specified, Dewy inspects the Docker image
- Single EXPOSE port → automatically used
- Multiple EXPOSE ports → error, must specify explicitly
- No EXPOSE ports → error, must specify explicitly

**Examples:**

```bash
# Auto-detect container port (container EXPOSEs port 8080)
dewy container --registry img://ghcr.io/owner/app --port 8080

# Explicit port mapping (proxy listens on 8080, forwards to container port 3000)
dewy container --registry img://ghcr.io/owner/app --port 8080:3000

# Multi-port application (HTTP + gRPC)
dewy container --registry img://ghcr.io/owner/app \
  --port 8080:80 \
  --port 9090:50051
```

### --health-path

Specifies the HTTP path for health checks. Optional in both commands.

For the `container` command, each new replica is probed at this path and traffic is switched only after it returns a successful response.

For the `server` command, the path is probed after the managed process starts or restarts, on the lowest TCP port among the configured `--port` values. On a restart, the probe is sent after the previous process has exited, so that the answer comes from the new process. A failure triggers a rollback to the previous release. The option requires at least one TCP `--port`; UDP ports are not probed, and a server running without a port (a job worker, for example) cannot be probed over HTTP. The `assets` command rejects it.

Because this probe runs after the switch, a new process that starts but does not answer (for example, one that is deadlocked) receives traffic until the probe fails and the rollback restarts the previous release. Requests sent during that time get no answer. [`--health-before-switch`](#--health-before-switch) probes each new process on a socket of its own before the switch, so the previous process keeps serving when the new one fails. With that option, `--health-path` does not require a TCP `--port`.

A response with status 2xx or 3xx counts as success. Redirects are not followed, so a 3xx is judged by its own status.

```bash
# Container: probe every new replica
dewy container --registry img://ghcr.io/owner/app --health-path /health

# Server: probe the process after a deploy and roll back on failure
dewy server --registry ghr://owner/repo --port 8000 --health-path /health -- /opt/app/current/app
```

### --health-timeout

Specifies the overall budget in seconds for one health check. The budget covers every attempt including the two-second wait between them, so raising it buys more attempts rather than a longer wait on a single request. Default is 30 seconds.

```bash
dewy container --registry img://ghcr.io/owner/app --health-timeout 60
```

### --no-rollback

Keeps a new release in place when its health check fails, instead of restoring the previous release. Applies to the `server` command.

The failed version is still recorded and is not deployed again until a different version is published, and the failure is still notified. Use this when an operator decides what to do next rather than Dewy restoring the previous release automatically.

The record names a `<tag>--<artifact>` cache key, so republishing the same tag with fixed contents does not release it. Publishing a different version, running without `--health-path`, or deleting the `blocked` entry from the cache store does.

```bash
dewy server --registry ghr://owner/repo --port 8000 --health-path /health --no-rollback -- /opt/app/current/app
```

### --drain-time

Specifies the drain time in seconds after traffic switch. Applies to the `server` and `container` commands; the `assets` command rejects it. Default is 30 seconds.

For the `container` command, the old container is sent SIGTERM once it is removed from the proxy, and is given up to this long to finish in-flight requests before being killed.

For the `server` command, the previous process is sent SIGTERM once the new process has replaced it, and is killed with SIGKILL if it is still running after this time. The same limit applies when Dewy shuts down: Dewy sends SIGTERM to every process, and kills the ones still running after this time.

```bash
dewy container --registry img://ghcr.io/owner/app --drain-time 60
dewy server --registry ghr://owner/repo --port 8000 --drain-time 60 -- /opt/app/current/app
```

### --runtime

Specifies the container runtime to use. Supports `docker` or `podman`. Default is `docker`.

```bash
dewy container --registry img://ghcr.io/owner/app --runtime podman
```

### --admin-port

Specifies the admin API port for the container command. Default is 17539. If the port is already in use, Dewy automatically increments the port number. The admin API is used by the `dewy container list` command to query container information.

```bash
# Use custom admin port
dewy container --registry img://ghcr.io/owner/app --admin-port 20000

# Default port (17539) - auto-increments if occupied
dewy container --registry img://ghcr.io/owner/app
```

**Note:** The `dewy container list` command automatically scans ports 17539-17548 to find running instances, so you typically don't need to specify this option unless you have specific port requirements.

### --cmd

Specifies the command and arguments to pass to the container. Can be specified multiple times. This overrides the container image's default CMD.

```bash
dewy container --registry img://ghcr.io/owner/app \
  --cmd "/bin/sh" \
  --cmd "-c" \
  --cmd "node server.js --debug"
```

### -- (Separator)

The `--` separator allows you to pass additional docker run options directly. All arguments after `--` are passed to the docker run command.

**Supported options:** Environment variables (`-e`), volumes (`-v`), resource limits (`--cpus`, `--memory`), entrypoint (`--entrypoint`), and most other docker run options.

**Forbidden options:** `-d`, `-i`, `-t`, `-it`, `-p`, `--label-file` (conflict with Dewy's management); `--privileged`, `--pid`, `--cap-add`, `--security-opt`, `--device`, `--userns`, `--cgroupns` (security-sensitive). The `dewy.` label prefix is also reserved for internal container tracking and cannot be used with `-l` / `--label`.

**Default user:** When `--user` or `-u` is not specified in extra args, Dewy automatically adds `--user UID:GID` using the UID and GID of the user running Dewy. This prevents containers from accidentally running as root. If your container requires root privileges, explicitly specify `--user=0:0`.

**Custom container name:** You can specify `--name` to customize the container base name. Dewy automatically appends a timestamp and replica index to ensure uniqueness.

```bash
# Environment variables and volumes
dewy container --registry img://ghcr.io/owner/app -- \
  -e API_KEY=secret \
  -e DATABASE_URL=postgres://localhost/db \
  -v /data:/app/data \
  -v /config:/app/config:ro

# Resource limits and custom entrypoint
dewy container --registry img://ghcr.io/owner/app -- \
  --cpus 2 \
  --memory 1g \
  --entrypoint /custom/entrypoint

# Custom container name (will be suffixed with timestamp and replica index)
dewy container --registry img://ghcr.io/owner/app --replicas 3 -- \
  --name myapp
# Results in: myapp-1234567890-0, myapp-1234567890-1, myapp-1234567890-2
```

## Registry URL Formats

Dewy supports multiple registry types, each using different URL formats.

### GitHub Releases

Registry URL format: `ghr://`

Used to retrieve version information from GitHub Releases. Supports both public and private repositories.

```bash
ghr://owner/repository
ghr://owner/repository?pre-release=true

# With CalVer
dewy server --registry ghr://owner/repository --calver YYYY.0M.0D.MICRO
```

**Authentication:**

Dewy supports two authentication methods:

1. **Personal Access Token (PAT)**: Set `GITHUB_TOKEN` or `GH_TOKEN` environment variable
2. **GitHub App** (recommended for production): Set `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, and `GITHUB_APP_PRIVATE_KEY` or `GITHUB_APP_PRIVATE_KEY_PATH`

See [Registry documentation](/registry#authentication) for detailed setup instructions.

### Amazon S3

Registry URL format: `s3://`

Retrieves version information from Amazon S3 bucket. AWS credentials are required.

```bash
s3://region/bucket/prefix
s3://region/bucket/prefix?pre-release=true
```

### Google Cloud Storage

Registry URL format: `gs://`

Retrieves version information from Google Cloud Storage bucket. GCP credentials are required.

```bash
gs://bucket/prefix
gs://bucket/prefix?pre-release=true
```

### OCI Registry

Registry URL format: `img://`

Retrieves version information from OCI-compatible container registries. Supports Docker Hub, GitHub Container Registry (GHCR), Google Container Registry (GCR), Amazon ECR, and other OCI registries.

```bash
# Docker Hub
img://namespace/repository
img://namespace/repository?pre-release=true

# GitHub Container Registry
img://ghcr.io/owner/repository

# Google Container Registry
img://gcr.io/project/repository

# Amazon ECR
img://account-id.dkr.ecr.region.amazonaws.com/repository
```

## Notification Formats

Dewy supports various notification channels. Deployment success and failure can be notified to appropriate destinations.

### Slack

Sends notifications using Slack Incoming Webhook or Bot Token. Channel specification is also possible.

```bash
slack://webhook-url
slack://token@channel
```

### Email (SMTP)

Sends email notifications through SMTP server. Authentication credentials and server settings are required.

```bash
smtp://user:password@host:port/to@example.com
```

## Environment Variables {% #environment-variables %}

Dewy reads the variables in the first group to authenticate and to locate services, and sets the variables in the second group for the application it runs. Each variable is described in detail on the linked page.

### Variables Dewy Reads {% #variables-dewy-reads %}

| Variable | Used by | Description |
|----------|---------|-------------|
| `GH_TOKEN` | `ghr://` | GitHub personal access token. Takes precedence over `GITHUB_TOKEN`, so a token set by GitHub Actions does not replace it. See [Registry](/registry) |
| `GITHUB_TOKEN` | `ghr://`, `img://ghcr.io/...` | GitHub personal access token, used when `GH_TOKEN` is not set. For GitHub Container Registry it is the password |
| `GITHUB_APP_ID` | `ghr://` | GitHub App ID. When it is set, GitHub App authentication is used instead of a token |
| `GITHUB_APP_INSTALLATION_ID` | `ghr://` | GitHub App installation ID. Required with `GITHUB_APP_ID` |
| `GITHUB_APP_PRIVATE_KEY` | `ghr://` | GitHub App private key in PEM format. Takes precedence over `GITHUB_APP_PRIVATE_KEY_PATH` |
| `GITHUB_APP_PRIVATE_KEY_PATH` | `ghr://` | Path to the GitHub App private key file |
| `GITHUB_API_URL` | `ghr://` | API endpoint of GitHub Enterprise Server (for example, `https://github.example.com/api/v3/`) |
| `GITHUB_ENDPOINT` | `ghr://` | Same as `GITHUB_API_URL`, used when `GITHUB_API_URL` is not set |
| `GITHUB_ARTIFACT` | `ghr://` | Artifact name, used when the registry URL has no `artifact` parameter. Kept for compatibility; prefer the URL parameter |
| `AWS_ENDPOINT_URL` | `s3://` registry, artifact and cache | Endpoint of an S3-compatible service, used when the URL has no `endpoint` parameter. Path-style addressing is used with it |
| `DOCKER_USERNAME`, `DOCKER_PASSWORD` | `img://`, container command | Username and password for any OCI registry. `DOCKER_USERNAME` takes precedence over the registry-specific variables below |
| `AWS_ECR_PASSWORD` | `img://`, container command | Token for Amazon ECR, as printed by `aws ecr get-login-password` |
| `GCR_TOKEN` | `img://`, container command | Service account key JSON for Google Container Registry and Artifact Registry |
| `SLACK_TOKEN` | `slack://` | Slack bot token. Required for Slack notifications. See [Notifier](/notifier) |
| `MAIL_USERNAME`, `MAIL_PASSWORD`, `MAIL_FROM` | `smtp://` | SMTP username, password and sender address, used for the values the notifier URL does not give |
| `DEWY_CACHEDIR` | `file://` cache | Directory of the local file cache. Defaults to `.dewy/cache` under the working directory. See [Cache](/cache) |

S3 and GCS also read the standard credentials of each SDK, such as `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_PROFILE` and `GOOGLE_APPLICATION_CREDENTIALS`. Dewy does not interpret them itself.

### Variables Dewy Sets for the Application {% #variables-dewy-sets %}

The `server` command sets these variables in the environment of the application process, in addition to the environment Dewy itself runs with. A Go application can read them with the `github.com/linyows/dewy/sock` module; see [Application Integration](/application#inherited-sockets).

| Variable | Description |
|----------|-------------|
| `DEWY_PORT` | The sockets passed to the application, one `<address>=<fd>` entry per `--port`, joined by `;` (for example, `8080=3;53/udp=4`). Set only when `--port` is given. See [Application Integration](/application#inherited-sockets) |
| `SERVER_STARTER_PORT` | The same value as `DEWY_PORT`, for applications written for server-starter |
| `DEWY_HEALTH_PORT` | The health check socket of this process in the `<address>=<fd>` form (for example, `127.0.0.1:41234=5`). Set only with `--health-before-switch`. See [Application Integration](/application#health-checks) |

## Exit Codes

Dewy uses the following exit codes to indicate execution results. These can be used for processing branches in scripts and CI/CD pipelines.

### Normal Exit (0)

Returned when the command completes normally. All processes executed as expected.

### Configuration Error (1)

Returned when there are problems with command line options or configuration files. Option verification or configuration review is required.

### Network Error (2)

Returned when connection to registry or artifacts fails. Check network connectivity and authentication credentials.

### Filesystem Error (3)

Returned when file read/write or directory access fails. Check permissions and disk space.

### Application Error (4)

Returned when the launched application terminates abnormally. Check application logs.

## Usage Examples

Here are common usage patterns for Dewy. Adjust settings according to your actual environment.

### Basic Usage Example

The simplest configuration to start Dewy. Monitors GitHub Releases and deploys applications.

```bash
dewy server \
  --registry ghr://owner/repo \
  --port 8080 \
  -- /opt/app/current/myapp
```

### Complete Configuration Example

Comprehensive configuration example specifying all major options. Suitable for production environment use.

```bash
dewy server \
  --registry ghr://mycompany/myapp \
  --artifact s3://mybucket/artifacts/ \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --notifier slack://hooks.slack.com/services/xxx/yyy/zzz \
  --port 8080 \
  --interval 300 \
  --keeptime 86400 \
  --timezone Asia/Tokyo \
  --user app-user \
  --group app-group \
  --workdir /opt/app/data \
  --verbose \
  -- /opt/app/current/myapp --config /opt/app/config/app.conf
```

### Artifact Information Check Example

Example to check the current artifact status. Can be used to understand deployment situation.

```bash
dewy assets --registry ghr://mycompany/myapp --verbose
```

### Development Environment Example

Configuration example for checking at short intervals in development environment. Suitable for environments requiring frequent updates.

```bash
dewy server \
  --registry ghr://mycompany/myapp-dev \
  --interval 60 \
  --port 8080 \
  --verbose \
  -- /opt/app/dev/myapp --env development
```

### Container Image Deployment Example

Example deploying container images with rolling update deployment strategy. Monitors OCI registry for new versions.

```bash
# Authenticate for private registry
docker login ghcr.io

# Basic deployment with health checks (auto-detect container port)
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --log-level info \
  -- \
  -e DATABASE_URL=postgres://db:5432/mydb \
  -v /data:/app/data

# Explicit port mapping (proxy:container)
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080:3000 \
  --health-path /health \
  --log-level info

# Multi-port application (HTTP + gRPC)
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080:80 \
  --port 9090:50051 \
  --health-path /health \
  --replicas 3

# Multiple replicas with custom command
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  --replicas 3 \
  --cmd "node" \
  --cmd "server.js" \
  --cmd "--workers=4" \
  -- \
  -e NODE_ENV=production \
  --cpus 2 \
  --memory 2g

# Custom container name and resource limits
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  -- \
  --name custom-app \
  -e API_KEY=secret \
  --cpus 4 \
  --memory 4g \
  --restart unless-stopped
```
