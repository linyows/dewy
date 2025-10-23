---
title: Introduction
description: Introduction to Dewy
---

# What is Dewy? {% #overview %}

Dewy is software for declaratively deploying applications primarily built with Go in non-container environments.
It ensures that applications and data on servers are always up-to-date.

## Background

Go can compile code into a single binary tailored to each environment. In distributed systems with orchestrators like Kubernetes, there are no issues deploying Go applications. However, there doesn't seem to be a clear answer for how to deploy Go binaries in non-container single physical host or virtual machine environments. There are various methods: writing shell scripts that use scp or rsync from your local machine, using Ansible for server configuration management, or using Ruby's Capistrano. However, considering audit logs and information sharing about who deployed what where in multi-person teams, there doesn't seem to be a tool that matches those use cases.

## Key Features

- Declarative pull-based deployment
- Graceful restarts
- Selectable registry and artifact stores
- Deployment status notifications
- Structured logging with JSON format support
- Audit logs

## Use Cases Where Dewy is Helpful

Dewy is optimal for keeping the latest version of applications running in mutable server environments such as hypervisor-type virtual servers and physical servers.

## Next Steps

To start using Dewy, refer to the following documentation:

- [Installation](../installation/)
- [Getting Started](../getting-started/)
- [Architecture](../architecture/)