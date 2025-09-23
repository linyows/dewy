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
  --interval 60s -- /opt/myapp/current/myapp

# Future distributed cache (Consul/Redis) for cache sharing
# dewy server --registry ghr://owner/repo \
#   --cache consul://localhost:8500 \
#   --interval 30s -- /opt/myapp/current/myapp
```

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

Expected configuration for future distributed cache support:

```sh
# Consul cache sharing across multiple instances (planned)
# dewy server --registry ghr://owner/repo \
#   --cache consul://consul-cluster:8500 \
#   --interval 60s -- /opt/myapp/current/myapp
```

## Related Topics {% #related %}

- [Architecture](/architecture) - Dewy's overall configuration and cache positioning
- [Registry](/registry) - Artifact source configuration
- [FAQ](/faq) - Frequently asked questions about cache
