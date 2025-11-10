---
title: Introduction
description: Introduction to Dewy
---

# What is Dewy? {% #overview %}

Dewy is software for declaratively deploying applications in both container and non-container environments.
It ensures that applications and data on servers are always up-to-date.

Dewy supports multiple deployment modes:
- **Binary deployment**: Deploy Go applications as single binaries (non-container environments)
- **Asset deployment**: Deploy static files (HTML, CSS, JavaScript, etc.)
- **Container image deployment**: Deploy container images with zero-downtime Blue-Green deployment

## Background

Go can compile code into a single binary tailored to each environment. In distributed systems with orchestrators like Kubernetes, there are no issues deploying Go applications. However, there doesn't seem to be a clear answer for how to deploy Go binaries in non-container single physical host or virtual machine environments. There are various methods: writing shell scripts that use scp or rsync from your local machine, using Ansible for server configuration management, or using Ruby's Capistrano. However, considering audit logs and information sharing about who deployed what where in multi-person teams, there doesn't seem to be a tool that matches those use cases.

## Key Features

- **Declarative pull-based deployment**: Applications automatically stay up-to-date
- **Multiple deployment modes**: Binary, assets, and container images
- **Zero-downtime deployment**: Blue-Green deployment for container images with automatic health checks
- **Graceful restarts**: Smooth application updates without dropping connections
- **Multiple registry support**: GitHub Releases, S3, GCS, Docker Hub, GitHub Container Registry, Google Artifact Registry, AWS ECR
- **Deployment notifications**: Slack, email, and other notification channels
- **Structured logging**: JSON format support for easy log aggregation
- **Audit logs**: Track who deployed what, when

## Use Cases Where Dewy is Helpful

Dewy is optimal for keeping the latest version of applications running in various environments:

### Binary Deployment
- Deploying Go applications as single binaries on virtual machines or physical servers
- Managing applications in non-container environments where Kubernetes is overkill
- Maintaining simple server applications with minimal infrastructure

### Container Image Deployment
- Zero-downtime deployments of containerized applications without Kubernetes
- Blue-Green deployments on Docker/Podman environments
- Automatic updates of container images from OCI registries
- Ideal for single-host Docker environments or simple multi-container setups

## Next Steps

To start using Dewy, refer to the following documentation:

- [Installation](../installation/)
- [Getting Started](../getting-started/)
- [Architecture](../architecture/)