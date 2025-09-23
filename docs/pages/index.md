---
title: Dewy
description: Dewy enables declarative deployment of applications in non-Kubernetes environments.
layout: landing
---

# What is Dewy?

Dewy is software for declaratively deploying applications primarily built with Go in non-container environments.
It ensures that applications and data on servers are always up-to-date.

## Key Features

- Declarative pull-based deployment
- Graceful restarts
- Selectable registry and artifact stores
- Deployment status notifications
- Structured logging with JSON format support
- Audit logs

## Use Cases Where Dewy is Helpful

Dewy is optimal for keeping the latest version of applications running in mutable server environments such as hypervisor-type virtual servers and physical servers.

## Quick Start

```bash
# Install Dewy
curl -L https://github.com/linyows/dewy/releases/latest/download/dewy_linux_amd64.tar.gz | tar xz

# Start deployment
dewy server --registry ghr://owner/repo --port 8080 -- /opt/app/current/app
```

## Architecture

Dewy acts as a supervisor for applications, serving as the main process and launching applications as child processes. It's composed of four interfaces as pluggable abstractions: Registry, Artifact, Cache, and Notifier.

{% callout %}
Ready to get started? Check out our [Installation Guide](/installation) or explore the [Architecture](/architecture) to understand how Dewy works.
{% /callout %}