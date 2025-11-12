---
title: Deployment Workflow
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy achieves continuous application delivery through an automated deployment workflow. It periodically monitors registries and automatically executes download, extraction, and deployment when new releases are detected. This process is fully automated, maintaining the latest version of applications in running state without manual operations.

## Deployment Workflow Overview

Dewy's deployment workflow consists of five major phases that work together to achieve safe and efficient deployment.

The deployment workflow executes repeatedly at configured intervals (default 10 seconds) to maintain the latest state at all times. First, **periodic checks** monitor the registry to detect new releases. Next, **version comparison** compares with the currently running version to determine if updates are needed. When updates are required, **artifact download** retrieves binaries, **deployment execution** extracts applications, and finally **application control** manages server startup and restart.

Each phase includes appropriate error handling and notifications, automatically reporting to administrators when problems occur. Optimizations to avoid unnecessary processing are also built in for efficient system resource usage.

## Detailed Processing by Phase

Each phase of the deployment workflow executes processing with specific responsibilities, ensuring safe and reliable deployment. The following sequence diagram shows dewy's typical deployment workflow.

```mermaid
sequenceDiagram
    participant D as Dewy
    participant R as Registry
    participant C as Cache
    participant F as FileSystem
    participant S as Server
    participant N as Notifier

    loop Periodic execution (default 10-second intervals)
        D->>R: Get latest release information
        alt Artifact not found
            R-->>D: 404 Not Found
            note over D: Grace period check<br/>(skip if within 30 minutes)
        else Normal acquisition
            R->>D: Release information (tag, artifact URL)
            D->>C: Check current version
            C->>D: Value of current key
            D->>C: Get cache list
            C->>D: Existing cache file list

            alt Same version + server running
                note over D: Skip deployment
            else New version or server stopped
                alt Cache does not exist
                    D->>R: Download artifact
                    R->>D: Artifact data
                    D->>C: Save to cache
                    D->>N: Download completion notification
                end

                note over D: Execute Before Deploy Hook
                D->>F: Extract archive
                F->>D: Extraction complete
                D->>F: Update symbolic link
                F->>D: Update complete
                note over D: Execute After Deploy Hook

                alt Server mode
                    alt Server running
                        D->>S: Send SIGHUP (restart)
                        S->>D: Restart complete
                        D->>N: Restart completion notification
                    else Server stopped
                        D->>S: Start server
                        S->>D: Startup complete
                        D->>N: Startup completion notification
                    end
                end
            end
        end
    end
```

### Check Phase

In the check phase, the latest release information is retrieved from the configured registry. Communication is performed according to the configured registry type such as GitHub Releases, S3, Google Cloud Storage, collecting available latest version information.

This phase implements an important feature called **grace period**. When artifacts are not found within 30 minutes of release creation, processing is skipped without treating it as an error. This enables operations that consider the time required for CI/CD systems to complete artifact builds and uploads after creating releases.

```bash
# Skip log example during grace period
DEBUG: Artifact not found within grace period message="artifact not found" grace_period=30m0s
```

### Cache Phase

In the cache phase, local cache status is verified based on retrieved release information. When artifacts of the same version are already cached, download processing is skipped for efficiency.

Currently running version information is managed with the `current` key and compared with the new version's cache key (`tag--artifact` format). When versions are identical and the server is operating normally, all subsequent processing is skipped.

```bash
# Log example when deployment is skipped
DEBUG: Deploy skipped
```

However, if application startup fails in server mode, deployment processing continues even when cache exists. This provides an opportunity to automatically repair application problems.

### Download Phase

In the download phase, new version artifacts are downloaded from the registry and saved to local cache. Downloads are executed using optimized methods according to the configured registry type.

Downloaded artifacts are first saved to memory buffers and persisted as cache files only after download is completely finished. This method prevents generation of corrupted files due to interruption during download.

```bash
# Log example when download completes
INFO: Cached artifact cache_key="v1.2.3--myapp_linux_amd64.tar.gz"
INFO: Download notification message="Downloaded artifact for v1.2.3"
```

After download completion, completion reports are sent to administrators and teams through the notification system.

### Deploy Phase

In the deploy phase, cached artifacts are extracted and placed in the application directory. This phase is the most critical stage, consisting of multiple sub-steps.

First, **Before Deploy Hook** is executed if configured. This hook enables automation of necessary preparation work before deployment (database migration, service stops, etc.). Even if hooks fail, deployment continues, but failures are recorded and notified.

Next, artifacts are extracted to directories in `releases/YYYYMMDDTHHMMSSZ` format. Extraction processing supports major archive formats such as tar.gz, zip, tar.bz2, and properly preserves file permissions.

```bash
# Deploy processing log example
INFO: Extract archive path="/opt/app/releases/20240115T103045Z"
INFO: Create symlink from="/opt/app/releases/20240115T103045Z" to="/opt/app/current"
```

After extraction completion, the `current` symbolic link is updated to point to the new release directory. Existing symbolic links are removed beforehand, ensuring atomic switching.

Finally, **After Deploy Hook** is executed if configured. This hook enables automation of post-deployment processing (cache clearing, notification sending, service resumption, etc.).

### Startup Control Phase

In the startup control phase, application startup state is managed when operating in server mode. In assets mode, this processing is skipped since only file placement is the objective.

When a server is already running, **server restart** processing is executed. Dewy sends a SIGHUP signal to its own process and executes a graceful restart through the server-starter library. This method enables switching to new versions while minimizing impact on connected clients.

```bash
# Log example during server restart
INFO: Send SIGHUP for server restart pid=12345
INFO: Restart notification message="Server restarted for v1.2.3"
```

When the server is not running, **server startup** processing is executed. Application processes are started using server-starter and listening on configured ports begins.

After startup/restart processing completion, results are reported through the notification system, enabling operations teams to understand the situation.

## Deployment Skip Conditions

Dewy includes functionality to automatically skip unnecessary deployment processing for efficient operation. This achieves system resource conservation and stable operation.

### Avoiding Duplicate Deployments with Same Version

The most basic skip condition is when the currently running version and newly checked version are identical. Duplicate processing of the same artifact is prevented through current version information and cache existence verification.

This determination is made by comparing the value stored in the `current` key with the cache key generated from the new release. When they match completely, all subsequent processing is skipped and a "Deploy skipped" log is output.

### Determination Based on Server Execution State

During server mode operation, application execution state is also considered. When versions are identical and the server is running normally, processing is skipped to maintain the current state.

However, when server startup failure is detected, deployment processing executes even with identical versions. This provides an opportunity to automatically repair startup failures due to configuration changes or resource problems.

### Always Skip in Assets Mode

When operating in assets mode, processing is always skipped for identical versions. Since the main purpose is static file placement, it is determined that redeployment is unnecessary for the same version regardless of server execution state.

```bash
# Skip example in assets mode
# Always return nil for identical versions
```

This behavior eliminates wasteful processing in static file delivery for CDNs and web servers.

## Error Handling

Dewy's deployment workflow incorporates comprehensive error handling functionality to respond to various failure situations. Appropriate responses and continuity assurance are provided for problems that may occur at each stage.

### Grace Period for Artifact Not Found

Considering CI/CD system characteristics, a grace period is applied for artifact not found immediately after release creation. Within 30 minutes of release tag creation, missing artifacts are not treated as errors.

This functionality ensures temporal allowance from release creation by GitHub Actions or other CI/CD systems until build process completion and artifact upload. Warning logs are output during the grace period, but alert notifications are not sent.

```go
// 30-minute grace period configuration
gracePeriod := 30 * time.Minute
if artifactNotFoundErr.IsWithinGracePeriod(gracePeriod) {
    // Avoid error notification and return nil
    return nil
}
```

When the grace period is exceeded, it is processed as a normal error and notifications are sent to administrators.

### Continued Processing During Deploy Hook Failures

Even when Before Deploy Hook or After Deploy Hook execution fails, deployment processing itself continues. This design prevents auxiliary processing failures from blocking main deployment.

Hook failures are logged in detail and reported to administrators through the notification system. Administrators can check failure details and manually address them as needed.

```bash
# Log example during hook failure
ERROR: Before deploy hook failure error="command failed with exit code 1"
```

Deployment continues even after Before Deploy Hook failure, and After Deploy Hook is also executed. This behavior enables completing system updates as much as possible even with partial failures.

### Error Processing and Notification in Each Phase

Errors occurring in each phase are appropriately categorized by type and corresponding notifications are sent. Specialized error messages and logs are generated for registry access errors, download failures, extraction errors, server startup failures, etc.

When critical errors occur, subsequent processing is aborted and the system maintains the previous state. This prevents service interruption due to incomplete deployments. For temporary errors, automatic retry occurs in the next periodic check cycle.

Error information is recorded as structured logs and can be utilized for automatic analysis in monitoring systems or detailed investigation by administrators.

## Notification and Logging

Dewy provides comprehensive notification and logging functionality to visualize the entire deployment process and enable operations teams to accurately understand the situation.

### Deployment Start, Completion, and Failure Notifications

At important deployment milestones, messages are automatically sent to configured notification channels (Slack, email, etc.). At deployment start, target versions and processing content are notified, and at completion, successful versions and execution times are reported.

```bash
# Notification message examples
"Downloaded artifact for v1.2.3"
"Server restarted for v1.2.3"
"Automatic shipping started by Dewy (v1.0.0: server)"
```

During failures, notifications containing detailed error information and recommended countermeasures are sent. This enables operations teams to quickly recognize problems and start appropriate responses.

### Hook Execution Result Notifications

Before Deploy Hook and After Deploy Hook execution results are notified with detailed execution information. For success, execution time and output content are reported; for failure, exit codes and error messages are reported.

```bash
# Hook execution result examples
INFO: Execute hook command="npm run build" stdout="Build completed" duration="45.2s"
ERROR: After deploy hook failure error="Migration failed" exit_code=1
```

This information enables detailed tracking of each step in automated deployment processes, helping with early problem detection and resolution.

### Detailed Log Output at Each Stage

Dewy uses structured logging to record detailed information at each stage of the deployment workflow. According to log levels, information can be collected at necessary granularity from debug information to important events.

Major log entries include timestamps, log levels, messages, and related metadata (versions, cache keys, process IDs, etc.). This enables rapid identification of necessary information during problem investigation.

```json
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"Dewy started","version":"v1.0.0","commit":"abc1234"}
{"time":"2024-01-15T10:30:46Z","level":"INFO","msg":"Cached artifact","cache_key":"v1.2.3--myapp_linux_amd64.tar.gz"}
{"time":"2024-01-15T10:30:47Z","level":"DEBUG","msg":"Deploy skipped"}
```

Log information is utilized for real-time monitoring, trend analysis, performance optimization, and satisfying audit requirements, forming an important foundation for dewy operations.

## Container Deployment Workflow

When operating in container mode (`dewy container`), the deployment workflow differs significantly from binary deployments. Container deployments use a Blue-Green deployment strategy with Docker network aliases to achieve zero-downtime updates.

### Container Deployment Sequence

The following sequence diagram shows the container deployment workflow using the network-alias strategy:

```mermaid
sequenceDiagram
    participant D as Dewy
    participant R as OCI Registry
    participant RT as Container Runtime
    participant N as Docker Network
    participant C as Current Container (Blue)
    participant G as New Container (Green)
    participant HC as Health Checker
    participant NO as Notifier

    loop Periodic execution (default 30-second intervals)
        D->>R: Get latest image tags (OCI API)
        R->>D: Image tag list (semantic versioning)
        D->>D: Compare with current version

        alt New version available
            D->>RT: Pull new image (docker pull)
            RT->>R: Download image layers
            R->>RT: Image layers (cached if exists)
            RT->>D: Pull complete
            D->>NO: Image pull notification

            Note over D: Prepare Blue-Green deployment

            D->>RT: Start Green container
            RT->>G: Create and start container
            G->>D: Container started (without network alias)

            D->>HC: HTTP/TCP health check (internal network)
            HC->>G: Check health endpoint

            alt Health check fails
                G-->>HC: Unhealthy response
                HC->>D: Health check failed
                D->>RT: Remove Green container (rollback)
                RT->>G: Stop and remove
                D->>NO: Deployment failed notification
            else Health check succeeds
                G-->>HC: Healthy response (200 OK)
                HC->>D: Health check passed

                Note over D,N: Traffic switching phase
                D->>N: Add network alias to Green
                N->>G: Attach alias (e.g., "myapp-current")
                Note over G: New traffic → Green

                alt Blue container exists
                    D->>N: Remove network alias from Blue
                    N->>C: Detach alias
                    Note over C: Only existing connections

                    Note over D: Drain period (default 30s)
                    D->>D: Wait for drain time
                    Note over C: Existing connections complete

                    D->>RT: Stop Blue container gracefully
                    RT->>C: Send SIGTERM (30s grace period)
                    C->>RT: Graceful shutdown
                    D->>RT: Remove Blue container
                    RT->>C: Remove container
                end

                D->>RT: Update Green label to "current"
                RT->>G: Label update
                D->>NO: Deployment success notification
                Note over D,G: Deployment completed
            end
        else Same version
            note over D: Skip deployment
        end
    end
```

### Container Deployment Phases

#### 1. Image Check Phase

In the container deployment workflow, the check phase queries the OCI/Docker registry using the Distribution API (v2). Unlike binary deployments that check GitHub Releases or S3, container deployments:

- Fetch image tags using `GET /v2/<name>/tags/list`
- Filter tags using Semantic Versioning rules
- Support pre-release tags with query parameters
- Authenticate using Docker config.json or environment variables

```bash
# Example registry URLs
img://ghcr.io/linyows/myapp
img://us-central1-docker.pkg.dev/project-id/myapp-repo/myapp
img://docker.io/library/nginx
```

#### 2. Image Pull Phase

When a new version is detected, dewy instructs the container runtime to pull the image:

```bash
# Log example during image pull
INFO: Pulling new image image="ghcr.io/linyows/myapp:v1.2.3"
INFO: Image pull complete digest="sha256:abc123..."
INFO: Image pull notification message="Pulled image v1.2.3"
```

Container images are composed of multiple layers, which are cached by the container runtime. Only changed layers are downloaded, significantly reducing deployment time for incremental updates.

#### 3. Blue-Green Deployment Phase

Container deployments use a Blue-Green strategy with Docker network aliases for zero-downtime updates. This phase consists of several critical steps:

**Step 1: Start Green Container**

The new version container (Green) is started on the Docker network without the network alias:

```bash
# Log example
INFO: Starting new container name="myapp-green-1234567890" image="myapp:v1.2.3"
```

**Step 2: Health Check**

Dewy performs health checks against the Green container using the internal Docker network:

```bash
# HTTP health check via internal network
INFO: Health checking new container url="http://myapp-green-1234567890:8080/health"
INFO: Health check passed status=200 duration="50ms"
```

If health checks fail, the Green container is immediately removed (rollback), and the Blue container continues serving traffic.

**Step 3: Traffic Switching**

Once health checks pass, dewy performs atomic traffic switching using Docker network aliases:

```bash
# Add alias to Green (new traffic goes here)
$ docker network connect --alias myapp-current myapp-net green-container-id

# Remove network from Blue (no new traffic)
$ docker network disconnect myapp-net blue-container-id
```

This operation is atomic from the network perspective. New connections immediately route to the Green container, while existing connections to Blue continue unaffected.

**Step 4: Drain Period**

After traffic switching, dewy waits for a configurable drain period (default 30 seconds) to allow existing connections to the Blue container to complete:

```bash
# Log example
INFO: Waiting for drain period duration=30s
```

During this period:
- Blue container continues processing existing requests
- Green container handles all new requests
- No traffic interruption occurs

**Step 5: Blue Container Removal**

After the drain period, the Blue container is gracefully stopped and removed:

```bash
# Send SIGTERM with 30-second grace period
INFO: Stopping old container gracefully container="myapp-blue-1234567880"
INFO: Deployment completed successfully container="myapp-green-1234567890"
```

### Container Deployment vs Binary Deployment

Key differences between container and binary deployment workflows:

| Aspect | Binary Deployment | Container Deployment |
|--------|------------------|---------------------|
| **Artifact Source** | GitHub Releases, S3, GCS | OCI Registry (Docker Hub, GHCR, etc.) |
| **Artifact Format** | tar.gz, zip archives | OCI Image (multi-layer) |
| **Deployment Strategy** | In-place update + SIGHUP | Blue-Green with network alias |
| **Downtime** | Minimal (restart time) | Zero (seamless switch) |
| **Rollback** | Previous release directory | Keep Blue container |
| **Health Check** | Process-based | HTTP/TCP endpoint |
| **Traffic Management** | server-starter (port handoff) | Docker network alias |

### Network Architecture

Container deployments require a Docker network and reverse proxy setup:

```
[External Client]
       ↓
   [nginx:80]  ← Host port binding
       ↓
   myapp-current (network alias)  ← Docker internal DNS
       ↓
   [myapp-green-xxxxx:8080]  ← Actual container
```

The reverse proxy (nginx, HAProxy, etc.) always connects to the network alias, which seamlessly points to the latest healthy container version.

### Container Deployment Error Handling

Container deployments include specific error handling mechanisms:

#### Health Check Failures

When Green container health checks fail, automatic rollback occurs:

```bash
ERROR: Health check failed error="connection refused" attempts=5
INFO: Rolling back deployment
INFO: Removing failed container container="myapp-green-1234567890"
```

The Blue container remains operational, ensuring service continuity.

#### Image Pull Failures

If image pulling fails due to network issues or authentication problems:

```bash
ERROR: Image pull failed error="authentication required" image="ghcr.io/linyows/myapp:v1.2.3"
```

The system retries on the next polling interval. The current container continues running unchanged.

#### Container Startup Failures

If the Green container fails to start (invalid configuration, missing dependencies):

```bash
ERROR: Container start failed error="port already in use: 8080"
INFO: Removing failed container
```

Dewy removes the failed container and maintains the current version in production.

### Container Deployment Notifications

Container-specific notifications include additional information:

```bash
# Pull notification
"Pulled image ghcr.io/linyows/myapp:v1.2.3 (digest: sha256:abc123...)"

# Deployment start
"Starting Blue-Green deployment: v1.2.2 → v1.2.3"

# Health check success
"Health check passed for new container (200 OK, 45ms)"

# Traffic switch
"Switched traffic to new container (Green)"

# Deployment complete
"Deployment completed successfully (total: 45s, drain: 30s)"
```

These notifications provide detailed visibility into the container deployment process, enabling operations teams to track deployment progress and diagnose issues quickly.

## Reverse Proxy with Caddy

Dewy supports automatic reverse proxy setup using [Caddy](https://caddyserver.com/) for container deployments. When enabled, dewy manages both the proxy container and application containers, providing a clean separation between external traffic and internal application traffic.

### Proxy Architecture

```mermaid
graph LR
    Client[External Client] -->|HTTP :8000| Proxy[Caddy Proxy Container]
    Proxy -->|Internal Network| Alias[dewy-current:3333]
    Alias -.->|DNS Resolution| App1[App Container v1.2.3]

    style Proxy fill:#f9f,stroke:#333,stroke-width:2px
    style Alias fill:#bbf,stroke:#333,stroke-width:2px
    style App1 fill:#bfb,stroke:#333,stroke-width:2px
```

Key components:

- **Caddy Proxy Container**: Handles external HTTP traffic on the specified port
- **Docker Network**: Internal network (`dewy-net` by default) for container communication
- **Network Alias**: DNS alias (`dewy-current` by default) that points to the current app container
- **App Container**: Application container with no external port mapping (internal only)

### Enabling Reverse Proxy

To enable the reverse proxy feature, use the `--proxy` flag:

```bash
dewy container \
  --registry "img://ghcr.io/linyows/myapp" \
  --container-port 3333 \
  --health-path /health \
  --proxy \
  --proxy-port 8000 \
  --proxy-image caddy:2-alpine
```

**Options:**

- `--proxy`: Enable reverse proxy (boolean flag)
- `--proxy-port`: External port for the proxy to listen on (default: `80`)
- `--proxy-image`: Caddy container image to use (default: `caddy:2-alpine`)

### How It Works

When `--proxy` is enabled, dewy performs the following additional steps:

#### 1. Proxy Container Startup

On dewy startup, before the first deployment:

```bash
# Check for existing proxy container
INFO: Checking for existing proxy container
INFO: Starting reverse proxy container name="app-proxy" port=8000

# Proxy starts successfully
INFO: Proxy started successfully container=abc123... proxy_port=8000 upstream=dewy-current:3333
```

If a proxy container already exists, dewy exits with an error to prevent conflicts.

#### 2. Application Deployment Without External Ports

When proxy is enabled, application containers are started without port mappings:

```bash
# Without proxy (normal mode)
docker run -d -p 3333:3333 --name myapp-123 myapp:v1.2.3

# With proxy enabled (no -p flag)
docker run -d --name myapp-123 --network dewy-net --network-alias dewy-current myapp:v1.2.3
```

External traffic can only reach the application through the Caddy proxy.

#### 3. Blue-Green Deployment with Proxy

During Blue-Green deployment, the proxy automatically routes to the new container via the network alias:

```mermaid
sequenceDiagram
    participant Client
    participant Proxy as Caddy Proxy
    participant Alias as dewy-current
    participant Blue as Blue Container (old)
    participant Green as Green Container (new)

    Note over Client,Blue: Before deployment
    Client->>Proxy: HTTP request
    Proxy->>Alias: Forward to dewy-current:3333
    Alias->>Blue: Route to Blue
    Blue->>Proxy: Response
    Proxy->>Client: Response

    Note over Green: New container starts & passes health check

    Note over Alias,Green: Network alias switched
    Alias-.->Green: Now points to Green

    Note over Client,Green: After deployment
    Client->>Proxy: HTTP request
    Proxy->>Alias: Forward to dewy-current:3333
    Alias->>Green: Route to Green
    Green->>Proxy: Response
    Proxy->>Client: Response

    Note over Blue: Blue container removed
```

The proxy container continues running throughout the deployment, providing uninterrupted service.

#### 4. Cleanup on Shutdown

When dewy receives `SIGINT`, `SIGTERM`, or `SIGQUIT`, it automatically cleans up all managed containers:

```bash
INFO: Cleaning up managed containers
INFO: Stopping managed container container=app-proxy
INFO: Removing managed container container=app-proxy
INFO: Stopping managed container container=myapp-123
INFO: Removing managed container container=myapp-123
INFO: Cleanup completed containers_cleaned=2
```

All containers labeled with `dewy.managed=true` are stopped and removed.

### Proxy Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Checking: dewy starts with --proxy
    Checking --> Error: Proxy already exists
    Error --> [*]

    Checking --> Starting: No proxy found
    Starting --> Running: Proxy started
    Running --> Running: App deployments
    Running --> Cleanup: SIGINT/SIGTERM
    Cleanup --> [*]: All containers removed
```

### Use Cases

The reverse proxy feature is useful for:

1. **Single Entry Point**: All traffic goes through one port, simplifying firewall rules
2. **TLS Termination**: Use Caddy's automatic HTTPS in production (requires domain configuration)
3. **Load Balancing**: Future support for running multiple app instances
4. **Request Logging**: Centralized logging at the proxy level
5. **Security**: Application containers are not directly exposed to external networks

### Limitations

Current limitations of the reverse proxy feature:

- HTTP only (HTTPS/TLS termination not yet supported)
- Single application per dewy instance
- Caddy configuration is managed automatically (no custom Caddyfile support)
- Proxy container name is fixed to `app-proxy`

### Example: Complete Setup

```bash
# Start dewy with reverse proxy
dewy container \
  --registry "img://ghcr.io/linyows/myapp?pre-release=true" \
  --container-port 3333 \
  --health-path /health \
  --health-timeout 30 \
  --proxy \
  --proxy-port 8000 \
  --log-level info

# Output:
# INFO: Dewy started version=v1.0.0
# INFO: Starting reverse proxy container name=app-proxy port=8000
# INFO: Proxy started successfully
# INFO: Pulling image ghcr.io/linyows/myapp:v1.2.3
# INFO: Starting container name=myapp-1234567890
# INFO: Health check passed
# INFO: Deployment completed successfully

# Access the application through the proxy
curl http://localhost:8000/

# View container logs (proxy)
docker logs -f app-proxy

# View container logs (app)
docker logs -f $(docker ps -q --filter "label=dewy.role=current")
```

The reverse proxy feature simplifies container deployment by managing network traffic and providing a consistent access point for your applications.