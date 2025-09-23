---
title: Architecture
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy acts as a supervisor for applications, serving as the main process and launching applications as child processes.

## Interfaces

Dewy is composed of four interfaces as pluggable abstractions: Registry, Artifact, Cache, and Notifier.

- Registry: Version management (GitHub Releases, S3, GCS, gRPC)
- Artifact: Binary acquisition (corresponding Registry formats)
- Cache: Downloaded file management (File, Memory, Consul, Redis)
- Notifier: Deployment notifications (Slack, Mail)

## Deployment Process

The following diagram illustrates Dewy's deployment process and configuration.

![Hi-level Architecture](https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true)

1. Registry polls the specified registry to detect the latest version of the application
1. Artifact downloads and extracts artifacts from the specified artifact store
1. Cache stores the latest version and artifacts
1. Dewy creates child processes for new version applications and begins processing requests with the new application
1. Registry saves information about when, where, and what was deployed as files
1. Notifier sends notifications to specified notification destinations

In this way, Dewy communicates with a central registry to achieve pull-based deployment.