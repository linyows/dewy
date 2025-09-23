---
title: Getting Started
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Let's try deploying an application using Dewy. This article explains the basic usage and actual deployment process step by step.

## Prerequisites

- Dewy is installed (see [Installation Guide](/installation))
- Go application you want to deploy is published on GitHub Releases
- Required environment variables are configured

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