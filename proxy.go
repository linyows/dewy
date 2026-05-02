package dewy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linyows/dewy/logging"
	"github.com/linyows/dewy/telemetry"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// tcpProxy manages a TCP proxy for a single port.
type tcpProxy struct {
	proxyPort    int
	listener     net.Listener
	backends     []tcpBackend
	backendIndex uint64 // Atomic counter for round-robin
	mu           sync.RWMutex
	done         chan struct{}
	logger       *logging.Logger
	idleTimeout  time.Duration // 0 means no timeout
	metrics      *telemetry.Metrics
}

// tcpBackend represents a backend server.
type tcpBackend struct {
	host string
	port int
}

// startProxy starts TCP proxies for all configured port mappings.
func (d *Dewy) startProxy(ctx context.Context) error {
	if len(d.config.Container.PortMappings) == 0 {
		return fmt.Errorf("no port mappings configured for proxy")
	}

	d.proxyMutex.Lock()
	d.tcpProxies = make(map[int]*tcpProxy)
	d.proxyMutex.Unlock()

	// Start a TCP proxy for each port mapping
	var metrics *telemetry.Metrics
	if d.telemetry != nil && d.telemetry.Enabled() {
		metrics = d.telemetry.Metrics()
	}
	for _, mapping := range d.config.Container.PortMappings {
		proxy, err := newTCPProxy(mapping.ProxyPort, d.logger, d.config.Container.ProxyIdleTimeout, metrics)
		if err != nil {
			// Clean up already started proxies
			if stopErr := d.stopProxy(ctx); stopErr != nil {
				d.logger.Error("Failed to stop proxies during cleanup", slog.String("error", stopErr.Error()))
			}
			return fmt.Errorf("failed to start proxy on port %d: %w", mapping.ProxyPort, err)
		}

		d.proxyMutex.Lock()
		d.tcpProxies[mapping.ProxyPort] = proxy
		d.proxyMutex.Unlock()

		d.logger.Info("TCP proxy started",
			slog.Int("proxy_port", mapping.ProxyPort))
	}

	d.logger.Info("All TCP proxies started successfully",
		slog.Int("count", len(d.config.Container.PortMappings)))

	return nil
}

// newTCPProxy creates and starts a new TCP proxy on the specified port.
func newTCPProxy(port int, logger *logging.Logger, idleTimeout time.Duration, metrics *telemetry.Metrics) (*tcpProxy, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	proxy := &tcpProxy{
		proxyPort:   port,
		listener:    listener,
		backends:    make([]tcpBackend, 0),
		done:        make(chan struct{}),
		logger:      logger,
		idleTimeout: idleTimeout,
		metrics:     metrics,
	}

	go proxy.acceptLoop()

	return proxy, nil
}

// acceptLoop accepts incoming connections and proxies them to backends.
func (p *tcpProxy) acceptLoop() {
	for {
		select {
		case <-p.done:
			return
		default:
		}

		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return
			default:
				p.logger.Debug("Accept error",
					slog.Int("proxy_port", p.proxyPort),
					slog.String("error", err.Error()))
				continue
			}
		}

		go p.handleConnection(conn)
	}
}

// handleConnection proxies a single connection to a backend.
func (p *tcpProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	ctx := context.Background()
	portAttr := otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort))

	// Record connection accepted metrics immediately
	if p.metrics != nil {
		p.metrics.ProxyConnectionsTotal.Add(ctx, 1, portAttr)
		p.metrics.ProxyActiveConnections.Add(ctx, 1, portAttr)
	}
	connStart := time.Now()
	defer func() {
		if p.metrics != nil {
			p.metrics.ProxyActiveConnections.Add(ctx, -1, portAttr)
			p.metrics.ProxyConnectionDuration.Record(ctx, time.Since(connStart).Seconds(), portAttr)
		}
	}()

	// Get backend using round-robin
	backend, ok := p.getNextBackend()
	if !ok {
		p.logger.Debug("No backend available",
			slog.Int("proxy_port", p.proxyPort))
		if p.metrics != nil {
			p.metrics.ProxyErrorsTotal.Add(ctx, 1, portAttr)
		}
		return
	}

	// Connect to backend with latency measurement
	backendAddr := net.JoinHostPort(backend.host, strconv.Itoa(backend.port))
	dialStart := time.Now()
	backendConn, err := net.DialTimeout("tcp", backendAddr, 5*time.Second)
	if p.metrics != nil {
		p.metrics.ProxyConnectLatency.Record(ctx, time.Since(dialStart).Seconds(), portAttr)
	}
	if err != nil {
		p.logger.Error("Failed to connect to backend",
			slog.Int("proxy_port", p.proxyPort),
			slog.String("backend", backendAddr),
			slog.String("error", err.Error()))
		if p.metrics != nil {
			p.metrics.ProxyErrorsTotal.Add(ctx, 1, portAttr)
		}
		return
	}
	defer backendConn.Close()

	p.logger.Debug("Proxying connection",
		slog.Int("proxy_port", p.proxyPort),
		slog.String("backend", backendAddr),
		slog.String("client", clientConn.RemoteAddr().String()))

	// Wrap connections with idle timeout (skip if timeout is 0)
	var src io.Reader = clientConn
	var dst io.Writer = backendConn
	var srcBack io.Reader = backendConn
	var dstBack io.Writer = clientConn
	if p.idleTimeout > 0 {
		tcClient := &timeoutConn{Conn: clientConn, idleTimeout: p.idleTimeout}
		tcBackend := &timeoutConn{Conn: backendConn, idleTimeout: p.idleTimeout}
		src = tcClient
		dst = tcBackend
		srcBack = tcBackend
		dstBack = tcClient
	}

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(dst, src)
		if p.metrics != nil && n > 0 {
			p.metrics.ProxyBytesTransferred.Add(ctx, n, portAttr)
		}
		done <- struct{}{}
	}()

	go func() {
		n, _ := io.Copy(dstBack, srcBack)
		if p.metrics != nil && n > 0 {
			p.metrics.ProxyBytesTransferred.Add(ctx, n, portAttr)
		}
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done
}

// getNextBackend returns the next backend using round-robin.
func (p *tcpProxy) getNextBackend() (tcpBackend, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.backends) == 0 {
		return tcpBackend{}, false
	}

	index := atomic.AddUint64(&p.backendIndex, 1) - 1
	return p.backends[index%uint64(len(p.backends))], true
}

// addBackend adds a backend to this proxy.
func (p *tcpProxy) addBackend(host string, port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.backends = append(p.backends, tcpBackend{host: host, port: port})
	p.logger.Info("Backend added to TCP proxy",
		slog.Int("proxy_port", p.proxyPort),
		slog.String("backend_host", host),
		slog.Int("backend_port", port),
		slog.Int("total_backends", len(p.backends)))

	if p.metrics != nil {
		p.metrics.ProxyBackendCount.Add(context.Background(), 1,
			otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort)))
	}
}

// removeBackend removes a backend from this proxy.
func (p *tcpProxy) removeBackend(host string, port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, b := range p.backends {
		if b.host == host && b.port == port {
			p.backends = append(p.backends[:i], p.backends[i+1:]...)
			p.logger.Info("Backend removed from TCP proxy",
				slog.Int("proxy_port", p.proxyPort),
				slog.String("backend_host", host),
				slog.Int("backend_port", port),
				slog.Int("remaining_backends", len(p.backends)))

			if p.metrics != nil {
				p.metrics.ProxyBackendCount.Add(context.Background(), -1,
					otelmetric.WithAttributes(attribute.Int("proxy_port", p.proxyPort)))
			}
			return true
		}
	}
	return false
}

// stop stops the TCP proxy.
func (p *tcpProxy) stop() error {
	close(p.done)
	return p.listener.Close()
}

// backendCount returns the number of backends.
func (p *tcpProxy) backendCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}

// stopProxy gracefully shuts down all TCP proxies.
func (d *Dewy) stopProxy(ctx context.Context) error {
	d.proxyMutex.Lock()
	defer d.proxyMutex.Unlock()

	if d.tcpProxies == nil {
		return nil
	}

	d.logger.Info("Stopping TCP proxies", slog.Int("count", len(d.tcpProxies)))

	var errs []error
	for port, proxy := range d.tcpProxies {
		if err := proxy.stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop proxy on port %d: %w", port, err))
		}
	}

	d.tcpProxies = nil

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	d.logger.Info("All TCP proxies stopped")
	return nil
}

// totalProxyBackends returns the total number of backends across all TCP proxies.
func (d *Dewy) totalProxyBackends() int {
	d.proxyMutex.RLock()
	defer d.proxyMutex.RUnlock()

	total := 0
	for _, proxy := range d.tcpProxies {
		total += proxy.backendCount()
	}
	return total
}
