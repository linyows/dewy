---
title: Application Integration for the Server Command
---

# {% $markdoc.frontmatter.title %} {% #overview %}

This page describes what an application needs to do to run under the `server` command. Dewy opens the ports given by `--port` before it starts the application, keeps them open while it replaces one process with the next, and stops the previous process with SIGTERM. The application therefore serves on the sockets that dewy passes to it and exits when it receives SIGTERM.

The `assets` command does not start a process, and the `container` command runs the application in a container that listens on its own port, so neither needs the changes on this page.

## Serving on the Sockets Passed by dewy {% #inherited-sockets %}

An application started by the `server` command does not bind its ports. Dewy has already opened every port and passes the sockets to the application as inherited file descriptors, starting from 3. Binding a port that dewy holds fails with "address already in use" on Linux. Serving on the inherited sockets is also what keeps the ports open across a restart: the new process accepts connections on the same socket that the previous process used, so no connection is refused while the processes are replaced.

### Using the sock Module in Go {% #sock %}

A Go application gets the sockets with the `github.com/linyows/dewy/sock` module. The module depends only on the standard library.

```bash
go get github.com/linyows/dewy/sock
```

Replace `net.Listen` with `sock.Listen`:

```go
package main

import (
    "log"
    "net/http"

    "github.com/linyows/dewy/sock"
)

func main() {
    ln, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(http.Serve(ln, http.DefaultServeMux))
}
```

`sock.Listen` and `sock.ListenPacket` take the same arguments as `net.Listen` and `net.ListenPacket`. Under dewy, they return the inherited socket whose port matches the address. The host part of the address is ignored, because `--host` decides the interface. Without dewy, or for a port that dewy did not open, they bind the address themselves, so the same code runs on a development machine.

The module also provides the following functions.

- **`sock.Listeners`** and **`sock.PacketConns`**: return every inherited TCP or UDP socket, in the order of `DEWY_PORT`. They suit an application that serves whatever `--port` lists, as in the [Multi-Port](/multi-port#inherited-sockets) sample.
- **`sock.HealthListener`**: returns the health check socket described in [Health Checks](#health-checks), or nil when there is none.
- **`sock.Inherited`** and **`sock.Entries`**: report whether the process was started with sockets from dewy, and return the parsed entries.

Each inherited socket can be returned once; a second request for it returns `sock.ErrTaken`. When `DEWY_PORT` is not set, the module reads `SERVER_STARTER_PORT`.

### The DEWY_PORT Environment Variable {% #dewy-port %}

Dewy lists the sockets in the `DEWY_PORT` environment variable. It also sets the same value in `SERVER_STARTER_PORT`, whose name and format are compatible with server-starter, so applications written for server-starter work without changes.

Entries are joined by `;`, and each entry is `<address>=<fd>`. `<address>` is the port as configured: `port`, or `host:port` when `--host` is set to an address other than `0.0.0.0` or `::`, with an IPv6 address in brackets. A UDP socket carries a `/udp` suffix on the address. The entries are sorted by port number, with the TCP port before the UDP port of the same number.

| `--port` and `--host` | `DEWY_PORT` |
|---|---|
| `--port 8080` | `8080=3` |
| `--port 8080,8090` | `8080=3;8090=4` |
| `--port 8080 --host 127.0.0.1` | `127.0.0.1:8080=3` |
| `--port 8080 --host ::1` | `[::1]:8080=3` |
| `--port 53,53/udp` | `53=3;53/udp=4` |
| `--port 53/udp --host ::1` | `[::1]:53/udp=3` |

The number after `=` is a file descriptor, not a port.

### Other Languages {% #other-languages %}

An application in another language reads `DEWY_PORT` and opens each descriptor as a socket. A library written for server-starter, such as Perl's Server::Starter, reads `SERVER_STARTER_PORT` and works as it does under server-starter.

In Python, `socket.socket(fileno=fd)` opens the descriptor and detects its family and type:

```python
import os
import socket

entries = dict(e.split("=") for e in os.environ["DEWY_PORT"].split(";"))
sock = socket.socket(fileno=int(entries["8080"]))
```

In Node.js, `server.listen` accepts the descriptor:

```javascript
const http = require("http");

const entries = Object.fromEntries(
  process.env.DEWY_PORT.split(";").map((e) => e.split("=")),
);
http.createServer(handler).listen({ fd: Number(entries["8080"]) });
```

These samples look up the entry by its address, so with `--host` the key is `host:port` instead of the port alone.

## Starting and Stopping {% #lifecycle %}

A new process replaces the previous one after it has stayed up for `--startup-wait` (1 second by default). A process that exits sooner fails the restart, and the previous process keeps serving. An application that can fail a few seconds after starting, for example while it loads its configuration or connects to a database, should set `--startup-wait` longer than that.

After the replacement, dewy sends SIGTERM to the previous process. The application should stop accepting new connections, finish the requests in progress, and exit. A process that has not exited `--drain-time` (30 seconds by default) after SIGTERM is killed with SIGKILL. Dewy sends SIGTERM to the running process in the same way when dewy itself stops on SIGINT, SIGTERM or SIGQUIT.

A process that exits without receiving SIGTERM is counted as a crash in the `dewy.server.crashes.total` metric and started again after a backoff that doubles from 1 second up to 30 seconds.

See [Signal Handling](/signal-handling) for graceful shutdown samples.

## Health Checks {% #health-checks %}

With `--health-path`, dewy sends an HTTP GET request to that path after each deployment and treats a 2xx or 3xx response as healthy. The request goes to the lowest TCP `--port`, on the `--host` address, or on `127.0.0.1` when `--host` is omitted or `0.0.0.0` and on `::1` when it is `::`. The application serves the path on its regular port, and needs nothing else.

With [`--health-before-switch`](/reference#--health-before-switch), dewy checks each new process before it replaces the previous one. Dewy passes each new process one more TCP socket, on the loopback interface, and announces it in the `DEWY_HEALTH_PORT` environment variable as a single `<address>=<fd>` entry, for example `127.0.0.1:41234=5`. The descriptor follows the `DEWY_PORT` sockets, or is 3 when there is no `--port`. Dewy sends `--health-path` requests to this socket after the startup wait, so the application must serve its health endpoint there. Serve the same handler as on the regular port, so that the check reflects the state of the process that will receive traffic.

```go
package main

import (
    "log"
    "net/http"

    "github.com/linyows/dewy/sock"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    // Report the state the application depends on, such as its database
    // connection, so that a process that cannot serve fails the check.
    w.WriteHeader(http.StatusOK)
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)

    // The health socket is set only under --health-before-switch.
    health, err := sock.HealthListener()
    if err != nil {
        log.Fatal(err)
    }
    if health != nil {
        go http.Serve(health, mux)
    }

    ln, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(http.Serve(ln, mux))
}
```

The health socket belongs to one process only, so the answer always comes from the new process, not from the previous one. An application that does not serve this socket fails every health check under `--health-before-switch`.
