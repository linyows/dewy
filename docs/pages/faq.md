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

Point the instances at one shared cache prefix on S3 or Google Cloud Storage and add `registry-ttl` to the cache URL:

```sh
dewy server --registry ghr://owner/repo \
  --cache 's3://ap-northeast-1/mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp
```

One instance per TTL window polls the upstream registry; the rest read the response from the shared cache. Polling is what consumes a rate limit, because it happens every interval per instance whether or not there is a new release, so this is the setting that matters. See [Registry result cache](/cache#registry-result-cache).

Lengthening the polling interval with `--interval` also helps, and the two combine.

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