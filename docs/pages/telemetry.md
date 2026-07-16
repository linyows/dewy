---
title: Telemetry
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy provides built-in telemetry support based on OpenTelemetry (OTel) for monitoring proxy performance, deployment activity, and container health. Telemetry is particularly valuable in **container mode**, where Dewy operates as an independent TCP proxy rather than as part of the application process.

## Architecture

In **server mode**, Dewy runs as part of the deployed application, so telemetry can be collected through the application's own OTel SDK (e.g., via otel-collector configured with systemd).

In **container mode**, Dewy operates as a standalone reverse proxy managing container lifecycle. Since it is separate from the application, Dewy needs its own telemetry pipeline. Dewy uses the OpenTelemetry SDK internally and supports two export paths:

- **Prometheus exporter**: Exposes a `/metrics` endpoint on the Admin API for scraping
- **OTLP exporter**: Sends metrics to an OpenTelemetry Collector via gRPC

Both exporters can be used simultaneously, allowing you to choose the best approach for your infrastructure.

```
┌──────────────────────────────────────────────────────┐
│  Dewy (container mode)                               │
│                                                      │
│  ┌──────────────┐   ┌────────────────────────────┐   │
│  │  TCP Proxy   │──▶│  OTel SDK (MeterProvider)  │   │
│  │  Deploy Mgr  │   │                            │   │
│  │  Health Check│   │  ┌──────────────────────┐  │   │
│  └──────────────┘   │  │ Prometheus Exporter  │──┼───┼──▶ GET /metrics (Admin API)
│                     │  └──────────────────────┘  │   │
│                     │  ┌──────────────────────┐  │   │
│                     │  │   OTLP Exporter      │──┼───┼──▶ OTel Collector (gRPC)
│                     │  └──────────────────────┘  │   │
│                     └────────────────────────────┘   │
└──────────────────────────────────────────────────────┘
```

## Enabling Telemetry

Telemetry is disabled by default. Enable it with the `--telemetry` flag or by specifying an `--otlp-endpoint`.

### Prometheus Only

Expose a `/metrics` endpoint on the Admin API server. Prometheus can then scrape this endpoint.

```bash
dewy container --telemetry \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080 --health-path /health
```

The metrics endpoint is available at `http://localhost:17539/metrics` (the Admin API port).

### Prometheus + OTLP

In addition to the Prometheus endpoint, send metrics to an OpenTelemetry Collector via gRPC.

```bash
dewy container --telemetry \
  --otlp-endpoint localhost:4317 \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080 --health-path /health
```

When `--otlp-endpoint` is specified, telemetry is automatically enabled even without the `--telemetry` flag.

### OTLP Only

If you only want OTLP export without the Prometheus endpoint, specify the endpoint:

```bash
dewy container --otlp-endpoint otel-collector.internal:4317 \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080
```

Note: The Prometheus exporter is always registered when telemetry is enabled. The `/metrics` endpoint will be available regardless, but you can simply choose not to scrape it.

## Metrics Reference

All metrics use the `dewy.` prefix and follow OpenTelemetry semantic conventions.

### Proxy Metrics

These metrics are recorded for each TCP proxy connection and are labeled with `proxy_port`.

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `dewy.proxy.connections.total` | Counter | {connection} | Total number of proxy connections accepted |
| `dewy.proxy.connections.active` | UpDownCounter | {connection} | Number of currently active proxy connections |
| `dewy.proxy.connection.duration` | Histogram | s | Duration of proxy connections (from accept to close) |
| `dewy.proxy.connect.latency` | Histogram | s | Time to establish connection to a backend |
| `dewy.proxy.bytes.transferred` | Counter | By | Total bytes transferred through the proxy (both directions) |
| `dewy.proxy.errors.total` | Counter | {error} | Total number of proxy errors (no backend, connection failures) |
| `dewy.proxy.backends` | UpDownCounter | {backend} | Number of active proxy backends |

The `dewy.proxy.connect.latency` histogram uses the following bucket boundaries optimized for network latency measurement:
`0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5` seconds.

### Deployment Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `dewy.deployments.total` | Counter | {deployment} | Total number of successful deployments |
| `dewy.deployment.duration` | Histogram | s | Duration of the deployment process |
| `dewy.deployment.errors.total` | Counter | {error} | Total number of failed deployments |

The `dewy.deployment.duration` histogram uses the following bucket boundaries:
`1, 5, 10, 30, 60, 120, 300, 600` seconds.

### Health Check Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `dewy.healthchecks.total` | Counter | {check} | Total number of health checks performed |
| `dewy.healthchecks.failures.total` | Counter | {check} | Total number of failed health checks |

### Container Metrics

In container mode, Dewy inspects the containers it manages on each scrape and
reports their lifecycle state. These are the metrics that answer "did a
container crash or restart?", analogous to what kube-state-metrics exposes for
Kubernetes pods. Per-container series are labeled with `app`, `container` (the
container name), and `replica` (its replica index).

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `dewy.container.replicas` | Gauge | {replica} | Number of running container replicas |
| `dewy.container.desired_replicas` | Gauge | {replica} | Number of replicas Dewy is configured to run |
| `dewy.container.restarts` | Gauge | {restart} | Restart count reported by the runtime |
| `dewy.container.status` | Gauge | {container} | 1 for the container's current `state` label, 0 otherwise |
| `dewy.container.last_terminated.exit_code` | Gauge | — | Exit code of a stopped container |
| `dewy.container.oom_killed` | Gauge | — | 1 if a stopped container was OOM-killed, else 0 |
| `dewy.container.started.timestamp` | Gauge | s | Container start time (Unix seconds) |
| `dewy.container.info` | Gauge | — | Always 1; carries `image` and `version` labels |

The `state` label on `dewy.container.status` takes one of `created`, `running`,
`paused`, `restarting`, `exited`, `dead`. The exit-code and OOM metrics are
reported only while a stopped container still exists; Dewy stops reporting a
container that terminated more than one hour ago so old series do not
accumulate.

> **`dewy.container.restarts` is a gauge, not a counter.** It mirrors the
> runtime's own restart count, which resets to 0 when Dewy replaces the
> container on a deploy. Query it with `changes()` or `delta()`, not
> `increase()`/`rate()` (those assume a monotonic counter and would miss or
> misread the resets).

#### Enabling restarts

Dewy does not restart crashed containers itself — it observes and reports. To
get automatic restarts (and a non-zero `dewy.container.restarts`), pass a
restart policy through to the runtime after the `--` separator:

```
dewy container --registry ... -- --restart=on-failure
```

Without a restart policy the container stays stopped after a crash, which shows
up as `dewy.container.replicas` falling below `dewy.container.desired_replicas`.

#### kube-state-metrics equivalents

| Dewy metric | kube-state-metrics analogue |
|-------------|-----------------------------|
| `dewy.container.restarts` | `kube_pod_container_status_restarts_total` |
| `dewy.container.status` | `kube_pod_status_phase` |
| `dewy.container.oom_killed` / `last_terminated.exit_code` | `kube_pod_container_status_last_terminated_reason` |
| `dewy.container.started.timestamp` | `kube_pod_start_time` |
| `dewy.container.replicas` | `kube_deployment_status_replicas_available` |
| `dewy.container.desired_replicas` | `kube_deployment_spec_replicas` |
| `dewy.container.info` | `kube_pod_info` |

## Integration Examples

### Prometheus + Grafana

A typical setup with Prometheus scraping Dewy's metrics endpoint:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'dewy'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:17539']
```

Useful PromQL queries:

```promql
# Request rate (connections per second)
rate(dewy_proxy_connections_total[5m])

# Active connections
dewy_proxy_connections_active

# P99 backend connection latency
histogram_quantile(0.99, rate(dewy_proxy_connect_latency_bucket[5m]))

# Deployment frequency (per hour)
increase(dewy_deployments_total[1h])

# Error rate
rate(dewy_proxy_errors_total[5m])

# Container restarts in the last hour (gauge: use changes, not increase)
changes(dewy_container_restarts[1h])

# Replica shortfall: containers Dewy wants minus what is actually running
dewy_container_desired_replicas - dewy_container_replicas

# Containers OOM-killed
dewy_container_oom_killed == 1
```

### OpenTelemetry Collector

Send metrics to any OTel-compatible backend (Datadog, New Relic, Grafana Cloud, etc.):

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  # Example: export to Prometheus remote write
  prometheusremotewrite:
    endpoint: "https://prometheus.example.com/api/v1/write"

  # Example: export to OTLP-compatible backend
  otlp:
    endpoint: "https://otel-ingest.example.com:4317"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheusremotewrite]
```

### systemd Integration

When running Dewy as a systemd service with telemetry:

```ini
# /etc/systemd/system/dewy.service
[Unit]
Description=Dewy Container Deployment
After=network.target docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dewy container \
  --telemetry \
  --otlp-endpoint localhost:4317 \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080 \
  --health-path /health \
  --replicas 2 \
  --log-format json \
  --log-level info
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## CLI Options

| Option | Description |
|--------|-------------|
| `--telemetry` | Enable telemetry (Prometheus metrics on Admin API `/metrics` endpoint) |
| `--otlp-endpoint` | OTLP gRPC endpoint for exporting metrics (e.g., `localhost:4317`). Automatically enables telemetry. |
| `--otlp-insecure` | Use insecure (plaintext) gRPC for OTLP export. Default is TLS. Use this for local or internal collectors without TLS. |
