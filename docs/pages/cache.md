---
title: Cache
description: |
  Cache is an important Dewy component that manages downloaded artifacts and avoids redundant network traffic.
  You can choose from multiple cache store implementations and enable cache sharing in distributed environments.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Overview {% #overview-details %}

The cache component plays the following important roles in Dewy's deployment process:

- **Artifact Storage**: Persistence of downloaded binary files
- **Version Management**: Recording and managing current version information
- **Duplicate Download Prevention**: Avoiding re-downloads of identical versions
- **Fast Deployment**: Immediate deployment from local cache

Cache is abstracted through a KVS interface, allowing selection of different implementations based on use cases.

## Cache Store Implementations {% #cache-stores %}

### File System (File) - Default {% #file-cache %}

The most basic implementation that stores artifacts in the local file system.

**Features:**
- Persistent data storage
- Data retention after system restart
- Simple configuration and management
- Built-in archive extraction functionality

**Supported Archive Formats:**
- `.tar.gz` / `.tgz`
- `.tar.bz2` / `.tbz2`
- `.tar.xz` / `.txz`
- `.tar`
- `.zip`

### AWS S3 {% #s3-cache %}

Shared cache backed by Amazon S3 (or any S3-compatible object storage). The S3 bucket is the source of truth across nodes, while each Dewy instance keeps a local staging copy for archive extraction.

**Features:**
- Cache sharing across many Dewy instances
- Greatly reduced artifact download traffic against the upstream registry
- Reuses AWS credentials already used for S3 registry sources
- Local staging makes extraction identical to the file backend

**URL Format:**

```sh
# s3://<region>/<bucket>/<prefix>?<options: endpoint>
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/mybucket/myapp -- /opt/myapp/current/myapp
```

The `endpoint` query parameter is supported for S3-compatible services. AWS authentication uses the standard credential chain (env vars, shared config, IAM role, etc.).

### Google Cloud Storage {% #gcs-cache %}

Shared cache backed by Google Cloud Storage. Same hybrid model as the S3 backend (cloud as source of truth, local staging for extraction).

**URL Format:**

```sh
# gs://<bucket>/<prefix>
dewy server --registry ghr://owner/repo \
  --cache gs://mybucket/myapp -- /opt/myapp/current/myapp
```

Authentication follows the standard Google Cloud authentication methods (`GOOGLE_APPLICATION_CREDENTIALS`, workload identity, ADC).

### Registry result cache {% #registry-result-cache %}

Both S3 and GCS cache backends accept a `registry-ttl=<duration>` query parameter. When set, the cache also stores the **upstream registry response**, and Dewy instances sharing the same prefix coordinate so that only one of them per TTL window calls the upstream registry. Followers read the cached response from the shared cache. Useful when many Dewy instances poll a rate-limited registry such as GitHub Releases.

```sh
# Shared registry-result cache with 30s freshness window
dewy server --registry ghr://owner/repo \
  --cache 's3://ap-northeast-1/mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp

# Same with GCS
dewy server --registry ghr://owner/repo \
  --cache 'gs://mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp
```

The refresh is serialized by a lock record stored under `locks/` in the same prefix, and the result is published with a conditional write (`If-Match` / `ifGenerationMatch`). On upstream failure the cache continues to serve the last known response (stale-but-usable), so a transient registry outage does not stop the cluster.

If the lock cannot be taken because the backend returns an error, Dewy logs `"failed to acquire registry refresh lock"` and polls the upstream registry directly. Coordination is lost for that tick, but deployments continue.

> Operational note: stale-but-usable hides upstream errors from `Dewy.Run()`'s normal error path, so prolonged outages won't surface via the configured notifier. Watch for the `"upstream registry failed; serving stale cache"` warning in the dewy log to detect them.

If `registry-ttl` is set on a backend that does not support atomic conditional writes (currently the file backend), Dewy logs a `"registry-ttl set but cache backend does not support atomic writes; ignoring"` warning at startup and proceeds without registry-result caching.

### Artifact download coordination {% #download-coordination %}

On the S3 and GCS backends, instances also coordinate the artifact download itself. This needs no configuration and is independent of `registry-ttl`: it is active whenever the backend supports conditional writes.

Without it, instances that poll at the same moment all miss the cache and all download the same artifact, which is up to 512MB per instance against the registry.

One instance takes a per-artifact lock under `locks/` and downloads. The others log `"Deploy deferred: a peer is downloading this artifact"` and skip the tick. On their next poll the artifact is already in the shared cache, and they take the ordinary cached path without downloading. A deferred tick is not treated as a failure: it does not trigger polling backoff, does not notify, and is not counted as a deployment.

The instance that downloads also records the artifact's SHA-256 digest and size in `blobs/<cache key>.json`. Instances that read the artifact from the shared cache check the bytes against that record, because they did not perform the checksum verification that a fresh download goes through. A mismatch fails the tick and deletes the local copy, so the next poll does not read the same bytes back from disk.

Artifacts cached by a Dewy release older than this feature have no digest record. They are used without that check; the record appears once the artifact is downloaded again.

The file backend is single-instance and is unaffected: downloads run exactly as before.

### Memory {% #memory-cache %}

{% callout type="warning" title="Not Implemented" %}
Memory cache is currently not implemented. It is planned for future versions.
{% /callout %}

High-speed implementation for managing artifacts in memory (planned).

**Expected Features:**
- Fast access
- Volatile (data lost on restart)
- Increased memory usage

### HashiCorp Consul {% #consul-cache %}

{% callout type="warning" title="Not Implemented" %}
Consul cache is currently not implemented. It is planned for future versions.
{% /callout %}

Implementation for achieving cache sharing in distributed environments (planned).

**Expected Benefits:**
- Cache sharing between multiple Dewy instances
- Reduced requests to registry
- Rate limiting countermeasures in distributed systems

### Redis {% #redis-cache %}

{% callout type="warning" title="Not Implemented" %}
Redis cache is currently not implemented. It is planned for future versions.
{% /callout %}

High-performance distributed cache system integration implementation (planned).

**Expected Features:**
- Fast distributed caching
- Automatic expiration with TTL settings
- Cluster support

## Cache Directory Configuration {% #cache-directory %}

Dewy determines the cache directory in the following priority order:

### 1. DEWY_CACHEDIR Environment Variable (Highest Priority)

```sh
export DEWY_CACHEDIR=/var/cache/dewy
dewy server --registry ghr://owner/repo -- /opt/myapp/current/myapp
```

### 2. Current Directory + .dewy/cache (Default)

```sh
# /opt/myapp/.dewy/cache will be used
cd /opt/myapp
dewy server --registry ghr://owner/repo -- ./current/myapp
```

### 3. Temporary Directory (Fallback)

When directory creation fails, it automatically falls back to a temporary directory.

### systemd Configuration Example

{% callout type="note" title="systemd Operation Tips" %}
When managing Dewy with systemd, it is recommended to specify a dedicated cache directory with `DEWY_CACHEDIR`.
{% /callout %}

```systemd
# /etc/systemd/system/dewy.service
[Unit]
Description=Dewy Application Deployment Service
After=network.target

[Service]
Type=simple
User=dewy
Group=dewy
Environment=DEWY_CACHEDIR=/var/cache/dewy
ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp -- /opt/myapp/current/myapp
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Set up directory and access permissions beforehand:

```sh
sudo mkdir -p /var/cache/dewy
sudo chown dewy:dewy /var/cache/dewy
sudo chmod 755 /var/cache/dewy
```

### Docker Environment Configuration

```sh
# Maintain cache with persistent volume
docker run -d \
  -e DEWY_CACHEDIR=/app/cache \
  -v /host/dewy-cache:/app/cache \
  dewy:latest server --registry ghr://owner/repo -- /opt/app/current/app
```

## Cache Key Mechanism {% #cache-keys %}

Dewy manages cache with the following key structure:

### current Key

A special key indicating the currently running application version.

```sh
# For file cache
cat /var/cache/dewy/current
# Output example: v1.2.3--app_linux_amd64.tar.gz
```

This value is used as a cache key that references the actual artifact file.

### Artifact Key

Format combining version tag and artifact name:

```
{version}--{artifact_name}
```

**Examples:**
- `v1.2.3--myapp_linux_amd64.tar.gz`
- `v2.0.0-rc.1--myapp_darwin_arm64.zip`

## Performance Optimization {% #performance %}

### Rate Limiting Countermeasures

When operating multiple Dewy instances, the following strategies can reduce requests to the registry:

```sh
# Increase polling interval (default: 10 seconds)
dewy server --registry ghr://owner/repo \
  --interval 60 -- /opt/myapp/current/myapp

# Use a shared S3/GCS cache to avoid each instance downloading the same artifact
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --interval 30 -- /opt/myapp/current/myapp
```

> Note: a shared cache reduces artifact download traffic against the upstream registry. It does not by itself reduce the number of metadata polls each instance makes; that is a separate concern (planned: stale-while-revalidate at the registry layer).

### Storage Management

```sh
# Cache size limitation (default: 64MB)
# Currently determined by file cache directory size
du -sh /var/cache/dewy
```

## Operations Guide {% #operations %}

### Troubleshooting

**When cache misses occur frequently:**

```sh
# Check cache directory
ls -la $DEWY_CACHEDIR

# Check current key
cat $DEWY_CACHEDIR/current

# Check permissions
ls -la $DEWY_CACHEDIR
```

**For permission errors:**

```sh
# Fix directory permissions
sudo chown -R dewy:dewy /var/cache/dewy
sudo chmod -R 755 /var/cache/dewy
```

**For insufficient disk space:**

```sh
# Check cache directory usage
df -h /var/cache/dewy

# Manually delete old cache files
find /var/cache/dewy -name "v*" -mtime +7 -delete
```

### Monitoring

**Check cache usage:**

```sh
# List cache files
ls -la /var/cache/dewy/

# Check current version
cat /var/cache/dewy/current

# Monitor cache access in logs
journalctl -u dewy.service -f | grep -i cache
```

## Configuration Examples and Best Practices {% #best-practices %}

### Recommended Production Configuration

```sh
# systemd environment
Environment=DEWY_CACHEDIR=/var/cache/dewy

# Appropriate polling interval
--interval 30s

# Structured logging for monitoring
--log-format json
```

### Lightweight Development Configuration

```sh
# Execute in project directory
cd /path/to/myproject
dewy server --registry ghr://owner/repo \
  --interval 5s \
  --log-format text -- ./current/myapp
```

### High Availability Configuration Strategy

Share artifact cache across many Dewy instances by pointing them at the same S3/GCS bucket:

```sh
# AWS S3 (any S3-compatible storage works with the endpoint option)
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --interval 60 -- /opt/myapp/current/myapp

# Google Cloud Storage
dewy server --registry ghr://owner/repo \
  --cache gs://dewy-cache/myapp \
  --interval 60 -- /opt/myapp/current/myapp
```

When dozens or hundreds of instances poll the same registry, only the first instance to detect a new release pays the artifact-download cost; the rest fetch from the shared cache.

## Related Topics {% #related %}

- [Architecture](/architecture) - Dewy's overall configuration and cache positioning
- [Registry](/registry) - Artifact source configuration
- [FAQ](/faq) - Frequently asked questions about cache
