---
title: Cache Configuration
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy stores artifacts in a local cache to significantly reduce the time required when downloading the same version of an application again. This caching functionality enables network load reduction and faster deployments. The cache is automatically managed on the filesystem, allowing efficient operation without user intervention.

## Cache Mechanism

Dewy's cache system manages downloaded artifacts with unique keys and automatically avoids duplicate downloads.

### Cache Keys

Each artifact is assigned a unique key in the format `tag--artifact`. This key ensures precise cache management through the combination of version and artifact name.

```
# Cache key examples
v1.2.3--myapp_linux_amd64.tar.gz
v2.0.0--frontend_linux_amd64.zip
v1.5.1--backend_darwin_arm64.tar.gz
```

This key format properly distinguishes artifacts for different versions or platforms of the same application.

### Current Version Management

Dewy uses a special key called `current` to track the version of the currently running application. This information enables efficient determination of whether a new version is available.

```bash
# Example in cache directory
.dewy/cache/
├── current                              # Current version information
├── v1.2.3--myapp_linux_amd64.tar.gz    # Cached artifact
└── v1.2.2--myapp_linux_amd64.tar.gz    # Previous version
```

When the current version matches the new version, dewy automatically skips downloading and uses the existing cache.

### Preventing Duplicate Deployments

The cache system automatically skips unnecessary processing when the same version of an application is already deployed. This achieves system resource conservation and stable operation.

However, if the application startup fails in server mode, deployment processing will execute even when cache exists, working to resolve the problem.

## Cache Directory Configuration

The cache file storage location can be flexibly configured according to environment and operational requirements.

### Default Configuration

When no special configuration is provided, dewy automatically creates a `.dewy/cache` directory within the working directory at runtime.

```bash
# Default cache directory
./dewy/cache/

# Actual placement example
/opt/myapp/.dewy/cache/
├── current
├── v1.2.3--myapp_linux_amd64.tar.gz
└── v1.2.2--myapp_linux_amd64.tar.gz
```

This default configuration ensures independent cache areas for each application, preventing mutual interference.

### Environment Variable Configuration

Using the `DEWY_CACHEDIR` environment variable, you can explicitly specify the cache directory. This feature is effective when you want to share cache between multiple applications or use specific disk areas.

```bash
# Specify cache directory with environment variable
export DEWY_CACHEDIR=/var/cache/dewy
dewy server --registry ghr://myorg/app --port 8080 -- /opt/app/current/app

# Shared cache for multiple applications
export DEWY_CACHEDIR=/shared/cache/dewy
```

When using shared cache, the same artifacts can be reused across multiple applications, reducing disk usage and download time.

### Fallback Functionality

If creation of the specified cache directory fails, dewy automatically falls back to a temporary directory. This functionality ensures continued system operation even in situations with permission issues or insufficient disk space.

```bash
# Temporary directory example during fallback
/tmp/dewy-123456789/
```

However, when using temporary directories, cache is lost upon system restart, so configuring a permanent cache directory is recommended.

## Cache Operations

Dewy's cache is completely auto-managed, requiring no special operations in normal operation.

### Automatic Management

All operations including cache reading/writing, size management, and deletion of old files are executed transparently by dewy. Users can benefit from fast deployments without being aware of the cache's existence.

When new version artifacts are downloaded, they are automatically saved to cache, and subsequent access to the same version is immediately read from cache.

### Size Limitations

By default, the cache directory size is limited to 64MB. This limitation prevents unlimited growth of disk usage and ensures stable system operation.

```bash
# Default maximum size: 64MB
# For typical application artifacts, multiple version caches are possible
```

When the size limit is reached, errors occur when saving new artifacts, but dewy continues efficient operation by utilizing existing cache as much as possible.

### Archive Extraction Functionality

Cached artifacts are automatically extracted during deployment. Dewy supports the following archive formats and automatically detects the format to execute appropriate extraction processing.

- **tar.gz / tgz**: gzip-compressed tar archives
- **tar.bz2 / tbz2**: bzip2-compressed tar archives
- **tar.xz / txz**: xz-compressed tar archives
- **tar**: uncompressed tar archives
- **zip**: ZIP format archives

During extraction processing, file permissions and structure within the archive are properly preserved, preparing an environment for normal application operation.