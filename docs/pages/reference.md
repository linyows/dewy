---
title: Dewy CLI Reference
description: Complete reference guide for Dewy CLI commands and options
---

# Dewy CLI Reference

This page provides detailed information about Dewy CLI commands, options, environment variables, and usage examples.

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

The `dewy container` command handles container image deployment with zero-downtime Blue-Green deployment strategy. It monitors OCI registries for new image versions and automatically deploys them.

```bash
dewy container [options]
```

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

Configures deployment status notifications. Notification channels such as Slack, Discord, and email can be set up.

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

Specifies the port used by Dewy's HTTP server. Default is 8080.

```bash
dewy server --port 9090 -- /opt/app/current/app
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

### --container-port

Specifies the port that the container listens on. Default is 8080. This is used for health checks and traffic routing.

```bash
dewy container --registry img://ghcr.io/owner/app --container-port 3000
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

### --env (-e)

Specifies environment variables to pass to the container. Can be specified multiple times.

```bash
dewy container --registry img://ghcr.io/owner/app \
  --env API_KEY=secret \
  --env DATABASE_URL=postgres://localhost/db
```

### --volume

Specifies volume mounts for the container. Format is `host:container` or `host:container:ro` for read-only. Can be specified multiple times.

```bash
dewy container --registry img://ghcr.io/owner/app \
  --volume /data:/app/data \
  --volume /config:/app/config:ro
```

## Environment Variables

Dewy can use the following environment variables to customize behavior. Environment variables have lower priority than command line options.

### DEWY_REGISTRY

Sets the default registry URL. Has the same effect as the `--registry` option.

```bash
export DEWY_REGISTRY=ghr://owner/repo
```

### DEWY_ARTIFACT

Sets the default artifact URL. Has the same effect as the `--artifact` option.

```bash
export DEWY_ARTIFACT=s3://bucket/path/to/artifact
```

### DEWY_CACHE

Specifies the default cache settings. Has the same effect as the `--cache` option.

```bash
export DEWY_CACHE=file:///tmp/dewy-cache
```

### DEWY_NOTIFIER

Specifies the default notification settings. Has the same effect as the `--notifier` option.

```bash
export DEWY_NOTIFIER=slack://webhook-url
```

### DEWY_PORT

Sets the Dewy HTTP server port. Has the same effect as the `--port` option.

```bash
export DEWY_PORT=8080
```

### DEWY_INTERVAL

Sets the registry check interval. Has the same effect as the `--interval` option.

```bash
export DEWY_INTERVAL=600
```

## Registry URL Formats

Dewy supports multiple registry types, each using different URL formats.

### GitHub Releases (ghr://)

Used to retrieve version information from GitHub Releases. Supports both public and private repositories.

```bash
ghr://owner/repository
ghr://owner/repository#tag-pattern
```

### Docker Hub (dockerhub://)

Retrieves version information from Docker Hub image tags. Can also be used with containerized applications.

```bash
dockerhub://namespace/repository
dockerhub://namespace/repository:tag-pattern
```

### Amazon ECR (ecr://)

Retrieves version information from Amazon Elastic Container Registry. AWS credentials are required.

```bash
ecr://region/repository
ecr://account-id.dkr.ecr.region.amazonaws.com/repository
```

### Git (git://)

Retrieves version information from Git repository tags. Supports SSH and HTTPS authentication.

```bash
git://github.com/owner/repository
git://gitlab.com/owner/repository
```

## Notification Formats

Dewy supports various notification channels. Deployment success and failure can be notified to appropriate destinations.

### Slack

Sends notifications using Slack Incoming Webhook or Bot Token. Channel specification is also possible.

```bash
slack://webhook-url
slack://token@channel
```

### Discord

Sends notifications using Discord Webhook or Bot Token.

```bash
discord://webhook-url
discord://token@channel-id
```

### Microsoft Teams

Sends notifications using Microsoft Teams Incoming Webhook.

```bash
teams://webhook-url
```

### Email (SMTP)

Sends email notifications through SMTP server. Authentication credentials and server settings are required.

```bash
smtp://user:password@host:port/to@example.com
```

### HTTP/HTTPS

POSTs notifications to custom HTTP endpoints. Supports webhook-style notifications.

```bash
http://your-webhook-endpoint
https://your-webhook-endpoint
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

### Environment Variables Example

Example using environment variables to keep the command line concise. This approach has good compatibility with Docker environments and configuration management tools.

```bash
export DEWY_REGISTRY=ghr://mycompany/myapp
export DEWY_ARTIFACT=s3://mybucket/artifacts/
export DEWY_CACHE=file:///tmp/dewy-cache
export DEWY_NOTIFIER=slack://hooks.slack.com/services/xxx/yyy/zzz
export DEWY_PORT=8080
export DEWY_INTERVAL=300
export DEWY_TIMEZONE=Asia/Tokyo

dewy server -- /opt/app/current/myapp
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

Example deploying container images with Blue-Green deployment strategy. Monitors OCI registry for new versions.

```bash
# Set credentials for private registry
export DOCKER_USERNAME=myusername
export DOCKER_PASSWORD=mypassword

# Deploy with health checks
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --container-port 8080 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --env DATABASE_URL=postgres://db:5432/mydb \
  --volume /data:/app/data \
  --log-level info
```

### Container Deployment with Custom Network

Example using custom Docker network and network alias for service discovery.

```bash
dewy container \
  --registry img://ghcr.io/mycompany/api \
  --network production-net \
  --network-alias api-service \
  --container-port 3000 \
  --interval 300 \
  --log-level info
```

### Container Deployment with Reverse Proxy

Example using Caddy reverse proxy for container deployment. The proxy handles external traffic while application containers remain on the internal network only.

```bash
dewy container \
  --registry img://ghcr.io/mycompany/webapp \
  --container-port 3333 \
  --health-path /health \
  --proxy \
  --proxy-port 8000 \
  --env API_KEY=secret \
  --log-level info
```

In this setup:
- External clients access the application through `http://localhost:8000`
- Caddy proxy forwards requests to `dewy-current:3333` (internal network)
- Application container port 3333 is not exposed externally
- Both proxy and application containers are managed by dewy
- All containers are cleaned up automatically on shutdown
