---
title: Frequently Asked Questions
---

# {% $markdoc.frontmatter.title %}

A collection of frequently asked questions.

## What happens if I delete the latest version from the registry?

Dewy will change to the latest version after deletion. While deleting or overwriting released versions is not desirable, there may be cases where deletion is unavoidable due to security issues or other reasons.

## Where are the audit logs?

Audit logs are saved as text file names where artifacts are hosted. Currently there is no searchability. If I think of a good method, I will change it. Separately from auditing, it may also be necessary to send notifications to observability products like OTEL.

## How can I handle registry rate limits caused by polling from multiple Dewy instances?

Using HashiCorp Consul or Redis for cache components allows multiple Dewy instances to share cache, which should reduce the total number of requests to the registry. In that case, it would be good to set the registry TTL to an appropriate time. Note that you can specify longer polling intervals using command options.

## How can I run multiple Dewy instances on the same host?

Dewy creates cache files in the current working directory (cwd). To run multiple Dewy instances on the same host, run each instance from a different directory. This ensures that each instance maintains its own cache and state files without conflicts.

For example:

```bash
# First instance
mkdir -p /opt/app1 && cd /opt/app1
dewy server --registry ghr://owner/repo1 --port 8001 -- /opt/app1/current/app

# Second instance
mkdir -p /opt/app2 && cd /opt/app2
dewy server --registry ghr://owner/repo2 --port 8002 -- /opt/app2/current/app
```

## Next Steps

For more information and detailed documentation, refer to the following resources:

- [Getting Started](../getting-started)
- [Architecture](../architecture)
- [Contributing](../contributing)