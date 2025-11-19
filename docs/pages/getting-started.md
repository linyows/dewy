---
title: Getting Started
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Let's try deploying an application using Dewy. This article explains the basic usage and actual deployment process step by step.

## Prerequisites

- Dewy is installed (see [Installation Guide](/installation))
- For binary/asset deployment: Go application or assets are published on GitHub Releases, S3, or GCS
- For container deployment: Docker or Podman is installed and running
- Required environment variables are configured (GitHub token, Docker credentials, etc.)

## Basic Usage

### Server Application Deployment

Example of automatically deploying a server application from GitHub Releases:

```bash
# Set environment variables
export GITHUB_TOKEN=your_github_token

# Start server application
dewy server --registry ghr://owner/repo --port 8000 -- /opt/myapp/current/myapp
```

In this example:
- `ghr://owner/repo`: GitHub Releases registry URL
- `--port 8000`: Port used by the application
- `/opt/myapp/current/myapp`: Path to the application to execute

### Static Asset Deployment

For deploying static files like HTML, CSS, and JavaScript files:

```bash
dewy assets --registry ghr://owner/frontend-assets
```

### Container Image Deployment

For deploying containerized applications with zero-downtime rolling update deployment:

```bash
# Authenticate if using private registry
docker login ghcr.io

# Deploy container image from OCI registry (auto-detect container port)
dewy container --registry img://ghcr.io/owner/app --port 8080

# Or specify explicit port mapping (proxy:container)
dewy container --registry img://ghcr.io/owner/app --port 8080:3000
```

In this example:
- `img://ghcr.io/owner/app`: OCI registry URL (supports Docker Hub, GHCR, GCR, ECR, etc.)
- `--port 8080`: Proxy listens on port 8080, container port auto-detected from Docker image
- `--port 8080:3000`: Proxy listens on 8080, forwards to container port 3000
- Health check path can be specified with `--health-path /health` (optional)

Dewy automatically:
- Performs rolling update deployment with zero downtime
- Switches traffic to the new container after health checks pass
- Removes the old container after the drain period

## Actual Deployment Example

### Example Using GitHub Releases

```bash
# Configure GitHub Personal Access Token
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx

# Create application directory
sudo mkdir -p /opt/myapp
sudo chown $USER:$USER /opt/myapp
cd /opt/myapp

# Start Dewy to deploy server application
dewy server \
  --registry ghr://myorg/myapp \
  --port 8080 \
  --log-level info \
  -- /opt/myapp/current/myapp
```

### Example Using OCI Registry

```bash
# Authenticate if using private registry
docker login ghcr.io

# Ensure Docker/Podman is running
docker info

# Deploy container image with rolling update deployment
dewy container \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080:3000 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --log-level info
```

In this example:
- Dewy polls the OCI registry for new image versions
- When a new version is detected, it pulls the image and starts a new container
- The new container is health-checked (if `--health-path` is specified)
- Traffic is switched to the new container by the proxy
- The old container continues running during the drain period, then is removed
- The process repeats automatically when new versions are published

The proxy listens on the specified port and forwards traffic to the current container. You can expose this port through a reverse proxy like nginx or Caddy.

## Next Steps

For more information, refer to the following documentation:

- [Architecture](../architecture/)
- [Dewy CLI Reference](../reference/)
- [FAQ](../faq/)