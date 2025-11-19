# Container Command Design Document

This document describes the design for adding container image deployment functionality to dewy. It achieves zero-downtime container deployment in non-Kubernetes environments through a built-in reverse proxy and rolling update deployment.

**Status**: Implemented
**Author**: @linyows
**Last Updated**: 2025-11

## Goals

1. **Container Environment Support**: Enable management of languages requiring runtimes (Node.js, Python, etc.) via containers
2. **Zero-Downtime Deployment**: Deploy new versions without service interruption
3. **Secure Implementation**: Avoid direct Docker socket manipulation, operate safely via CLI
4. **Simple Operations**: Complete on a single VM without Kubernetes
5. **Multiple Replica Support**: High availability and load balancing

## Non-Goals

1. **Multi-Host Support**: Distributed deployment across multiple servers is out of scope
2. **Advanced Load Balancing**: Weighted round-robin, sticky sessions, etc. are out of scope for initial version
3. **Kubernetes-Equivalent Features**: Service discovery, ConfigMap, Secrets, etc. are out of scope
4. **Auto-Scaling**: Dynamic adjustment of replica count is out of scope
5. **Audit Support**: Deployment history tracking is not supported for OCI registries (see Limitations section)

# Background

Dewy treats automatic deployment of binaries (like Go applications) and static files as first-class use cases. Languages requiring separate runtimes (like Node.js and Python) are also supported, though this requires the runtime to be present on the server. When server administrators and application developers are separate, this necessitates coordination and increases operational costs including runtime version management. This has led to demand for using containers so application developers can manage runtimes themselves.

This demand is valid, and could be addressed by using container orchestration platforms like Kubernetes. However, self-managed Kubernetes has operational costs, and managed services can be budget-prohibitive, leading to cases where users simply want to run Docker containers on VMs.

Watchtower is software that keeps containers on Docker updated with the latest container images. Watchtower runs as a container and manipulates the Docker socket, requiring root privileges, creating security risks. Diun notifies of Docker image updates but doesn't perform deployments.

# Design

The container runtimes dewy supports are Docker and Podman. For security, dewy operates containers from outside the container. To reduce library dependencies on runtime within dewy and assuming fewer specification changes, we use CLI. Like dewy server, dewy receives requests and acts as a reverse proxy, starting containers with `-p` (random port exposed on localhost) and `-l` (label assignment), proxying to containers managed by dewy. Port numbers are obtained from docker inspect, and dewy switches internal upstream targets. When dewy receives SIGINT, it terminates managed containers.

## Final Implementation

After consideration, we implemented the following design:

### Built-in Reverse Proxy

A reverse proxy built into dewy using Go's `net/http/httputil.ReverseProxy`.

**Reasons for Selection:**
- No need to manage external proxy containers (Caddy, etc.)
- No complexity in configuration changes
- Deployment logic is centralized and easier to understand
- Lower security risk

### Rolling Update Deployment

Initially considered Blue-Green deployment, but adopted Rolling Update approach considering multiple replica support.

**Deployment Flow:**
1. Pull new image
2. Find existing containers (`dewy.managed=true` label)
3. Start new replicas one by one
   - Each container binds to `127.0.0.1::containerPort` (localhost-only)
   - Docker assigns random host port
   - Health check via localhost
   - Add to proxy backends on success (round-robin)
4. After all new replicas start successfully, remove old replicas one by one
5. Rollback new containers on error

**Reasons for Selection:**
- Easy to support multiple replicas
- Gradual rollout reduces risk
- Works well with load balancing

### Port Mapping

Flexible port mapping with automatic container port detection.

**Specification:**
- `--port proxy`: Auto-detect container port from Docker image EXPOSE directive
- `--port proxy:container`: Explicit port mapping
- Multiple mappings supported: `--port 8080:80 --port 9090:50051`

**Auto-Detection Behavior:**
- If container port not specified, dewy inspects the Docker image
- Single EXPOSE port → automatically used
- Multiple EXPOSE ports → error, must specify explicitly
- No EXPOSE ports → error, must specify explicitly

**Examples:**
```bash
# Auto-detect (container EXPOSEs port 8080)
dewy container --registry img://myapp --port 8080

# Explicit mapping (proxy 8080 → container 80)
dewy container --registry img://myapp --port 8080:80

# Multi-port (HTTP + gRPC)
dewy container --registry img://myapp --port 8080:80 --port 9090:50051
```

### Multiple Replicas & Load Balancing

Start multiple containers with `--replicas` flag, dewy performs round-robin load balancing.

**Implementation Details:**
- `proxyBackends []*url.URL`: Slice of multiple backends
- `proxyIndex int`: Round-robin counter
- `sync.RWMutex`: Thread-safe backend management

### Security Design

**Localhost-only Binding:**
- Container ports bind to `127.0.0.1::containerPort`
- Prevents direct external access to containers
- Accessible only through dewy's proxy

**Container Management:**
- Label-based management (`dewy.managed=true`, `dewy.app=<name>`)
- Container operations via CLI (no root privileges required)
- Proper cleanup through signal handling

### Health Checks

Optional HTTP health check functionality.

**Specification:**
- `--health-path`: Health check endpoint (e.g., `/health`)
- `--health-timeout`: Timeout (default: 30 seconds)
- Check via localhost (`http://localhost:<mappedPort><healthPath>`)
- Retry until success (HTTP 200-299)

### Drain Time

Wait time for old containers after traffic switch.

**Specification:**
- `--drain-time`: Drain time (default: 30 seconds)
- Grace period to wait for existing requests to complete
- Stop and remove old containers after this period

## Architecture Diagrams

### System Configuration (3 Replica Example)

```
┌─────────────────────────────────────────────────────────┐
│                     External Clients                     │
└────────────────────────┬────────────────────────────────┘
                         │
                         ↓ :8080 (external interface)
┌─────────────────────────────────────────────────────────┐
│                    Dewy Reverse Proxy                    │
│                  (net/http/httputil)                     │
│                   Round-Robin Selector                   │
└───┬──────────────────┬──────────────────┬───────────────┘
    │                  │                  │
    ↓                  ↓                  ↓
┌─────────┐      ┌─────────┐      ┌─────────┐
│ app-1   │      │ app-2   │      │ app-3   │  (containers)
│ :8080   │      │ :8080   │      │ :8080   │
└─────────┘      └─────────┘      └─────────┘
    ↑                ↑                ↑
127.0.0.1        127.0.0.1        127.0.0.1   (localhost-only)
:random          :random          :random      (Docker assigned)
```

### Rolling Update Flow (3 Replica Example)

#### Phase 1: Current State
```
Dewy Proxy → [old-1, old-2, old-3]  (v1.0)
```

#### Phase 2: Gradual Startup of New Replicas
```
Step 1:
Dewy Proxy → [old-1, old-2, old-3, new-1]  (v1.0 + v2.0)
                                      ↑
                              Health Check OK

Step 2:
Dewy Proxy → [old-1, old-2, old-3, new-1, new-2]
                                             ↑
                                     Health Check OK

Step 3:
Dewy Proxy → [old-1, old-2, old-3, new-1, new-2, new-3]
                                                  ↑
                                          Health Check OK
```

#### Phase 3: Gradual Removal of Old Replicas
```
Step 1:
Dewy Proxy → [old-2, old-3, new-1, new-2, new-3]
               (old-1 removed)

Step 2:
Dewy Proxy → [old-3, new-1, new-2, new-3]
               (old-2 removed)

Step 3:
Dewy Proxy → [new-1, new-2, new-3]  (v2.0)
               (old-3 removed)
```

### Rollback Flow

When new replica health check fails:

```
Failure in Phase 2:
Dewy Proxy → [old-1, old-2, old-3, new-1, new-2]
                                             ↑
                                     Health Check FAIL

Rollback:
1. Remove new-2 from backends
2. Stop and remove new-2 and new-1
3. Report deployment failure

Result:
Dewy Proxy → [old-1, old-2, old-3]  (v1.0 maintained)
```

## Testing Strategy

### Unit Tests

1. **OCI Registry**: Image information retrieval from registry
2. **Container Runtime**: Mock Docker/Podman CLI operations
3. **Health Check**: Verify health check logic
4. **Proxy**: Verify round-robin selector behavior
5. **Deployment Flow**: Rolling update flow state transitions

### Integration Tests

1. **Real Container Deployment**: Using `img://ghcr.io/linyows/dewy-testapp`
2. **Rolling Update**: Gradual update of multiple replicas
3. **Health Check**: Actual HTTP endpoint checks
4. **Rollback**: Automatic rollback on health check failure
5. **Cleanup**: Delete all containers on SIGINT/SIGTERM

### Manual Testing

End-to-end testing in real environments:
- Different container runtimes (Docker, Podman)
- Different registries (Docker Hub, GHCR, GCR, ECR)
- Different replica counts (1, 3, 5)
- Long-running stability verification

## Monitoring & Observability

### Log Output

Records the following in structured logs (JSON format):

1. **Deployment Events**: Start, success, failure
2. **Container Lifecycle**: Start, stop, remove
3. **Health Checks**: Success, failure, timeout
4. **Proxy**: Backend addition, removal
5. **Errors**: Detailed errors with stack traces

### Metrics (Future Support)

Currently unimplemented, but considering the following metrics in the future:

- Request count (per backend)
- Response time (p50, p95, p99)
- Error rate
- Active connection count
- Deployment frequency and success rate

### Debugging Methods

1. **Log Level Adjustment**: Detailed logs with `--log-level debug`
2. **Container Check**: `docker ps -a --filter label=dewy.managed=true`
3. **Port Mapping Check**: `docker port <container-id>`
4. **Manual Health Check**: `curl http://localhost:<port>/health`

## Performance Considerations

### Resource Usage

**Dewy Process:**
- Memory: Approximately 30-50MB (depends on replica count)
- CPU: Minimal proxy processing overhead (<5%)
- Network: Near-zero overhead (via localhost)

**Containers:**
- Application-dependent
- Replica count × container size

### Throughput

Go `net/http/httputil.ReverseProxy` performance:
- Single proxy: 10k+ req/sec
- Round-robin selector overhead: <1μs
- Practical bottleneck is on the application side

### Scalability

**Current Limitations:**
- Single host: Up to host resource limits
- Replica count: Recommended maximum of 10 (more possible but operations become complex)

**Scale-out Methods:**
- Multiple dewy instances (on different ports)
- Place Nginx/HAProxy in front for load balancing

### Deployment Time

Rolling update duration (3 replica example):
- Image pull: 30 seconds - 5 minutes (depends on size and network)
- New replica startup: 5-30 seconds each (including health check)
- Old replica removal: 1-2 seconds each
- **Total**: Approximately 1-8 minutes (application-dependent)

Downtime: **0 seconds** (at least one replica always running)

## Trade-offs and Future Considerations

### Current Limitations

1. **Single Host Limitation**: Distributed deployment to multiple servers is not supported
2. **Load Balancing**: Round-robin only (weighted, sticky sessions, etc. not supported)
3. **Health Checks**: HTTP GET only (TCP, gRPC, etc. not supported)
4. **Audit Information**: Deployment history tracking is not supported

### Future Expansion Possibilities

- **Graceful Shutdown**: SIGTERM propagation to applications inside containers
- **Custom Load Balancing**: Least connections, IP hash, etc.
- **Metrics**: Record request count, latency, etc. through proxy
- **Multiple Port Support**: Support for multiple protocols (HTTP + gRPC, etc.)

# Why Not Separate the Proxy?

Why not have dewy not handle the proxy directly and instead delegate to existing reverse proxies like nginx or caddy? Separating the proxy functionality from dewy would allow flexibility in proxy choice. However, to perform deployment correctly, users would need to learn dewy's behavior, raising the barrier to adoption. If users want to use a specific proxy, they can simply place it in front of dewy, so there's no need to provide flexibility in proxy choice.

Two deployment methods were considered for container-based proxying.

**Conclusion: Ultimately, these methods were not adopted, and a built-in reverse proxy was implemented in dewy.**

## DNS Name Resolution Switching (Rejected)

Technically uses docker-network and network-alias.

### Pros:

- Uses name resolution so no need to change proxy configuration, proxy-independent

### Cons:

- Proxy keep-alive feature causes hanging issues
- Not logged, so timing of switches is not precisely known
- With high request volume, timing lag may cause some requests to go to non-existent containers resulting in EOF
- (For nginx, requires resolver and valid settings to add periodically)

## Upstream Target Change Switching (Rejected)

Caddy allows easy upstream target changes via admin API over socket.

### Pros:

- Proxy configuration reload reflects changes, so switch timing is precisely known

### Cons:

- Requires a proxy that can reload
- Nginx, which manages configuration only via files, requires templating functionality (increases complexity)
- Difficult to introduce desired proxy without understanding the mechanism
- Requires management of proxy container (start, stop, update)
- Proxy and application lifecycle management becomes complex

## Why Built-in Proxy Was Chosen

After considering the above two methods, we implemented a built-in reverse proxy in dewy for the following reasons:

### Reasons for Adoption:

1. **Simplicity**: No need to manage external proxy containers
2. **Integration**: Centralized management of deployment logic and proxy control
3. **Flexibility**: Easy implementation of multiple replica support and load balancing
4. **Performance**: Go's `net/http/httputil.ReverseProxy` is sufficiently fast
5. **Security**: No proxy configuration complexity, smaller attack surface
6. **Maintainability**: Unified codebase, easier debugging and improvement

### Trade-offs:

- Cannot use advanced proxy features like Caddy (automatic HTTPS, complex routing, etc.)
- However, if these are needed, they can be addressed by placing Caddy or Nginx in front of dewy

# References

## Related Software

- **Watchtower**: Tool for automatically updating containers on Docker
  - https://containrrr.dev/watchtower/
  - Features: Operates via Docker socket (requires root privileges)

- **Diun**: Docker image update notification tool
  - https://crazymax.dev/diun/
  - Features: Notification only, does not perform deployment

- **Portainer**: Docker management GUI
  - https://www.portainer.io/
  - Features: Manual operation-focused, no automatic deployment

## Technical Specifications

- **OCI Distribution Spec**: Container registry API specification
  - https://github.com/opencontainers/distribution-spec

- **Docker CLI Reference**: Container operation commands
  - https://docs.docker.com/engine/reference/commandline/cli/

## Implementation Files

- `dewy.go`: Rolling update deployment implementation (`deployContainer` function)
- `container/docker.go`: Docker CLI wrapper with validation and docker run option passthrough
- `container/container.go`: Runtime interface and data structures (RunOptions, DeployOptions)
- `registry/img.go`: OCI registry implementation
- `cli.go`: CLI definition for `container` command
  - Port mapping: `--port proxy[:container]` (auto-detects container port if not specified)
  - Dewy-specific options: `--replicas`, `--health-path`, `--cmd`
  - Docker run options passthrough via `--` separator
  - Forbidden options validation: `-d`, `-it`, `-i`, `-t`, `-l`, `-p`
- `config.go`: Container configuration structure (PortMappings, Command, ExtraArgs)

## Related Documentation

- [Deployment Workflow](../pages/deployment-workflow.md): User-facing deployment flow explanation
- [Getting Started](../pages/getting-started.md): How to use container deployment
- [Registry Documentation](../pages/registry.md): OCI registry (`img://`) specification
