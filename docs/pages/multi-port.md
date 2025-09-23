---
title: Multi-Port Support
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy provides functionality to simultaneously manage multiple TCP ports for a single application. This feature enables unified deployment of services with different purposes, such as HTTP and HTTPS, API and WebUI, or production and debug endpoints. Integration with the server-starter library enables independent process management and graceful restarts for each port.

## Multi-Port Use Cases

In development environments or resource-constrained environments, there are often requirements for a single application to provide multiple services that would ideally be separated into independent services. Dewy's multi-port functionality is designed to address these complex requirements.

### Protocol-Based Separation

In development environments or cost-constrained environments, you may want to provide different protocols like REST API, gRPC, and GraphQL through a single application. Multi-port functionality allows you to support multiple protocols while reducing management costs.

```bash
# Provide REST API, gRPC, and GraphQL through a single application
dewy server --registry ghr://myorg/webapp --port 8080,8443,8090 -- /opt/webapp/current/webapp
```

This configuration allows port 8080 to serve REST API, port 8443 to serve gRPC services, and port 8090 to serve GraphQL endpoints simultaneously, simplifying server management in development environments.

### Function-Based Separation

Even within a single application, providing different functions like API, admin interface, and metrics collection on independent ports allows you to apply different security policies and access controls to each function.

```bash
# Multiple ports for API, admin interface, and metrics
dewy server --registry ghr://myorg/webapp \
  --port 8080,8090,9090 \
  -- /opt/webapp/current/webapp
```

In this example, a single application can provide the main API on port 8080, admin interface on port 8090, and Prometheus metrics on port 9090.

### Development Environment

Development environments need to provide both production and debug functionality simultaneously. Debug ports can provide detailed log output and profiling information, while production ports allow verification of normal operations.

```bash
# Parallel operation of production and debug functionality
dewy server --registry ghr://myorg/devapp \
  --port 8080,8081 \
  -- /opt/devapp/current/devapp --prod-port=8080 --debug-port=8081
```

Developers can test under the same conditions as production while accessing debug information when needed.

## Multi-Port Specification Methods

Dewy allows flexible and intuitive specification of multiple ports. Choose the optimal specification method based on your use case and environment.

### Single Port

In the simplest case, specify only one port. This behaves the same as traditional single-port applications.

```bash
# Single port specification
dewy server --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
# Or short form
dewy server --registry ghr://myorg/app -p 8080 -- /opt/app/current/app
```

### Multiple Ports

When specifying multiple ports, you can use multiple flags or comma-separated values. Both methods achieve the same result, so choose based on script readability and maintainability considerations.

```bash
# Multiple flag specification
dewy server --registry ghr://myorg/app -p 8080 -p 8081 -p 9090 -- /opt/app/current/app

# Comma-separated specification
dewy server --registry ghr://myorg/app --port 8080,8081,9090 -- /opt/app/current/app
```

The comma-separated method is suitable for configuration file or environment variable management, while the multiple flag method is suitable for dynamic configuration changes.

### Port Ranges

When specifying multiple consecutive ports, you can use range specification to write configurations concisely. This feature is particularly useful for load balancing or multi-instance configurations.

```bash
# Port range specification (8080 to 8085)
dewy server --registry ghr://myorg/app --port 8080-8085 -- /opt/app/current/app
```

Range specification has a maximum limit of 100 ports from security and resource management perspectives. If you need more ports, combine multiple range specifications.

### Mixed Specification

In actual operations, you can flexibly handle complex requirements by combining different specification methods.

```bash
# Combination of comma-separated, range, and multiple flags
dewy server --registry ghr://myorg/app \
  --port 80,443 \
  --port 8080-8085 \
  -p 9090 \
  -- /opt/app/current/app
```

This example specifies web server ports (80, 443), application instance ports (8080-8085), and metrics port (9090) all at once.

## Practical Configuration Examples

Here are specific configuration examples based on actual operational environments.

### Development Environment

Development environments require detailed information in an easy-to-understand format for rapid problem identification and resolution.

```bash
# Recommended configuration for development environment
dewy server \
  --log-level debug \
  --log-format text \
  --registry ghr://myorg/myapp \
  --port 8080,8081 \
  -- /opt/myapp/current/myapp
```

### Staging Environment

Staging environments test under conditions close to production while collecting information necessary for problem investigation.

```bash
# Recommended configuration for staging environment
dewy server \
  --log-level info \
  --log-format json \
  --registry ghr://myorg/myapp \
  --port 8080,8090 \
  -- /opt/myapp/current/myapp
```

### Production Environment

Production environments prioritize performance and record only essential information.

```bash
# Recommended configuration for production environment
dewy server \
  --log-level error \
  --log-format json \
  --registry ghr://myorg/myapp \
  --port 8080,8443,9090 \
  -- /opt/myapp/current/myapp
```

## Application Implementation Examples

Here are implementation patterns for applications that utilize dewy's multi-port functionality.

### Multi-Port Application Design Patterns

Multi-port applications typically provide different functions or configurations for each port.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "sync"
    "syscall"
    "time"
)

type MultiPortServer struct {
    servers []*http.Server
    wg      sync.WaitGroup
}

func (m *MultiPortServer) Start(ports []string) error {
    for _, port := range ports {
        server := &http.Server{
            Addr:    ":" + port,
            Handler: m.createHandler(port),
        }
        m.servers = append(m.servers, server)

        m.wg.Add(1)
        go func(s *http.Server, p string) {
            defer m.wg.Done()
            log.Printf("Starting server on port %s", p)
            if err := s.ListenAndServe(); err != http.ErrServerClosed {
                log.Printf("Server error on port %s: %v", p, err)
            }
        }(server, port)
    }
    return nil
}

func (m *MultiPortServer) createHandler(port string) http.Handler {
    mux := http.NewServeMux()

    switch port {
    case "8080":
        // Main API
        mux.HandleFunc("/api/", m.apiHandler)
        mux.HandleFunc("/health", m.healthHandler)
    case "8090":
        // Admin interface
        mux.HandleFunc("/admin/", m.adminHandler)
        mux.HandleFunc("/admin/health", m.adminHealthHandler)
    case "9090":
        // Metrics
        mux.HandleFunc("/metrics", m.metricsHandler)
    }

    return mux
}

func (m *MultiPortServer) Shutdown(ctx context.Context) error {
    for _, server := range m.servers {
        if err := server.Shutdown(ctx); err != nil {
            return err
        }
    }
    m.wg.Wait()
    return nil
}
```

This design provides different functionality on each port while implementing unified shutdown processing.

### Retrieving Port Information from Environment Variables

You can dynamically retrieve port information using environment variables provided by server-starter.

```go
package main

import (
    "os"
    "strconv"
    "strings"
)

func getServerStarterPorts() ([]string, error) {
    // Retrieve port information from SERVER_STARTER_PORT environment variable
    portEnv := os.Getenv("SERVER_STARTER_PORT")
    if portEnv == "" {
        // Fallback: application-specific environment variables
        return getApplicationPorts()
    }

    var ports []string
    for _, portSpec := range strings.Split(portEnv, ";") {
        if portSpec == "" {
            continue
        }

        // Parse "name=port" format
        parts := strings.Split(portSpec, "=")
        if len(parts) == 2 {
            ports = append(ports, parts[1])
        }
    }

    return ports, nil
}

func getApplicationPorts() ([]string, error) {
    // Retrieve from application-specific environment variables
    portsStr := os.Getenv("APP_PORTS")
    if portsStr == "" {
        return []string{"8080"}, nil // Default
    }

    return strings.Split(portsStr, ","), nil
}

func main() {
    ports, err := getServerStarterPorts()
    if err != nil {
        log.Fatal(err)
    }

    server := &MultiPortServer{}
    if err := server.Start(ports); err != nil {
        log.Fatal(err)
    }

    // Signal waiting and shutdown processing
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    <-sigCh

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

This implementation allows applications to automatically adapt to dewy configuration changes.

### Port-Specific Routing Configuration

Large-scale applications require detailed routing configuration for each port.

```go
package main

import (
    "net/http"
    "github.com/gorilla/mux"
)

type PortConfig struct {
    Port        string
    Middlewares []func(http.Handler) http.Handler
    Routes      map[string]http.HandlerFunc
}

func createPortConfigs() map[string]PortConfig {
    return map[string]PortConfig{
        "8080": {
            Port: "8080",
            Middlewares: []func(http.Handler) http.Handler{
                loggingMiddleware,
                corsMiddleware,
                rateLimitMiddleware,
            },
            Routes: map[string]http.HandlerFunc{
                "GET /api/users":     getUsersHandler,
                "POST /api/users":    createUserHandler,
                "GET /api/health":    healthHandler,
            },
        },
        "8090": {
            Port: "8090",
            Middlewares: []func(http.Handler) http.Handler{
                authMiddleware,
                adminLoggingMiddleware,
            },
            Routes: map[string]http.HandlerFunc{
                "GET /admin/dashboard": adminDashboardHandler,
                "GET /admin/users":     adminUsersHandler,
                "POST /admin/config":   adminConfigHandler,
            },
        },
        "9090": {
            Port: "9090",
            Middlewares: []func(http.Handler) http.Handler{
                metricsMiddleware,
            },
            Routes: map[string]http.HandlerFunc{
                "GET /metrics":     metricsHandler,
                "GET /debug/pprof": pprofHandler,
            },
        },
    }
}

func createServer(config PortConfig) *http.Server {
    router := mux.NewRouter()

    // Configure routes
    for pattern, handler := range config.Routes {
        parts := strings.SplitN(pattern, " ", 2)
        if len(parts) == 2 {
            router.HandleFunc(parts[1], handler).Methods(parts[0])
        }
    }

    // Apply middleware
    var handler http.Handler = router
    for i := len(config.Middlewares) - 1; i >= 0; i-- {
        handler = config.Middlewares[i](handler)
    }

    return &http.Server{
        Addr:         ":" + config.Port,
        Handler:      handler,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
}
```

This design allows you to apply different security requirements and performance settings for each port.

### Graceful Shutdown for All Ports

Multi-port applications need to execute graceful shutdown simultaneously on all ports.

```go
package main

import (
    "context"
    "log"
    "sync"
    "time"
)

type GracefulServer struct {
    servers []*http.Server
    mu      sync.RWMutex
}

func (g *GracefulServer) AddServer(server *http.Server) {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.servers = append(g.servers, server)
}

func (g *GracefulServer) StartAll() error {
    g.mu.RLock()
    defer g.mu.RUnlock()

    for _, server := range g.servers {
        go func(s *http.Server) {
            log.Printf("Starting server on %s", s.Addr)
            if err := s.ListenAndServe(); err != http.ErrServerClosed {
                log.Printf("Server error on %s: %v", s.Addr, err)
            }
        }(server)
    }

    return nil
}

func (g *GracefulServer) ShutdownAll(timeout time.Duration) error {
    g.mu.RLock()
    defer g.mu.RUnlock()

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    var wg sync.WaitGroup
    errors := make(chan error, len(g.servers))

    for _, server := range g.servers {
        wg.Add(1)
        go func(s *http.Server) {
            defer wg.Done()
            log.Printf("Shutting down server on %s", s.Addr)
            if err := s.Shutdown(ctx); err != nil {
                errors <- err
            }
        }(server)
    }

    // Wait for all servers to complete shutdown
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Println("All servers shut down successfully")
        return nil
    case err := <-errors:
        log.Printf("Server shutdown error: %v", err)
        return err
    case <-ctx.Done():
        log.Println("Shutdown timeout reached")
        return ctx.Err()
    }
}
```

This implementation enables safe service termination on all ports during deployments or maintenance.

## Operational Considerations

When operating multi-port configurations in production environments, there are additional elements that don't need consideration in typical single-port configurations. Considering these elements in advance enables stable operations.

### Port Number Management and Documentation

When using multiple ports, it's important to clearly manage port number assignments and their purposes. Establish a numbering system that avoids port number conflicts organization-wide and considers future expansion.

```bash
# Port number management example
# 8000-8099: Web applications
# 8100-8199: API services
# 8200-8299: Microservices
# 9000-9099: Metrics and monitoring
# 9100-9199: Administration and debugging
```

In documentation, clearly specify the purpose, protocol, and access control requirements for each port, and share this information across teams. Always update documentation when making configuration changes to prevent operational errors.

### Firewall Configuration Integration

Multi-port configurations require port-specific access control according to security policies. Configure appropriate firewall rules based on purpose, such as internal communication, external access, or administrative use.

```bash
# Firewall configuration example (ufw)
sudo ufw allow 80/tcp    # HTTP (public)
sudo ufw allow 443/tcp   # HTTPS (public)
sudo ufw allow from 10.0.0.0/8 to any port 8080 # API (internal network only)
sudo ufw allow from 192.168.1.0/24 to any port 9090 # Metrics (management network only)
```

Combined with network segmentation, you can apply appropriate security levels to each port.

### Monitoring and Health Checks

When providing services on multiple ports, you need to monitor the health of each port individually. Design appropriate health check endpoints so that load balancers and monitoring systems can accurately understand the status of each port.

```bash
# Prometheus configuration example
- job_name: 'multi-port-app'
  static_configs:
    - targets: ['app.example.com:8080']  # Main API
      labels:
        service: 'api'
    - targets: ['app.example.com:8090']  # Admin interface
      labels:
        service: 'admin'
    - targets: ['app.example.com:9090']  # Metrics
      labels:
        service: 'metrics'
```

By collecting different metrics for each port and creating service-specific dashboards, you can enable early problem detection and rapid response.

### Security and Access Control

Multi-port environments commonly apply different security requirements for each port. Configure appropriate security settings for each port's purpose, such as authentication methods, encryption levels, and access log detail levels.

Apply strong authentication to administrative ports and rate limiting to API ports. Also, prepare mechanisms to quickly disable only affected ports during security incidents.

## Troubleshooting

This section covers problems specific to multi-port configurations and their solutions.

### Resolving Port Binding Errors

The most common issue is when specified ports are already in use. This occurs when multiple applications are running on the same server or when previous processes haven't terminated properly.

```bash
# Check ports in use
sudo netstat -tlnp | grep :8080
sudo lsof -i :8080

# Check and terminate processes
ps aux | grep dewy
sudo kill -TERM <process_id>

# Wait for port to be released
while sudo lsof -i :8080 > /dev/null; do sleep 1; done
```

When specifying multiple ports, if binding fails on some ports, the entire application may fail to start. Check error logs to identify problematic port numbers and address them.

### Addressing Permission Issues

Binding to privileged ports (below 1024) requires root privileges. From a security perspective, proper permission configuration is important in production environments.

```bash
# Check permissions
id

# Run with sudo if necessary
sudo dewy server --registry ghr://myorg/app --port 80,443 -- /opt/app/current/app

# Or grant CAP_NET_BIND_SERVICE
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/dewy
```

When using systemd, you can specify User=root in the service file or use AmbientCapabilities to grant only necessary privileges.

### Log Verification and Debugging Procedures

Debugging multi-port configurations requires detailed log output for each port. Enable dewy's debug mode to check the status of each port in detail.

```bash
# Run with debug level
dewy server --log-level debug --registry ghr://myorg/app --port 8080,8081,9090 -- /opt/app/current/app

# Filter logs for specific ports
journalctl -u dewy | grep "port.*8080"

# Real-time log monitoring
tail -f /var/log/dewy.log | grep -E "(port|bind|listen)"
```

You can also verify that port information is correctly transmitted by checking environment variables from server-starter.

### Common Configuration Mistakes and Solutions

Here are common mistakes in configuration files or command-line arguments and their solutions.

**Invalid port range specification**: Correct invalid range specifications like `--port 8080-8081-8082` to `--port 8080-8082` or `--port 8080,8081,8082`.

**Duplicate port specification**: Duplicates like `--port 8080,8080,8081` are automatically removed, but check for unintended duplicates.

**Out-of-range port numbers**: Port numbers outside the range 1-65535 are invalid. Check your configuration and use numbers within the valid range.

**Application configuration mismatch**: If ports specified by dewy don't match ports the application actually listens on, connections will fail. Verify that correct port information is passed to the application through environment variables or command-line arguments.