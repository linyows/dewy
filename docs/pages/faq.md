---
title: Frequently Asked Questions
---

# {% $markdoc.frontmatter.title %}

A collection of frequently asked questions.

## What happens if I delete the latest version from the registry?

Dewy will change to the latest version after deletion. While deleting or overwriting released versions is not desirable, there may be cases where deletion is unavoidable due to security issues or other reasons.

## Where are the audit logs?

Dewy records each deployment in the registry it deployed from. The record names the host, the command and the time, for example `shipped_to_web-1_server_at_20261122T083000.123456789Z.txt`.

| Registry | Where the record is kept |
|---|---|
| GitHub Releases | A text file with that name, uploaded as an asset of the deployed release |
| S3 / GCS | An empty object with that name under `<prefix><tag>/` |
| gRPC | Sent to the gRPC server with the `Report` call; the server decides how to store it |
| OCI | Not recorded |

The record is kept next to the release it refers to, so the hosts that run a version can be listed from that release. Listing deployments across releases means reading the assets or objects of every release; to keep the history in a database of your own, use the [gRPC registry](/registry#grpc), whose server receives every record. Recording on GitHub Releases needs write permission on the repository contents; see [Registry](/registry#github-releases).

[Telemetry](/telemetry) does not replace these records. Dewy exports OpenTelemetry metrics, such as `dewy.deployments.total` and `dewy.deployments.rollbacks.total`, which count deployments for each command and suit dashboards and alerts, but a metric does not identify the version or the host of each deployment. To receive each deployment as an event outside the registry, use [notifications](/notifier), or collect the [JSON logs](/structured-logging) with a log platform.

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

## Why does my application not start under the server command?

Dewy reports a start that failed with an error such as `worker exited during the 1s startup window: exit status 1`. The application exited before `--startup-wait` (1 second by default) passed. The common causes are:

- **The application binds the port itself.** Dewy has already opened the `--port` ports, so a second bind fails with "address already in use". Serve on the socket that dewy passes instead; see [Application Integration for the Server Command](/application#inherited-sockets).
- **The application runs in the background.** An application that forks into the background, for example with a daemon option, ends the process that dewy started. Run it in the foreground.
- **The application fails while it starts**, for example while it reads its configuration or connects to a database. Dewy passes the application's stdout and stderr through to its own, so the reason appears in the same output as dewy's logs. If it only needs longer to become ready, set `--startup-wait` longer than that.

## Why does my application start but not answer requests?

The application is most likely serving on a socket other than the ones dewy passes, for example on a port written in its configuration. Connections to `--port` are accepted by dewy's socket and wait there, because no process reads from it, so clients hang instead of being refused. With `--health-path`, the health check fails after `--health-timeout`. Serve on the inherited sockets; see [Application Integration for the Server Command](/application#inherited-sockets).

With `--health-before-switch`, every deployment also fails when the application does not serve its health endpoint on the `DEWY_HEALTH_PORT` socket; see [Health Checks](/application#health-checks).

## Why does the previous version keep running after a deployment?

The process that dewy started has probably exited while the application kept running. This happens when the command is a shell script that starts the application as a child process: dewy sends SIGTERM to the shell only, the shell exits, and the application continues to run with the inherited sockets, serving the previous version next to the new one. Start the application with `exec` in the script, so that the application replaces the shell and receives the signals itself:

```sh
#!/bin/sh
export APP_ENV=production
exec /opt/myapp/current/myapp
```

## Why does each deployment take 30 seconds to stop the previous process?

The previous process does not exit on SIGTERM. Dewy waits for `--drain-time` (30 seconds by default) and then kills it with SIGKILL, which also cuts off the requests in progress. Handle SIGTERM by finishing the requests in progress and exiting; see [Signal Handling](/signal-handling).

## Next Steps

For more information and detailed documentation, refer to the following resources:

- [Getting Started](../getting-started)
- [Architecture](../architecture)
- [Contributing](../contributing)