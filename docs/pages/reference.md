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

Specifies artifact cache settings. Local filesystem or Redis can be used as cache storage.

```bash
dewy server --cache file:///tmp/dewy-cache -- /opt/app/current/app
```

### --notifier (-n)

Configures deployment status notifications. Notification channels such as Slack and email can be set up.

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

Specifies the port(s) used by the application server. This option is optional for the `server` command.

When specified, the server-starter manages the application with the given port(s). When omitted, the application runs without port listening (useful for job workers, message queue consumers, etc.).

```bash
# With port (HTTP server)
dewy server --port 9090 -- /opt/app/current/app

# Without port (job worker, etc.)
dewy server --registry ghr://owner/repo -- /opt/worker/current/worker
```

### --interval (-i)

Specifies the interval in seconds to check the registry. Default is 600 seconds (10 minutes).

```bash
dewy server --interval 300 -- /opt/app/current/app
```

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

Specifies the HTTP path for health checks. If specified, Dewy will wait for this endpoint to return a successful response before switching traffic. Optional.

```bash
dewy container --registry img://ghcr.io/owner/app --health-path /health
```

### --health-timeout

Specifies the timeout in seconds for health checks. Default is 30 seconds.

```bash
dewy container --registry img://ghcr.io/owner/app --health-timeout 60
```

### --drain-time

Specifies the drain time in seconds after traffic switch. The old container remains running during this period to complete in-flight requests. Default is 30 seconds.

```bash
dewy container --registry img://ghcr.io/owner/app --drain-time 60
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

**Forbidden options:** `-d`, `-it`, `-i`, `-t`, `-l`, `-p` (these conflict with Dewy's management)

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
```

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
  --cache redis://localhost:6379/0 \
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
