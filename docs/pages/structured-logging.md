---
title: Structured Logging
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy provides structured logging functionality to systematically record information necessary for operational monitoring and troubleshooting. Structured logging differs from traditional text-based logs by organizing information in key-value format. This functionality makes log searching, filtering, and aggregation easier, and enables integration with automated monitoring systems.

## Log Format Selection

Dewy allows you to choose from two log formats depending on your use case. Select the optimal format based on your environment and operational setup.

### text format

The text format is human-readable and suitable for development environments and debugging work. It's effective when you need to directly examine log contents or manually investigate problems.

```bash
# Text format output
dewy server --log-format text --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

Text format output example:
```
time=2024-01-15T10:30:45.123Z level=INFO msg="Dewy started" version=v1.2.3 commit=abc1234 date=2024-01-15
time=2024-01-15T10:30:46.456Z level=INFO msg="Cached artifact" cache_key=v1.2.3--myapp_linux_amd64.tar.gz
```

This format is suitable for console display and simple log file verification.

### json format

The json format is suitable for machine processing and optimal for integration with log aggregation systems in production environments. It has high compatibility with log processing tools like Elasticsearch, Logstash, and Fluentd, enabling automated monitoring and analysis.

```bash
# JSON format output
dewy server --log-format json --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

JSON format output example:
```json
{"time":"2024-01-15T10:30:45.123Z","level":"INFO","msg":"Dewy started","version":"v1.2.3","commit":"abc1234","date":"2024-01-15"}
{"time":"2024-01-15T10:30:46.456Z","level":"INFO","msg":"Cached artifact","cache_key":"v1.2.3--myapp_linux_amd64.tar.gz"}
```

This format allows individual indexing of each log field, enabling fast searching and filtering.

## Command Line Configuration

Log settings can be controlled through command-line arguments. Choose appropriate combinations based on your environment and use case.

### --log-level Option

Log level is specified with the `--log-level` or `-l` option. Available values are debug, info, warn, and error (case-insensitive).

```bash
# Log level specification
dewy server --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
dewy server -l debug --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

The default setting is ERROR level.

### --log-format Option

Log format is specified with the `--log-format` or `-f` option. Available values are text and json (case-insensitive).

```bash
# Log format specification
dewy server --log-format json --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
dewy server -f text --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

The default setting is text format.