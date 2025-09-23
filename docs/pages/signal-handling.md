---
title: Signal Handling
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy uses Unix signals to control processes, enabling proper application restarts and clean shutdown procedures. Signal handling is a critical feature for zero-downtime deployments and safe system operations in production environments.

## Supported Signals

Dewy supports the following Unix signals, each triggering different behaviors:

### SIGHUP

SIGHUP signals are received but ignored by dewy itself. This design assumes that SIGHUP will be sent to child processes (managed applications). Applications are expected to handle SIGHUP signals to perform graceful restarts or configuration reloads.

### SIGUSR1

When dewy receives a SIGUSR1 signal, it gracefully restarts the currently running server application. This signal can be used to manually restart the server when a new version of the application has been deployed.

### SIGINT

SIGINT signals (typically sent by Ctrl+C) cause dewy to terminate gracefully. This includes stopping the scheduler and sending termination messages through the notification system as part of the cleanup process.

### SIGTERM

SIGTERM signals are treated as termination requests from process management systems (such as systemd). Like SIGINT, they trigger proper cleanup procedures before process termination.

### SIGQUIT

SIGQUIT signals are also treated as termination signals, executing the same shutdown procedures as SIGINT and SIGTERM.

## Signal Handling Implementation

Dewy's signal handling is implemented in the `waitSigs()` function (dewy.go:125-153). This function runs in a goroutine that continuously monitors specified signals.

```go
func (d *Dewy) waitSigs(ctx context.Context) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

    for sig := range sigCh {
        d.logger.Debug("PID received signal", slog.Int("pid", os.Getpid()), slog.String("signal", sig.String()))
        switch sig {
        case syscall.SIGHUP:
            continue
        case syscall.SIGUSR1:
            // Server restart processing
        case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
            // Termination processing
        }
    }
}
```

When signals are received, the process ID and signal name are logged, and appropriate actions are executed based on the signal type.

## Server Restart Mechanism

Server restarts triggered by SIGUSR1 signals are handled by the `restartServer()` function. This feature enables switching to new application versions without downtime.

The restart process sends a SIGHUP signal to the current process, which triggers a graceful restart through the server-starter library integration.

```bash
# Manual server restart example
kill -USR1 <dewy_process_id>
```

Upon successful restart, completion notifications are sent through the notification system.

## Shutdown Process

When termination signals (SIGINT, SIGTERM, SIGQUIT) are received, dewy performs cleanup in the following sequence:

First, the periodic job scheduler is stopped to prevent new deployment processes from starting. Next, termination messages are sent through the notification system to alert administrators that dewy is shutting down. Finally, the main processing loop is terminated and the process is completely stopped.

## Application Implementation Examples

Applications managed by dewy can implement proper signal handling to enable safer and more reliable deployments.

### HTTP Server Applications

HTTP server applications should implement graceful shutdown to properly terminate connections during request processing.

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: http.DefaultServeMux,
    }

    // Create signal channel
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)

        // Graceful shutdown
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            log.Printf("Server shutdown error: %v", err)
        }
    }()

    log.Println("Starting server on :8080")
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        log.Printf("Server error: %v", err)
    }
}
```

This example implements graceful shutdown when receiving SIGHUP signals, allowing existing requests to complete before stopping the server.

### Applications with Database Connections

Applications using database connection pools need to properly close connections during termination.

```go
package main

import (
    "database/sql"
    "log"
    "os"
    "os/signal"
    "syscall"
    _ "github.com/lib/pq"
)

type App struct {
    db *sql.DB
}

func (a *App) shutdown() {
    log.Println("Closing database connections...")
    if err := a.db.Close(); err != nil {
        log.Printf("Database close error: %v", err)
    }
    log.Println("Application shutdown complete")
}

func main() {
    db, err := sql.Open("postgres", "postgresql://user:pass@localhost/db")
    if err != nil {
        log.Fatal(err)
    }

    app := &App{db: db}

    // Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        app.shutdown()
        os.Exit(0)
    }()

    // Application main processing
    log.Println("Application started")
    select {} // Block indefinitely
}
```

Proper database connection pool closure prevents connection leaks and reduces load on the database server.

### Background Workers

Worker applications that perform long-running processes need to handle interruption and resumption properly.

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
)

type Worker struct {
    ctx    context.Context
    cancel context.CancelFunc
}

func (w *Worker) start() {
    w.ctx, w.cancel = context.WithCancel(context.Background())

    for {
        select {
        case <-w.ctx.Done():
            log.Println("Worker stopped")
            return
        default:
            // Simulate long-running processing
            log.Println("Processing...")
            time.Sleep(5 * time.Second)
        }
    }
}

func (w *Worker) stop() {
    log.Println("Stopping worker...")
    w.cancel()
}

func main() {
    worker := &Worker{}

    // Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        worker.stop()
    }()

    log.Println("Worker started")
    worker.start()
}
```

This pattern uses context.Context to control process interruption, enabling safe process termination upon signal reception.

### WebSocket Servers

Applications managing WebSocket connections need proper connection termination procedures.

```go
package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/gorilla/websocket"
)

type WebSocketServer struct {
    clients map[*websocket.Conn]bool
    mu      sync.RWMutex
}

func (ws *WebSocketServer) addClient(conn *websocket.Conn) {
    ws.mu.Lock()
    defer ws.mu.Unlock()
    ws.clients[conn] = true
}

func (ws *WebSocketServer) removeClient(conn *websocket.Conn) {
    ws.mu.Lock()
    defer ws.mu.Unlock()
    delete(ws.clients, conn)
    conn.Close()
}

func (ws *WebSocketServer) closeAllConnections() {
    ws.mu.Lock()
    defer ws.mu.Unlock()

    log.Printf("Closing %d WebSocket connections", len(ws.clients))
    for conn := range ws.clients {
        conn.WriteMessage(websocket.CloseMessage, []byte("Server shutting down"))
        conn.Close()
    }
    ws.clients = make(map[*websocket.Conn]bool)
}

func main() {
    server := &WebSocketServer{
        clients: make(map[*websocket.Conn]bool),
    }

    upgrader := websocket.Upgrader{}

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            return
        }
        server.addClient(conn)
        defer server.removeClient(conn)

        // WebSocket processing
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                break
            }
        }
    })

    // Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        server.closeAllConnections()
        os.Exit(0)
    }()

    log.Println("WebSocket server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Proper WebSocket connection termination prevents timeout errors on the client side and improves user experience.

## Practical Usage Examples

In production environments, signal transmission is typically performed through process management systems or scripts.

### systemd Integration

When managing dewy with systemd, proper service file configuration enables system-level signal transmission.

```systemd
[Unit]
Description=Dewy Deployment Service
After=network.target

[Service]
Type=simple
User=dewy
WorkingDirectory=/opt/dewy
ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp --port 8080 -- /opt/myapp/current/myapp
ExecReload=/bin/kill -USR1 $MAINPID
KillSignal=SIGTERM
TimeoutStopSec=30
Restart=always

[Install]
WantedBy=multi-user.target
```

This configuration enables application restart via the `systemctl reload dewy` command.

### Monitoring and Log Output

Log output during signal reception provides important information for system operation monitoring.

```bash
# Monitor dewy logs
journalctl -u dewy -f

# Display only signal reception logs
journalctl -u dewy | grep "received signal"
```

Logs record received signal types, process IDs, and execution results, which can be utilized for troubleshooting and operational monitoring.

## Troubleshooting

This section covers common signal handling issues and their solutions.

### When Signals Are Not Processed Correctly

If signals are not being handled as expected, first check that the process is running normally.

```bash
# Check dewy process status
ps aux | grep dewy

# Send signal to process
kill -USR1 <process_id>

# Check signal reception in logs
tail -f /var/log/dewy.log
```

Issues may arise if the process is in a zombie state or if there are insufficient permissions to send signals.

### When Restart Fails

If restart via SIGUSR1 fails, there may be issues with the application's signal handling implementation. Check application logs to verify that SIGHUP signals are being processed correctly.

Also verify that server-starter library configuration is correct. Issues may stem from port binding problems or incorrect process startup path configuration.

### Log Verification Methods

For detailed problem identification, debug-level output is effective.

```bash
# Start dewy with debug level
dewy server --log-level debug --registry ghr://myorg/myapp --port 8080 -- /opt/myapp/current/myapp
```

Debug-level logs record detailed signal reception information and inter-process communication status, making it easier to identify root causes of problems.