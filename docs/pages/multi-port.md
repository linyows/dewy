---
title: Multi-Port Support
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy provides functionality to simultaneously manage multiple TCP and UDP ports for a single application. This feature enables unified deployment of services with different purposes, such as HTTP and HTTPS, API and WebUI, or production and debug endpoints. Dewy opens every port once when the server starts and passes all of them to the application, so a restart replaces the application process without closing any port.

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

### UDP Ports

A port without a suffix is TCP. Append `/udp` to a port or a range to open a UDP socket instead, as in Docker's port syntax. A `/tcp` suffix is accepted and means the same as no suffix. The same port number can be given for both protocols.

```bash
# DNS on TCP and UDP port 53, plus HTTP on 8080
dewy server --registry ghr://myorg/dns \
  --port 53,53/udp,8080 \
  -- /opt/dns/current/dns

# A UDP port range
dewy server --registry ghr://myorg/app --port 6000-6002/udp -- /opt/app/current/app
```

The `/udp` suffix is available for the `server` command only. `--health-path` probes the lowest TCP port, or the first `--socket` without one, and requires at least one of them; UDP ports are not probed.

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

When dewy starts the application, dewy has already opened every configured port. The application does not bind the ports itself: it serves on the sockets that dewy passes to it, as shown in [Using the Sockets Passed by dewy](#inherited-sockets). Binding a port that dewy holds fails with "address already in use" on Linux, and the inherited sockets are what keep the ports open across a restart.

```go
package main

import (
    "context"
    "log"
    "net"
    "net/http"
    "sync"
)

type MultiPortServer struct {
    servers []*http.Server
    wg      sync.WaitGroup
}

func (m *MultiPortServer) Start(listeners []net.Listener) {
    for _, l := range listeners {
        server := &http.Server{
            Handler: m.createHandler(listenerPort(l)),
        }
        m.servers = append(m.servers, server)

        m.wg.Add(1)
        go func(s *http.Server, l net.Listener) {
            defer m.wg.Done()
            log.Printf("Starting server on %s", l.Addr())
            if err := s.Serve(l); err != http.ErrServerClosed {
                log.Printf("Server error on %s: %v", l.Addr(), err)
            }
        }(server, l)
    }
}

func (m *MultiPortServer) createHandler(port int) http.Handler {
    mux := http.NewServeMux()

    switch port {
    case 8080:
        // Main API
        mux.HandleFunc("/api/", m.apiHandler)
        mux.HandleFunc("/health", m.healthHandler)
    case 8090:
        // Admin interface
        mux.HandleFunc("/admin/", m.adminHandler)
        mux.HandleFunc("/admin/health", m.adminHealthHandler)
    case 9090:
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

// listenerPort returns the TCP port of l, or 0 for a Unix socket.
func listenerPort(l net.Listener) int {
    if addr, ok := l.Addr().(*net.TCPAddr); ok {
        return addr.Port
    }
    return 0
}
```

This design provides different functionality on each port while implementing unified shutdown processing.

### Using the Sockets Passed by dewy {% #inherited-sockets %}

Dewy lists the sockets it opened in the `DEWY_PORT` environment variable, and a Go application gets them with the `github.com/linyows/dewy/sock` module; see [Application Integration for the Server Command](/application) for the variable, the module, and other languages. A multi-port application can take every inherited socket with `sock.Listeners` and `sock.PacketConns` instead of listing its ports:

```go
package main

import (
    "context"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/linyows/dewy/sock"
)

// serveUDP answers each datagram on pc with the same payload.
func serveUDP(pc net.PacketConn) {
    buf := make([]byte, 65535)
    for {
        n, from, err := pc.ReadFrom(buf)
        if err != nil {
            return
        }
        pc.WriteTo(buf[:n], from)
    }
}

func main() {
    listeners, err := sock.Listeners()
    if err != nil {
        log.Fatal(err)
    }
    packetConns, err := sock.PacketConns()
    if err != nil {
        log.Fatal(err)
    }
    if !sock.Inherited() {
        // Started directly, for example during development.
        l, err := net.Listen("tcp", ":8080")
        if err != nil {
            log.Fatal(err)
        }
        listeners = []net.Listener{l}
    }

    server := &MultiPortServer{}
    server.Start(listeners)
    for _, pc := range packetConns {
        go serveUDP(pc)
    }

    // dewy sends SIGTERM to the previous worker after a restart.
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    <-sigCh

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

The sockets follow the configuration, so the application does not need its own list of ports: changing `--port` changes the sockets it serves on.

Dewy keeps these sockets open while it replaces the application process. A UDP datagram that arrives during the replacement is queued on the socket and read by whichever worker reads next. During the replacement, the previous and the new worker read from the same UDP socket, so each datagram is received by only one of them.

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
        Handler:      handler,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
}
```

Serve each server on the inherited listener for its port, selected with `listenerPort` from the example above. The server has no `Addr`, because it does not open the port itself.

This design allows you to apply different security requirements and performance settings for each port.

### Graceful Shutdown for All Ports

Multi-port applications need to execute graceful shutdown simultaneously on all ports.

```go
package main

import (
    "context"
    "log"
    "net"
    "net/http"
    "sync"
    "time"
)

type GracefulServer struct {
    servers   []*http.Server
    listeners []net.Listener
    mu        sync.RWMutex
}

// AddServer registers server to be served on l, an inherited listener.
func (g *GracefulServer) AddServer(server *http.Server, l net.Listener) {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.servers = append(g.servers, server)
    g.listeners = append(g.listeners, l)
}

func (g *GracefulServer) StartAll() {
    g.mu.RLock()
    defer g.mu.RUnlock()

    for i, server := range g.servers {
        go func(s *http.Server, l net.Listener) {
            log.Printf("Starting server on %s", l.Addr())
            if err := s.Serve(l); err != http.ErrServerClosed {
                log.Printf("Server error on %s: %v", l.Addr(), err)
            }
        }(server, g.listeners[i])
    }
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
            log.Println("Shutting down server")
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

Dewy binds every configured port when the server starts. If any port cannot be bound, dewy closes the ports it has already opened and does not start the application. The error log names the address that failed.

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

You can also verify that port information is correctly transmitted by checking the `DEWY_PORT` environment variable that dewy passes to the application.

### Common Configuration Mistakes and Solutions

Here are common mistakes in configuration files or command-line arguments and their solutions.

**Invalid port range specification**: Correct invalid range specifications like `--port 8080-8081-8082` to `--port 8080-8082` or `--port 8080,8081,8082`.

**Duplicate port specification**: Duplicates like `--port 8080,8080,8081` are automatically removed, but check for unintended duplicates.

**Out-of-range port numbers**: Port numbers outside the range 1-65535 are invalid. Check your configuration and use numbers within the valid range.

**Application configuration mismatch**: If ports specified by dewy don't match ports the application actually listens on, connections will fail. Verify that correct port information is passed to the application through environment variables or command-line arguments.
