---
title: テレメトリ
---

# {% $markdoc.frontmatter.title %} {% #overview %}

DewyはOpenTelemetry（OTel）ベースのテレメトリ機能を内蔵しており、プロキシのパフォーマンス、デプロイメントの状況、コンテナのヘルス状態を監視できます。テレメトリは特に**containerモード**で有用です。containerモードではDewyがアプリケーションとは独立したTCPプロキシとして動作するためです。

## アーキテクチャ

**serverモード**ではDewyはデプロイ対象のアプリケーションの一部として動作するため、アプリケーション側のOTel SDKを通じてテレメトリを収集できます（例：systemdで構成したotel-collector経由）。

**containerモード**ではDewyがコンテナのライフサイクルを管理するスタンドアロンのリバースプロキシとして動作します。アプリケーションとは別プロセスのため、Dewy自身にテレメトリパイプラインが必要です。DewyはOpenTelemetry SDKを内部で使用し、2つのエクスポートパスをサポートしています：

- **Prometheus exporter**: Admin APIの `/metrics` エンドポイントでスクレイプ可能な形式で公開
- **OTLP exporter**: gRPC経由でOpenTelemetry Collectorにメトリクスを送信

両方のエクスポーターを同時に使用でき、インフラ環境に合わせて最適な方法を選択できます。

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

## テレメトリの有効化

テレメトリはデフォルトで無効です。`--telemetry` フラグ、または `--otlp-endpoint` の指定で有効化できます。

### Prometheusのみ

Admin APIサーバーの `/metrics` エンドポイントを公開します。Prometheusからこのエンドポイントをスクレイプできます。

```bash
dewy container --telemetry \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080 --health-path /health
```

メトリクスエンドポイントは `http://localhost:17539/metrics`（Admin APIのポート）で利用できます。

### Prometheus + OTLP

Prometheusエンドポイントに加え、gRPC経由でOpenTelemetry Collectorにメトリクスを送信します。

```bash
dewy container --telemetry \
  --otlp-endpoint localhost:4317 \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080 --health-path /health
```

`--otlp-endpoint` が指定された場合、`--telemetry` フラグがなくてもテレメトリは自動的に有効化されます。

### OTLPのみ

Prometheusエンドポイントなしでotlpエクスポートのみが必要な場合は、エンドポイントを指定してください：

```bash
dewy container --otlp-endpoint otel-collector.internal:4317 \
  --registry img://ghcr.io/myorg/myapp \
  --port 8080
```

注意：テレメトリが有効な場合、Prometheusエクスポーターは常に登録されます。`/metrics` エンドポイントは常に利用可能ですが、スクレイプしないことを選択できます。

## メトリクスリファレンス

すべてのメトリクスは `dewy.` プレフィックスを使用し、OpenTelemetryのセマンティック規約に従います。

### プロキシメトリクス

各TCPプロキシ接続について記録され、`proxy_port` ラベルが付与されます。

| メトリクス | 種類 | 単位 | 説明 |
|-----------|------|------|------|
| `dewy.proxy.connections.total` | Counter | {connection} | プロキシが受け付けた接続の総数 |
| `dewy.proxy.connections.active` | UpDownCounter | {connection} | 現在アクティブなプロキシ接続数 |
| `dewy.proxy.connection.duration` | Histogram | s | プロキシ接続の持続時間（acceptからcloseまで） |
| `dewy.proxy.connect.latency` | Histogram | s | バックエンドへの接続確立にかかる時間 |
| `dewy.proxy.bytes.transferred` | Counter | By | プロキシを通じて転送されたバイト数（両方向の合計） |
| `dewy.proxy.errors.total` | Counter | {error} | プロキシエラーの総数（バックエンドなし、接続失敗） |
| `dewy.proxy.backends` | UpDownCounter | {backend} | アクティブなプロキシバックエンド数 |

`dewy.proxy.connect.latency` ヒストグラムはネットワークレイテンシ計測に最適化された以下のバケット境界を使用します：
`0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5` 秒

### デプロイメントメトリクス

| メトリクス | 種類 | 単位 | 説明 |
|-----------|------|------|------|
| `dewy.deployments.total` | Counter | {deployment} | 成功したデプロイの総数 |
| `dewy.deployment.duration` | Histogram | s | デプロイプロセスの所要時間 |
| `dewy.deployment.errors.total` | Counter | {error} | 失敗したデプロイの総数 |

`dewy.deployment.duration` ヒストグラムは以下のバケット境界を使用します：
`1, 5, 10, 30, 60, 120, 300, 600` 秒

### ヘルスチェックメトリクス

| メトリクス | 種類 | 単位 | 説明 |
|-----------|------|------|------|
| `dewy.healthchecks.total` | Counter | {check} | 実行されたヘルスチェックの総数 |
| `dewy.healthchecks.failures.total` | Counter | {check} | 失敗したヘルスチェックの総数 |

### コンテナメトリクス

| メトリクス | 種類 | 単位 | 説明 |
|-----------|------|------|------|
| `dewy.container.replicas` | UpDownCounter | {replica} | 稼働中のコンテナレプリカ数 |

## 連携例

### Prometheus + Grafana

Prometheusがdewyのメトリクスエンドポイントをスクレイプする一般的なセットアップ：

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'dewy'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:17539']
```

よく使うPromQLクエリ：

```promql
# リクエストレート（1秒あたりの接続数）
rate(dewy_proxy_connections_total[5m])

# アクティブ接続数
dewy_proxy_connections_active

# P99バックエンド接続レイテンシ
histogram_quantile(0.99, rate(dewy_proxy_connect_latency_bucket[5m]))

# デプロイ頻度（1時間あたり）
increase(dewy_deployments_total[1h])

# エラーレート
rate(dewy_proxy_errors_total[5m])
```

### OpenTelemetry Collector

OTel互換のバックエンド（Datadog、New Relic、Grafana Cloudなど）にメトリクスを送信：

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  # 例: Prometheus remote writeにエクスポート
  prometheusremotewrite:
    endpoint: "https://prometheus.example.com/api/v1/write"

  # 例: OTLP互換バックエンドにエクスポート
  otlp:
    endpoint: "https://otel-ingest.example.com:4317"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheusremotewrite]
```

### systemd連携

テレメトリ付きでDewyをsystemdサービスとして実行する場合：

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

## CLIオプション

| オプション | 説明 |
|-----------|------|
| `--telemetry` | テレメトリを有効化（Admin APIの `/metrics` エンドポイントでPrometheusメトリクスを公開） |
| `--otlp-endpoint` | メトリクスをエクスポートするOTLP gRPCエンドポイント（例：`localhost:4317`）。指定するとテレメトリが自動的に有効化されます。 |
| `--otlp-insecure` | OTLPエクスポートにinsecure（平文）gRPCを使用。デフォルトはTLS。TLSなしのローカルまたは内部Collectorに使用します。 |
